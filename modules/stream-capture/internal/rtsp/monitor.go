package rtsp

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/tinyzimmer/go-gst/gst"
)

// ErrorCounters holds atomic counters for different error categories
type ErrorCounters struct {
	Network *uint64 // Network-related errors (connection, timeout, DNS)
	Codec   *uint64 // Codec/stream errors (decode failures, format issues)
	Auth    *uint64 // Authentication/authorization errors
	Unknown *uint64 // Unclassified errors
}

// MonitorMetrics holds stream metrics for monitoring
type MonitorMetrics struct {
	RTSPURL        string
	Resolution     string
	FrameCount     *uint64
	ReconnectCount *uint32
	StartedAt      time.Time
}

// MonitorPipelineBus monitors the GStreamer pipeline bus for messages
//
// This function:
//  1. Polls pipeline bus for messages (EOS, Error, StateChanged)
//  2. Classifies errors for telemetry
//  3. Updates error counters atomically
//  4. Resets reconnection state on PLAYING transition
//
// Returns an error if the pipeline encounters an error (triggers reconnection).
// Returns nil if context is cancelled (graceful shutdown).
//
// Parameters:
//   - ctx: Context for cancellation
//   - pipeline: GStreamer pipeline to monitor
//   - errorCounters: Atomic counters for error telemetry
//   - reconnectState: Reconnection state (reset on PLAYING)
//   - metrics: Stream metrics for logging
func MonitorPipelineBus(
	ctx context.Context,
	pipeline *gst.Pipeline,
	errorCounters *ErrorCounters,
	reconnectState *ReconnectState,
	metrics *MonitorMetrics,
) error {
	if pipeline == nil {
		return fmt.Errorf("pipeline not initialized")
	}

	bus := pipeline.GetPipelineBus()

	for {
		select {
		case <-ctx.Done():
			slog.Debug("stream-capture: context cancelled, stopping pipeline monitor")
			return nil

		default:
			// Poll for messages with short timeout for responsive shutdown
			msg := bus.TimedPop(50 * time.Millisecond)
			if msg == nil {
				continue
			}

			switch msg.Type() {
			case gst.MessageEOS:
				slog.Info("stream-capture: end of stream received",
					"rtsp_url", metrics.RTSPURL,
					"uptime", time.Since(metrics.StartedAt),
					"frames_processed", atomic.LoadUint64(metrics.FrameCount),
				)
				return fmt.Errorf("end of stream")

			case gst.MessageError:
				gerr := msg.ParseError()

				// Classify error for telemetry
				category := ClassifyGStreamerError(gerr)

				// Update error counters (atomic)
				switch category {
				case ErrCategoryNetwork:
					atomic.AddUint64(errorCounters.Network, 1)
				case ErrCategoryCodec:
					atomic.AddUint64(errorCounters.Codec, 1)
				case ErrCategoryAuth:
					atomic.AddUint64(errorCounters.Auth, 1)
				case ErrCategoryUnknown:
					atomic.AddUint64(errorCounters.Unknown, 1)
				}

				slog.Error("stream-capture: pipeline error",
					"error", gerr.Error(),
					"debug", gerr.DebugString(),
					"category", category.String(),
					"rtsp_url", metrics.RTSPURL,
					"resolution", metrics.Resolution,
					"uptime", time.Since(metrics.StartedAt),
					"frames_processed", atomic.LoadUint64(metrics.FrameCount),
					"reconnects", atomic.LoadUint32(reconnectState.Reconnects),
				)
				// Return error to trigger reconnection
				return fmt.Errorf("pipeline error [%s]: %s", category.String(), gerr.Error())

			case gst.MessageStateChanged:
				if msg.Source() == pipeline.GetName() {
					old, new := msg.ParseStateChanged()
					slog.Debug("stream-capture: pipeline state changed",
						"from", old,
						"to", new,
					)

					// Reset reconnection state when reaching PLAYING state
					if new == gst.StatePlaying {
						ResetReconnectState(reconnectState)
						slog.Info("stream-capture: pipeline playing, reconnect state reset")
					}
				}
			}
		}
	}
}
