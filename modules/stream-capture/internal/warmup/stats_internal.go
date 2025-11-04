package warmup

import (
	"time"

	streamcapture "github.com/e7canasta/orion-care-sensor/modules/stream-capture"
)

// calculateFPSStatsInternal wraps the public CalculateFPSStats function
// and converts between internal Frame timestamps and public WarmupStats
//
// This avoids code duplication and maintains a single source of truth
// for FPS calculation logic in the public API.
func calculateFPSStatsInternal(frameTimes []time.Time, totalDuration time.Duration) *WarmupStats {
	// Delegate to public function
	publicStats := streamcapture.CalculateFPSStats(frameTimes, totalDuration)

	// Convert public type to internal type (they have identical fields)
	return &WarmupStats{
		FramesReceived: publicStats.FramesReceived,
		Duration:       publicStats.Duration,
		FPSMean:        publicStats.FPSMean,
		FPSStdDev:      publicStats.FPSStdDev,
		FPSMin:         publicStats.FPSMin,
		FPSMax:         publicStats.FPSMax,
		IsStable:       publicStats.IsStable,
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
