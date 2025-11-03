package stream

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"time"
)

// WarmupStats contains statistics from stream warm-up phase
type WarmupStats struct {
	FramesReceived int
	Duration       time.Duration
	FPSMean        float64
	FPSStdDev      float64
	FPSMin         float64
	FPSMax         float64
	IsStable       bool
}

// FrameGetter interface for warm-up (minimal interface to avoid circular import)
type FrameGetter interface {
	GetTimestamp() time.Time
	GetSeq() uint64
}

// WarmupStream warms up the stream and measures real FPS stability
// Does NOT start workers - just consumes frames to stabilize stream
// Takes a channel of frames directly to avoid interface mismatch
func WarmupStream(ctx context.Context, frames <-chan interface{}, duration time.Duration) (*WarmupStats, error) {
	slog.Info("warming up stream",
		"duration", duration,
		"reason", "measure real FPS and stabilize pipeline",
	)

	startTime := time.Now()

	// Track frame arrival times
	frameTimes := make([]time.Time, 0, 100)
	var lastFrameTime time.Time

	warmupCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Consume frames without processing
	for {
		select {
		case <-warmupCtx.Done():
			// Warm-up complete
			goto analyze

		case frameData, ok := <-frames:
			if !ok {
				return nil, fmt.Errorf("stream closed during warm-up")
			}

			// Try to get timestamp from frame
			var frameTime time.Time
			var frameSeq uint64

			if frame, ok := frameData.(FrameGetter); ok {
				frameTime = frame.GetTimestamp()
				frameSeq = frame.GetSeq()
			} else {
				// Fallback: use current time
				frameTime = time.Now()
			}

			frameTimes = append(frameTimes, frameTime)

			if !lastFrameTime.IsZero() {
				interval := frameTime.Sub(lastFrameTime)
				slog.Debug("warm-up frame received",
					"seq", frameSeq,
					"interval_ms", interval.Milliseconds(),
				)
			}

			lastFrameTime = frameTime
		}
	}

analyze:
	elapsed := time.Since(startTime)

	if len(frameTimes) < 2 {
		return nil, fmt.Errorf("not enough frames during warm-up (got %d)", len(frameTimes))
	}

	// Calculate FPS statistics
	stats := calculateFPSStats(frameTimes, elapsed)

	slog.Info("stream warm-up complete",
		"frames", stats.FramesReceived,
		"duration", stats.Duration,
		"fps_mean", fmt.Sprintf("%.2f", stats.FPSMean),
		"fps_stddev", fmt.Sprintf("%.2f", stats.FPSStdDev),
		"fps_range", fmt.Sprintf("%.1f-%.1f", stats.FPSMin, stats.FPSMax),
		"stable", stats.IsStable,
	)

	if !stats.IsStable {
		slog.Warn("stream FPS is unstable, may affect inference timing",
			"fps_stddev", stats.FPSStdDev,
		)
	}

	return stats, nil
}

// calculateFPSStats calculates FPS statistics from frame timestamps
func calculateFPSStats(frameTimes []time.Time, totalDuration time.Duration) *WarmupStats {
	n := len(frameTimes)

	// Calculate mean FPS (overall)
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

	if len(instantaneousFPS) == 0 {
		return &WarmupStats{
			FramesReceived: n,
			Duration:       totalDuration,
			FPSMean:        fpsMean,
			IsStable:       false,
		}
	}

	// Calculate min/max FPS
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

	// Determine if stream is stable
	// Stable if stddev < 15% of mean FPS
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

// CalculateOptimalInferenceRate calculates optimal inference rate based on warm-up stats
// Returns inference rate in Hz (never exceeds maxRate)
func CalculateOptimalInferenceRate(warmupStats *WarmupStats, maxRate float64) float64 {
	if warmupStats == nil {
		return maxRate
	}

	// Always respect max rate (typically 1.0 Hz)
	optimalRate := maxRate

	// If stream FPS is very low, reduce inference rate accordingly
	if warmupStats.FPSMean < maxRate {
		optimalRate = warmupStats.FPSMean * 0.9 // 90% of stream FPS to be safe
		slog.Info("reducing inference rate due to low stream FPS",
			"stream_fps", warmupStats.FPSMean,
			"max_rate", maxRate,
			"optimal_rate", optimalRate,
		)
	}

	return optimalRate
}

// CalculateProcessInterval calculates process_interval from inference rate and stream FPS
// Returns number of frames to skip between inferences
func CalculateProcessInterval(streamFPS float64, inferenceRateHz float64) int {
	if streamFPS <= 0 || inferenceRateHz <= 0 {
		return 10 // Safe default
	}

	// interval = stream_fps / inference_rate
	// Example: 6 FPS stream, 1 Hz inference → interval = 6/1 = 6 frames
	interval := int(math.Ceil(streamFPS / inferenceRateHz))

	if interval < 1 {
		interval = 1 // Process at least every frame
	}

	slog.Debug("calculated process interval",
		"stream_fps", streamFPS,
		"inference_rate_hz", inferenceRateHz,
		"process_interval", interval,
	)

	return interval
}
