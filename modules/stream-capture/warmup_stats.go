package streamcapture

import (
	"time"

	"github.com/e7canasta/orion-care-sensor/modules/stream-capture/internal/warmup"
)

// CalculateFPSStats calculates FPS statistics from frame timestamps
//
// This is a public wrapper around internal/warmup.CalculateFPSStats to maintain
// backward compatibility. The canonical implementation lives in internal/warmup/stats.go.
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
func CalculateFPSStats(frameTimes []time.Time, totalDuration time.Duration) *WarmupStats {
	// Delegate to internal implementation
	internalStats := warmup.CalculateFPSStats(frameTimes, totalDuration)

	// Convert internal WarmupStats to public WarmupStats (identical structure)
	return &WarmupStats{
		FramesReceived: internalStats.FramesReceived,
		Duration:       internalStats.Duration,
		FPSMean:        internalStats.FPSMean,
		FPSStdDev:      internalStats.FPSStdDev,
		FPSMin:         internalStats.FPSMin,
		FPSMax:         internalStats.FPSMax,
		IsStable:       internalStats.IsStable,
		JitterMean:     internalStats.JitterMean,
		JitterStdDev:   internalStats.JitterStdDev,
		JitterMax:      internalStats.JitterMax,
	}
}
