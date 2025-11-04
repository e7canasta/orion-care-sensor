package warmup

import (
	"math"
	"time"
)

const (
	// fpsStabilityThreshold is the maximum allowed FPS standard deviation as a fraction of mean FPS.
	// A stream is considered stable if stddev < 15% of mean FPS.
	// Example: 30 FPS mean → stable if stddev < 4.5 FPS
	fpsStabilityThreshold = 0.15

	// jitterStabilityThreshold is the maximum allowed mean jitter as a fraction of expected interval.
	// A stream is considered stable if mean jitter < 20% of expected inter-frame interval.
	// Example: 30 FPS (33ms interval) → stable if jitter < 6.6ms
	jitterStabilityThreshold = 0.20
)

// CalculateFPSStats calculates FPS statistics from frame timestamps
//
// This function:
//  1. Calculates mean FPS (overall)
//  2. Calculates instantaneous FPS for each frame interval
//  3. Finds min/max instantaneous FPS
//  4. Calculates standard deviation of instantaneous FPS
//  5. Calculates jitter statistics (inter-frame interval variance)
//  6. Determines stability (stddev < 15% of mean AND jitter < 20%)
//
// Stability threshold:
//  - FPS: stddev < 15% of mean FPS
//  - Jitter: mean jitter < 20% of expected interval
//
// Example: 30 FPS mean → stable if stddev < 4.5 AND jitter < 0.007s
//
// NOTE: This is the canonical implementation. Public API (warmup_stats.go)
// wraps this function to maintain backward compatibility.
func CalculateFPSStats(frameTimes []time.Time, totalDuration time.Duration) *WarmupStats {
	n := len(frameTimes)

	// Handle edge case: no frames
	if n == 0 {
		return &WarmupStats{
			FramesReceived: 0,
			Duration:       totalDuration,
			FPSMean:        0,
			FPSStdDev:      0,
			FPSMin:         0,
			FPSMax:         0,
			IsStable:       false,
			JitterMean:     0,
			JitterStdDev:   0,
			JitterMax:      0,
		}
	}

	// Calculate mean FPS (overall rate)
	fpsMean := float64(n) / totalDuration.Seconds()

	// Calculate instantaneous FPS for each interval
	instantaneousFPS := make([]float64, 0, n-1)
	for i := 1; i < n; i++ {
		interval := frameTimes[i].Sub(frameTimes[i-1]).Seconds()
		if interval > 0 {
			fps := 1.0 / interval
			instantaneousFPS = append(instantaneousFPS, fps)
		}
	}

	// Handle edge case: no valid intervals
	if len(instantaneousFPS) == 0 {
		return &WarmupStats{
			FramesReceived: n,
			Duration:       totalDuration,
			FPSMean:        fpsMean,
			FPSStdDev:      0,
			FPSMin:         0,
			FPSMax:         0,
			IsStable:       false,
			JitterMean:     0,
			JitterStdDev:   0,
			JitterMax:      0,
		}
	}

	// Find min/max instantaneous FPS
	fpsMin := instantaneousFPS[0]
	fpsMax := instantaneousFPS[0]
	for _, fps := range instantaneousFPS {
		if fps < fpsMin {
			fpsMin = fps
		}
		if fps > fpsMax {
			fpsMax = fps
		}
	}

	// Calculate standard deviation of instantaneous FPS
	var sumSquares float64
	for _, fps := range instantaneousFPS {
		diff := fps - fpsMean
		sumSquares += diff * diff
	}
	fpsStdDev := math.Sqrt(sumSquares / float64(len(instantaneousFPS)))

	// Calculate jitter statistics
	// Jitter = variance from expected inter-frame interval
	expectedInterval := 1.0 / fpsMean // Expected time between frames (seconds)

	jitters := make([]float64, 0, n-1)
	for i := 1; i < n; i++ {
		actualInterval := frameTimes[i].Sub(frameTimes[i-1]).Seconds()
		jitter := math.Abs(actualInterval - expectedInterval)
		jitters = append(jitters, jitter)
	}

	// Calculate jitter mean
	var jitterSum float64
	jitterMax := 0.0
	for _, j := range jitters {
		jitterSum += j
		if j > jitterMax {
			jitterMax = j
		}
	}
	jitterMean := jitterSum / float64(len(jitters))

	// Calculate jitter standard deviation
	var jitterSumSquares float64
	for _, j := range jitters {
		diff := j - jitterMean
		jitterSumSquares += diff * diff
	}
	jitterStdDev := math.Sqrt(jitterSumSquares / float64(len(jitters)))

	// Determine stability: stddev < 15% of mean AND jitter < 20% of expected interval
	fpsStable := fpsStdDev < (fpsMean * fpsStabilityThreshold)
	jitterStable := jitterMean < (expectedInterval * jitterStabilityThreshold)
	isStable := fpsStable && jitterStable

	return &WarmupStats{
		FramesReceived: n,
		Duration:       totalDuration,
		FPSMean:        fpsMean,
		FPSStdDev:      fpsStdDev,
		FPSMin:         fpsMin,
		FPSMax:         fpsMax,
		IsStable:       isStable,
		JitterMean:     jitterMean,
		JitterStdDev:   jitterStdDev,
		JitterMax:      jitterMax,
	}
}

// CalculateOptimalInferenceRate calculates optimal inference rate based on warm-up stats
//
// This function ensures the inference rate never exceeds maxRate, and reduces
// it if the stream FPS is lower than expected.
//
// Logic:
//   - If stream FPS >= maxRate: return maxRate
//   - If stream FPS < maxRate: return 90% of stream FPS (safety margin)
//
// Example:
//   - maxRate=1.0, stream FPS=30 → return 1.0 (use max)
//   - maxRate=1.0, stream FPS=0.8 → return 0.72 (90% of 0.8)
func CalculateOptimalInferenceRate(warmupStats *WarmupStats, maxRate float64) float64 {
	if warmupStats == nil {
		return maxRate
	}

	// Always respect max rate (configured inference rate)
	optimalRate := maxRate

	// If stream FPS is very low, reduce inference rate accordingly
	if warmupStats.FPSMean < maxRate {
		optimalRate = warmupStats.FPSMean * 0.9 // 90% of stream FPS (safety margin)
	}

	return optimalRate
}
