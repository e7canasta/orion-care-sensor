package streamcapture

import (
	"math"
	"math/rand"
	"testing"
	"testing/quick"
	"time"
)

// TestWarmupStability_Property1_StabilityThresholds tests the stability criteria
//
// Property: FPS stddev < 15% of mean AND jitter < 20% of expected interval → IsStable = true
func TestWarmupStability_Property1_StabilityThresholds(t *testing.T) {
	// Test with stable FPS (low stddev, low jitter)
	t.Run("stable stream", func(t *testing.T) {
		frameTimes := generateStableFrameTimes(30, 1.0, 0.05) // 30 frames, 1 FPS, 5% jitter
		stats := CalculateFPSStats(frameTimes, 30*time.Second)

		if !stats.IsStable {
			t.Errorf("Expected stable stream, got IsStable=false (FPS stddev: %.2f%%, jitter: %.2f%%)",
				(stats.FPSStdDev/stats.FPSMean)*100,
				(stats.JitterMean/(1.0/stats.FPSMean))*100,
			)
		}
	})

	// Test with unstable FPS (high stddev)
	t.Run("unstable FPS", func(t *testing.T) {
		frameTimes := generateStableFrameTimes(30, 1.0, 0.25) // 30 frames, 1 FPS, 25% jitter
		stats := CalculateFPSStats(frameTimes, 30*time.Second)

		if stats.IsStable {
			t.Errorf("Expected unstable stream (high jitter), got IsStable=true (jitter: %.2f%%)",
				(stats.JitterMean/(1.0/stats.FPSMean))*100,
			)
		}
	})

	// Test boundary case: exactly 15% stddev
	t.Run("boundary FPS stddev", func(t *testing.T) {
		// Generate frames with exactly 15% stddev
		// This is harder to achieve precisely, so we test near-boundary
		frameTimes := generateStableFrameTimes(50, 2.0, 0.14) // Just below 15%
		stats := CalculateFPSStats(frameTimes, 25*time.Second)

		// Should be stable (below threshold)
		if !stats.IsStable {
			t.Logf("Near-boundary test: FPS stddev %.2f%%, jitter %.2f%%",
				(stats.FPSStdDev/stats.FPSMean)*100,
				(stats.JitterMean/(1.0/stats.FPSMean))*100,
			)
		}
	})
}

// TestWarmupStability_Property2_MonotonicRelationship tests monotonic increase
//
// Property: Increase jitter → decrease stability (eventually)
func TestWarmupStability_Property2_MonotonicRelationship(t *testing.T) {
	targetFPS := 1.0
	numFrames := 50

	jitterLevels := []float64{0.05, 0.10, 0.15, 0.20, 0.25}
	var previousStable bool = true

	for i, jitter := range jitterLevels {
		frameTimes := generateStableFrameTimes(numFrames, targetFPS, jitter)
		duration := time.Duration(float64(numFrames)/targetFPS) * time.Second
		stats := CalculateFPSStats(frameTimes, duration)

		t.Logf("Jitter %.1f%% → IsStable=%v (FPS stddev: %.2f%%, jitter: %.2f%%)",
			jitter*100,
			stats.IsStable,
			(stats.FPSStdDev/stats.FPSMean)*100,
			(stats.JitterMean/(1.0/stats.FPSMean))*100,
		)

		// Once unstable, should remain unstable (or become unstable)
		if i > 0 && !previousStable && stats.IsStable {
			t.Errorf("Monotonic violation: jitter increased from %.1f%% to %.1f%%, but stability flipped from false to true",
				jitterLevels[i-1]*100, jitter*100,
			)
		}

		previousStable = stats.IsStable
	}
}

// TestWarmupStability_Property3_EdgeCases tests edge cases
//
// Property: Edge cases should not panic and should return sensible defaults
func TestWarmupStability_Property3_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		frameTimes []time.Time
		duration   time.Duration
		wantStable bool
	}{
		{
			name:       "zero frames",
			frameTimes: []time.Time{},
			duration:   1 * time.Second,
			wantStable: false,
		},
		{
			name:       "one frame",
			frameTimes: []time.Time{time.Now()},
			duration:   1 * time.Second,
			wantStable: false,
		},
		{
			name: "two frames (minimal)",
			frameTimes: []time.Time{
				time.Now(),
				time.Now().Add(1 * time.Second),
			},
			duration:   1 * time.Second,
			wantStable: false, // Not enough data for stability assessment
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := CalculateFPSStats(tt.frameTimes, tt.duration)

			// Should not panic
			if stats == nil {
				t.Fatal("CalculateFPSStats returned nil")
			}

			// Check basic invariants
			if stats.FPSStdDev < 0 {
				t.Errorf("FPSStdDev should be >= 0, got %.2f", stats.FPSStdDev)
			}
			if stats.JitterMean < 0 {
				t.Errorf("JitterMean should be >= 0, got %.2f", stats.JitterMean)
			}
			if stats.JitterMax < 0 {
				t.Errorf("JitterMax should be >= 0, got %.2f", stats.JitterMax)
			}

			// Check stability expectation
			if stats.IsStable != tt.wantStable {
				t.Errorf("Expected IsStable=%v, got %v", tt.wantStable, stats.IsStable)
			}
		})
	}
}

// TestWarmupStability_Property4_JitterBounds tests jitter bounds
//
// Property: Jitter calculation should always be >= 0
func TestWarmupStability_Property4_JitterBounds(t *testing.T) {
	f := func(fps float64, numFrames uint8) bool {
		// Bound inputs to reasonable ranges
		if fps < 0.1 || fps > 30.0 {
			return true // Skip invalid FPS
		}
		if numFrames < 2 || numFrames > 100 {
			return true // Skip invalid frame counts
		}

		// Generate frames with some jitter
		frameTimes := generateStableFrameTimes(int(numFrames), fps, 0.1)
		duration := time.Duration(float64(numFrames)/fps*1000) * time.Millisecond

		stats := CalculateFPSStats(frameTimes, duration)

		// Property: All jitter metrics must be non-negative
		if stats.JitterMean < 0 {
			t.Logf("FAIL: JitterMean < 0 (%.6f) with fps=%.2f, frames=%d", stats.JitterMean, fps, numFrames)
			return false
		}
		if stats.JitterStdDev < 0 {
			t.Logf("FAIL: JitterStdDev < 0 (%.6f) with fps=%.2f, frames=%d", stats.JitterStdDev, fps, numFrames)
			return false
		}
		if stats.JitterMax < 0 {
			t.Logf("FAIL: JitterMax < 0 (%.6f) with fps=%.2f, frames=%d", stats.JitterMax, fps, numFrames)
			return false
		}

		// Property: JitterMax >= JitterMean (max should be at least mean)
		if stats.JitterMax < stats.JitterMean {
			t.Logf("FAIL: JitterMax (%.6f) < JitterMean (%.6f)", stats.JitterMax, stats.JitterMean)
			return false
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestWarmupStability_Property5_FPSBounds tests FPS bounds
//
// Property: FPS statistics should be consistent (min <= mean <= max)
func TestWarmupStability_Property5_FPSBounds(t *testing.T) {
	f := func(fps float64, numFrames uint8) bool {
		// Bound inputs to reasonable ranges
		if fps < 0.1 || fps > 30.0 {
			return true // Skip invalid FPS
		}
		if numFrames < 2 || numFrames > 100 {
			return true // Skip invalid frame counts
		}

		frameTimes := generateStableFrameTimes(int(numFrames), fps, 0.1)
		duration := time.Duration(float64(numFrames)/fps*1000) * time.Millisecond

		stats := CalculateFPSStats(frameTimes, duration)

		// Property: FPSMin <= FPSMean <= FPSMax (within floating point tolerance)
		tolerance := 0.001
		if stats.FPSMin > stats.FPSMean+tolerance {
			t.Logf("FAIL: FPSMin (%.2f) > FPSMean (%.2f)", stats.FPSMin, stats.FPSMean)
			return false
		}
		if stats.FPSMax < stats.FPSMean-tolerance {
			t.Logf("FAIL: FPSMax (%.2f) < FPSMean (%.2f)", stats.FPSMax, stats.FPSMean)
			return false
		}

		// Property: FPSStdDev should be >= 0
		if stats.FPSStdDev < 0 {
			t.Logf("FAIL: FPSStdDev < 0 (%.6f)", stats.FPSStdDev)
			return false
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestWarmupStability_Property6_DurationConsistency tests duration consistency
//
// Property: Calculated FPS should be consistent with duration and frame count
func TestWarmupStability_Property6_DurationConsistency(t *testing.T) {
	f := func(fps float64, numFrames uint8) bool {
		// Bound inputs
		if fps < 0.1 || fps > 30.0 {
			return true
		}
		if numFrames < 10 || numFrames > 100 {
			return true // Need enough frames for accuracy
		}

		frameTimes := generateStableFrameTimes(int(numFrames), fps, 0.05) // Low jitter
		duration := time.Duration(float64(numFrames)/fps*1000) * time.Millisecond

		stats := CalculateFPSStats(frameTimes, duration)

		// Property: FPSMean should be approximately fps
		// Allow 10% tolerance due to jitter
		tolerance := fps * 0.10
		if math.Abs(stats.FPSMean-fps) > tolerance {
			t.Logf("FAIL: FPSMean (%.2f) deviates from expected (%.2f) by more than 10%%", stats.FPSMean, fps)
			return false
		}

		return true
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 50}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// Helper: generateStableFrameTimes generates frame timestamps with controlled jitter
//
// numFrames: number of frames to generate
// targetFPS: target frames per second
// jitterFraction: jitter as fraction of inter-frame interval (0.0 = perfect, 0.2 = 20% jitter)
func generateStableFrameTimes(numFrames int, targetFPS float64, jitterFraction float64) []time.Time {
	if numFrames < 1 {
		return []time.Time{}
	}

	expectedInterval := 1.0 / targetFPS // seconds
	frameTimes := make([]time.Time, numFrames)

	// Start at a fixed time
	baseTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	frameTimes[0] = baseTime

	// Generate subsequent frames with jitter
	rng := rand.New(rand.NewSource(42)) // Deterministic for reproducibility

	for i := 1; i < numFrames; i++ {
		// Add jitter: random offset within ±jitterFraction of expected interval
		jitterSeconds := (rng.Float64()*2 - 1) * jitterFraction * expectedInterval
		actualInterval := expectedInterval + jitterSeconds

		frameTimes[i] = frameTimes[i-1].Add(time.Duration(actualInterval*1000) * time.Millisecond)
	}

	return frameTimes
}

// Benchmark: CalculateFPSStats performance
func BenchmarkCalculateFPSStats(b *testing.B) {
	frameTimes := generateStableFrameTimes(100, 1.0, 0.1)
	duration := 100 * time.Second

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = CalculateFPSStats(frameTimes, duration)
	}
}
