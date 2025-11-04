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
	IsStable       bool          // True if FPS is stable (stddev < 15% of mean)
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

	// Calculate FPS statistics
	stats := calculateFPSStats(frameTimes, elapsed)

	slog.Info("warmup: stream warm-up complete",
		"frames", stats.FramesReceived,
		"duration", stats.Duration,
		"fps_mean", fmt.Sprintf("%.2f", stats.FPSMean),
		"fps_stddev", fmt.Sprintf("%.2f", stats.FPSStdDev),
		"fps_range", fmt.Sprintf("%.1f-%.1f", stats.FPSMin, stats.FPSMax),
		"stable", stats.IsStable,
	)

	if !stats.IsStable {
		slog.Warn("warmup: stream FPS is unstable, may affect inference timing",
			"fps_stddev", stats.FPSStdDev,
			"fps_mean", stats.FPSMean,
		)
	}

	return stats, nil
}
