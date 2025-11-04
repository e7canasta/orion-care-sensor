package streamcapture

import (
	"math"
	"time"
)

// CalculateFPSStats calculates FPS statistics from frame timestamps
//
// This function:
//  1. Calculates mean FPS (overall)
//  2. Calculates instantaneous FPS for each frame interval
//  3. Finds min/max instantaneous FPS
//  4. Calculates standard deviation of instantaneous FPS
//  5. Determines stability (stddev < 15% of mean)
//
// Stability threshold: stddev < 15% of mean FPS
// Example: 30 FPS mean → stable if stddev < 4.5
//
// Extracted to avoid duplication between Warmup() and internal/warmup package.
func CalculateFPSStats(frameTimes []time.Time, totalDuration time.Duration) *WarmupStats {
	n := len(frameTimes)

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

	// Determine stability: stddev < 15% of mean
	isStable := fpsStdDev < (fpsMean * 0.15)

	return &WarmupStats{
		FramesReceived: n,
		Duration:       totalDuration,
		FPSMean:        fpsMean,
		FPSStdDev:      fpsStdDev,
		FPSMin:         fpsMin,
		FPSMax:         fpsMax,
		IsStable:       isStable,
	}
}
