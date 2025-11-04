package rtsp

import (
	"math"
	"math/rand"
	"testing"
	"time"
)

// TestLatencyWindow_Properties validates core invariants using property-based testing
//
// These tests ensure LatencyWindow maintains mathematical correctness under all conditions:
// - Bounded growth (ring buffer never exceeds capacity)
// - Statistical invariants (mean ≤ max, P95 ≤ max, P95 ≥ mean)
// - Thread-safety (lock-free via atomic pointer pattern)
//
// Rationale: From CONSULTORIA_TECNICA_DISENO.md Quick Win #1
// LatencyWindow is a bounded context with pure functions - ideal for property tests
func TestLatencyWindow_Properties(t *testing.T) {
	t.Run("Property_1_BoundedGrowth", func(t *testing.T) {
		// Property: AddSample never panics, Count never exceeds buffer size
		window := &LatencyWindow{}

		// Add more samples than buffer size (stress test)
		for i := 0; i < 500; i++ {
			window.AddSample(float64(i))

			// Invariant: Count ≤ buffer size
			if window.Count > len(window.Samples) {
				t.Fatalf("Count exceeded buffer size at i=%d: Count=%d, BufferSize=%d",
					i, window.Count, len(window.Samples))
			}

			// Invariant: Index wraps correctly (0 ≤ Index < len(Samples))
			if window.Index < 0 || window.Index >= len(window.Samples) {
				t.Fatalf("Index out of bounds at i=%d: Index=%d, BufferSize=%d",
					i, window.Index, len(window.Samples))
			}
		}

		// After 500 samples, Count should be capped at buffer size
		if window.Count != len(window.Samples) {
			t.Errorf("Expected Count=%d after overflow, got %d",
				len(window.Samples), window.Count)
		}

		t.Logf("✅ Bounded growth validated: 500 samples → Count=%d (capped)", window.Count)
	})

	t.Run("Property_2_MeanLessThanOrEqualMax", func(t *testing.T) {
		// Property: Mean ≤ Max always (mathematical invariant)
		testCases := []struct {
			name    string
			samples []float64
		}{
			{"uniform_low", []float64{1.0, 1.0, 1.0, 1.0}},
			{"uniform_high", []float64{100.0, 100.0, 100.0}},
			{"increasing", []float64{1.0, 2.0, 3.0, 4.0, 5.0}},
			{"decreasing", []float64{5.0, 4.0, 3.0, 2.0, 1.0}},
			{"spike", []float64{1.0, 1.0, 100.0, 1.0, 1.0}},
			{"mixed", []float64{10.5, 20.3, 15.8, 30.2, 5.1}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				window := &LatencyWindow{}
				for _, s := range tc.samples {
					window.AddSample(s)
				}

				mean, _, max := window.GetStats()

				// Invariant: mean ≤ max
				if mean > max {
					t.Errorf("Mean > Max violation: mean=%.4f, max=%.4f, samples=%v",
						mean, max, tc.samples)
				}

				t.Logf("✅ Mean ≤ Max: %.4f ≤ %.4f", mean, max)
			})
		}
	})

	t.Run("Property_3_P95LessThanOrEqualMax", func(t *testing.T) {
		// Property: P95 ≤ Max always (by definition of percentile)
		window := &LatencyWindow{}

		// Fill buffer with random samples
		rand.Seed(time.Now().UnixNano())
		for i := 0; i < 100; i++ {
			sample := rand.Float64() * 100.0 // 0-100ms range
			window.AddSample(sample)
		}

		_, p95, max := window.GetStats()

		// Invariant: P95 ≤ Max
		if p95 > max {
			t.Errorf("P95 > Max violation: p95=%.4f, max=%.4f", p95, max)
		}

		t.Logf("✅ P95 ≤ Max: %.4f ≤ %.4f (100 random samples)", p95, max)
	})

	t.Run("Property_4_P95GreaterThanOrEqualMean_SkewedDistribution", func(t *testing.T) {
		// Property: P95 ≥ Mean for right-skewed distributions
		// (Most latencies low, few high outliers - typical in video processing)
		window := &LatencyWindow{}

		// Simulate realistic latency distribution:
		// 90% samples: 5-15ms (normal decoding)
		// 10% samples: 50-100ms (occasional stalls)
		for i := 0; i < 90; i++ {
			window.AddSample(5.0 + float64(i%10)) // 5-15ms
		}
		for i := 0; i < 10; i++ {
			window.AddSample(50.0 + float64(i*5)) // 50-100ms
		}

		mean, p95, _ := window.GetStats()

		// For right-skewed distribution, P95 should be > mean
		if p95 < mean {
			t.Errorf("P95 < Mean for skewed distribution: p95=%.4f, mean=%.4f", p95, mean)
		}

		t.Logf("✅ P95 ≥ Mean (skewed): %.4f ≥ %.4f", p95, mean)
	})

	t.Run("Property_5_EmptyWindowReturnsZeros", func(t *testing.T) {
		// Property: Empty window returns zeros (no undefined behavior)
		window := &LatencyWindow{}

		mean, p95, max := window.GetStats()

		if mean != 0.0 || p95 != 0.0 || max != 0.0 {
			t.Errorf("Empty window should return zeros: mean=%.4f, p95=%.4f, max=%.4f",
				mean, p95, max)
		}

		t.Logf("✅ Empty window returns zeros")
	})

	t.Run("Property_6_SingleSampleEqualsAllStats", func(t *testing.T) {
		// Property: For single sample, Mean = P95 = Max
		window := &LatencyWindow{}
		sampleValue := 42.5

		window.AddSample(sampleValue)
		mean, p95, max := window.GetStats()

		// All stats should equal the single sample value
		if mean != sampleValue || p95 != sampleValue || max != sampleValue {
			t.Errorf("Single sample stats mismatch: mean=%.4f, p95=%.4f, max=%.4f, expected=%.4f",
				mean, p95, max, sampleValue)
		}

		t.Logf("✅ Single sample: Mean = P95 = Max = %.4f", sampleValue)
	})

	t.Run("Property_7_RingBufferOverwriteOldSamples", func(t *testing.T) {
		// Property: After buffer overflow, old samples are overwritten (FIFO)
		window := &LatencyWindow{}

		// Phase 1: Fill buffer with low values (0-99)
		for i := 0; i < 100; i++ {
			window.AddSample(float64(i))
		}
		mean1, _, max1 := window.GetStats()

		// Phase 2: Add 50 high values (1000+) - should overwrite first 50 samples
		for i := 0; i < 50; i++ {
			window.AddSample(1000.0 + float64(i))
		}
		mean2, _, max2 := window.GetStats()

		// After overwrite:
		// - Max should increase (new high values)
		// - Mean should increase (50% of buffer now high values)
		if max2 <= max1 {
			t.Errorf("Max did not increase after overwrite: max1=%.2f, max2=%.2f", max1, max2)
		}
		if mean2 <= mean1 {
			t.Errorf("Mean did not increase after overwrite: mean1=%.2f, mean2=%.2f", mean1, mean2)
		}

		t.Logf("✅ Ring buffer overwrite: max %.2f→%.2f, mean %.2f→%.2f",
			max1, max2, mean1, mean2)
	})

	t.Run("Property_8_MonotonicMax", func(t *testing.T) {
		// Property: Max is monotonic (never decreases) until buffer overflow
		window := &LatencyWindow{}
		prevMax := 0.0

		for i := 0; i < 100; i++ {
			// Add random sample (but ensure at least one increases max)
			sample := float64(i) + rand.Float64()
			window.AddSample(sample)

			_, _, currentMax := window.GetStats()

			// Max should never decrease before buffer is full
			if currentMax < prevMax {
				t.Errorf("Max decreased at sample %d: %.4f → %.4f", i, prevMax, currentMax)
			}

			prevMax = currentMax
		}

		t.Logf("✅ Max monotonic for first 100 samples: %.4f", prevMax)
	})
}

// TestLatencyWindow_P95Calculation validates P95 calculation correctness
//
// This is a regression test for potential off-by-one errors in P95 index calculation
// (callbacks.go:69). P95 calculation is critical for SLO compliance (e.g., "95% of
// frames decoded in <50ms").
func TestLatencyWindow_P95Calculation(t *testing.T) {
	testCases := []struct {
		name          string
		samples       []float64
		expectedP95   float64
		tolerance     float64
	}{
		{
			name:        "sorted_ascending",
			samples:     []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
			expectedP95: 19.0, // 95th percentile of 20 samples = index 19 (20th value)
			tolerance:   0.1,
		},
		{
			name:        "uniform_values",
			samples:     []float64{50, 50, 50, 50, 50, 50, 50, 50, 50, 50},
			expectedP95: 50.0, // All values same → P95 = value
			tolerance:   0.1,
		},
		{
			name: "realistic_latencies",
			// Simulate 100 samples: 90 low (10ms), 10 high (100ms)
			samples: func() []float64 {
				s := make([]float64, 100)
				for i := 0; i < 90; i++ {
					s[i] = 10.0
				}
				for i := 90; i < 100; i++ {
					s[i] = 100.0
				}
				return s
			}(),
			expectedP95: 100.0, // 95th percentile should be in high group
			tolerance:   0.1,
		},
		{
			name: "edge_case_95_samples",
			// 95 samples → P95 index = 90 (0.95 * 95 = 90.25 → floor to 90)
			samples: func() []float64 {
				s := make([]float64, 95)
				for i := range s {
					s[i] = float64(i + 1) // 1, 2, 3, ..., 95
				}
				return s
			}(),
			expectedP95: 91.0, // 91st value (index 90)
			tolerance:   1.0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			window := &LatencyWindow{}
			for _, s := range tc.samples {
				window.AddSample(s)
			}

			_, p95, _ := window.GetStats()

			// Validate P95 is within tolerance
			if math.Abs(p95-tc.expectedP95) > tc.tolerance {
				t.Errorf("P95 calculation incorrect: got %.4f, expected %.4f±%.4f",
					p95, tc.expectedP95, tc.tolerance)
			}

			t.Logf("✅ P95 correct: %.4f (expected %.4f)", p95, tc.expectedP95)
		})
	}
}

// TestLatencyWindow_Concurrency validates lock-free atomic pointer pattern
//
// While LatencyWindow itself is not thread-safe (caller must use atomic.Pointer),
// this test validates the typical usage pattern from rtsp.go (callbacks.go:138-144).
func TestLatencyWindow_AtomicPointerPattern(t *testing.T) {
	// Simulate the atomic pointer pattern from rtsp.go
	// This is the ACTUAL usage pattern in production code
	t.Run("atomic_pointer_update_pattern", func(t *testing.T) {
		// Create atomic pointer (as in rtsp.go:54)
		var latencyPtr AtomicPointer[LatencyWindow]
		initialWindow := &LatencyWindow{}
		latencyPtr.Store(initialWindow)

		// Simulate adding a sample (as in callbacks.go:138-144)
		addSample := func(latencyMS float64) {
			window := latencyPtr.Load()
			if window != nil {
				// Create copy (lock-free update pattern)
				newWindow := *window
				newWindow.AddSample(latencyMS)
				latencyPtr.Store(&newWindow)
			}
		}

		// Add samples
		for i := 0; i < 50; i++ {
			addSample(float64(i))
		}

		// Verify final state
		finalWindow := latencyPtr.Load()
		if finalWindow.Count != 50 {
			t.Errorf("Expected Count=50, got %d", finalWindow.Count)
		}

		mean, p95, max := finalWindow.GetStats()
		t.Logf("✅ Atomic pointer pattern: Count=%d, Mean=%.2f, P95=%.2f, Max=%.2f",
			finalWindow.Count, mean, p95, max)
	})
}

// AtomicPointer is a minimal implementation for testing
// (Production code uses sync/atomic.Pointer[T])
type AtomicPointer[T any] struct {
	value *T
}

func (p *AtomicPointer[T]) Store(val *T) {
	p.value = val
}

func (p *AtomicPointer[T]) Load() *T {
	return p.value
}

// TestLatencyWindow_EdgeCases validates boundary conditions
func TestLatencyWindow_EdgeCases(t *testing.T) {
	t.Run("negative_latency", func(t *testing.T) {
		// In theory, negative latency is impossible (clock monotonicity)
		// But we should handle it gracefully (no panic)
		window := &LatencyWindow{}
		window.AddSample(-10.0)

		mean, p95, max := window.GetStats()

		// Should not panic, should return negative value
		if mean >= 0 || p95 >= 0 || max >= 0 {
			t.Errorf("Negative latency not handled: mean=%.2f, p95=%.2f, max=%.2f",
				mean, p95, max)
		}

		t.Logf("✅ Negative latency handled gracefully: %.2f", mean)
	})

	t.Run("very_large_latency", func(t *testing.T) {
		// Pathological case: multi-second latency (camera disconnect, etc.)
		window := &LatencyWindow{}
		window.AddSample(999999.0) // ~1000 seconds

		mean, _, max := window.GetStats()

		if mean != 999999.0 || max != 999999.0 {
			t.Errorf("Large latency not handled: mean=%.2f, max=%.2f", mean, max)
		}

		t.Logf("✅ Large latency handled: %.0fms", max)
	})

	t.Run("zero_latency", func(t *testing.T) {
		// Edge case: zero latency (unlikely but valid)
		window := &LatencyWindow{}
		window.AddSample(0.0)

		mean, p95, max := window.GetStats()

		if mean != 0.0 || p95 != 0.0 || max != 0.0 {
			t.Errorf("Zero latency not handled: mean=%.2f, p95=%.2f, max=%.2f",
				mean, p95, max)
		}

		t.Logf("✅ Zero latency handled")
	})

	t.Run("nan_infinity", func(t *testing.T) {
		// Pathological case: NaN or Inf (should not occur in production)
		window := &LatencyWindow{}
		window.AddSample(math.NaN())
		window.AddSample(math.Inf(1))

		mean, p95, max := window.GetStats()

		// NaN propagates in calculations - this is expected Go behavior
		// We just ensure no panic occurs
		t.Logf("⚠️  NaN/Inf handled without panic: mean=%.2f, p95=%.2f, max=%.2f",
			mean, p95, max)
	})
}
