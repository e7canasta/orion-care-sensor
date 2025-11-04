package warmup

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Frame is a minimal frame struct for internal use (avoids import cycle)
type Frame struct {
	Seq       uint64
	Timestamp time.Time
}

// WarmupStats contains statistics collected during warm-up phase
type WarmupStats struct {
	FramesReceived int           // Number of frames received during warm-up
	Duration       time.Duration // Actual warm-up duration
	FPSMean        float64       // Mean FPS across all frames
	FPSStdDev      float64       // Standard deviation of FPS
	FPSMin         float64       // Minimum instantaneous FPS
	FPSMax         float64       // Maximum instantaneous FPS
	IsStable       bool          // True if FPS is stable (stddev < 15% of mean AND jitter < 20%)
	JitterMean     float64       // Average inter-frame interval variance (seconds)
	JitterStdDev   float64       // Standard deviation of jitter (seconds)
	JitterMax      float64       // Maximum jitter observed (seconds)
}

// WarmupStream warms up the stream by consuming frames for the specified duration
//
// This function:
//  1. Consumes frames from the channel without processing them
//  2. Tracks frame arrival times to measure FPS statistics
//  3. Calculates FPS mean, standard deviation, min, max
//  4. Determines if the stream is stable (stddev < 15% of mean)
//
// The warm-up phase allows the GStreamer pipeline to stabilize and provides
// accurate FPS measurements for the stream.
//
// Returns WarmupStats with collected statistics, or an error if:
//   - Stream closes during warm-up
//   - Not enough frames received (< 2)
//   - Context is cancelled
func WarmupStream(
	ctx context.Context,
	frames <-chan Frame,
	duration time.Duration,
) (*WarmupStats, error) {
	slog.Info("warmup: starting stream warm-up",
		"duration", duration,
		"reason", "measure real FPS and stabilize pipeline",
	)

	startTime := time.Now()

	// Track frame timestamps
	frameTimes := make([]time.Time, 0, 100) // Pre-allocate for ~30 FPS @ 3s

	// Create timeout context
	warmupCtx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	// Consume frames during warm-up
	for {
		select {
		case <-warmupCtx.Done():
			// Warm-up duration elapsed - analyze statistics
			goto analyze

		case frame, ok := <-frames:
			if !ok {
				return nil, fmt.Errorf("warmup: stream closed during warm-up")
			}

			// Record frame timestamp
			frameTimes = append(frameTimes, frame.Timestamp)

			slog.Debug("warmup: frame received",
				"seq", frame.Seq,
				"frames_collected", len(frameTimes),
			)
		}
	}

analyze:
	elapsed := time.Since(startTime)

	// Validate minimum frames received
	if len(frameTimes) < 2 {
		return nil, fmt.Errorf(
			"warmup: not enough frames received (got %d, need at least 2)",
			len(frameTimes),
		)
	}

	// Calculate FPS statistics (local function, no external dependencies)
	stats := CalculateFPSStats(frameTimes, elapsed)

	slog.Info("warmup: stream warm-up complete",
		"frames", stats.FramesReceived,
		"duration", stats.Duration,
		"fps_mean", fmt.Sprintf("%.2f", stats.FPSMean),
		"fps_stddev", fmt.Sprintf("%.2f", stats.FPSStdDev),
		"fps_range", fmt.Sprintf("%.1f-%.1f", stats.FPSMin, stats.FPSMax),
		"jitter_mean", fmt.Sprintf("%.3fs", stats.JitterMean),
		"stable", stats.IsStable,
	)

	// Fail-fast: Warmup MUST verify stream stability before production use
	// Unstable FPS indicates network issues, camera problems, or pipeline misconfiguration
	if !stats.IsStable {
		return nil, fmt.Errorf(
			"warmup: stream FPS unstable (mean=%.2f Hz, stddev=%.2f, jitter=%.3fs, threshold: FPS<15%%, jitter<20%%)",
			stats.FPSMean,
			stats.FPSStdDev,
			stats.JitterMean,
		)
	}

	return stats, nil
}
