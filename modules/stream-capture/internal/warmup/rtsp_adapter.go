package warmup

import (
	"context"
	"fmt"
	"time"
)

const (
	// adapterBufferSize is the buffer size for the frame adapter channel.
	// This provides a small buffer to handle frame conversion without blocking.
	adapterBufferSize = 10

	// adapterTimeout is the safety margin added to warmup duration for the adapter goroutine.
	// This prevents premature timeout in the adapter while warmup is still collecting frames.
	adapterTimeout = 1 * time.Second
)

// WarmupWithAdapter performs warmup by converting frames from an external source channel.
//
// This function bridges external frame types to warmup.Frame, avoiding import cycles.
// It launches a goroutine to convert frames, then delegates to WarmupStream.
//
// Type parameters:
//   T: External frame type (must have Seq uint64 and Timestamp time.Time fields)
//
// Parameters:
//   ctx: Cancellation context
//   sourceFrames: Channel of external frames (e.g., streamcapture.Frame)
//   duration: Warmup duration
//   converter: Function to convert T → warmup.Frame
//
// Returns warmup statistics or error if warmup fails.
//
// Example usage from rtsp.go:
//   stats, err := warmup.WarmupWithAdapter(ctx, s.frames, duration, func(f Frame) warmup.Frame {
//       return warmup.Frame{Seq: f.Seq, Timestamp: f.Timestamp}
//   })
func WarmupWithAdapter[T any](
	ctx context.Context,
	sourceFrames <-chan T,
	duration time.Duration,
	converter func(T) Frame,
) (*WarmupStats, error) {
	// Create adapter channel
	adaptedFrames := make(chan Frame, adapterBufferSize)

	// Launch converter goroutine
	go func() {
		defer close(adaptedFrames)

		// Create timeout context for adapter goroutine
		adapterCtx, cancel := context.WithTimeout(ctx, duration+adapterTimeout)
		defer cancel()

		for {
			select {
			case <-adapterCtx.Done():
				return
			case frame, ok := <-sourceFrames:
				if !ok {
					return
				}
				// Convert external frame → warmup.Frame
				warmupFrame := converter(frame)

				select {
				case adaptedFrames <- warmupFrame:
					// Sent successfully
				case <-adapterCtx.Done():
					return
				}
			}
		}
	}()

	// Delegate to warmup helper (includes fail-fast validation)
	stats, err := WarmupStream(ctx, adaptedFrames, duration)
	if err != nil {
		return nil, fmt.Errorf("warmup adapter failed: %w", err)
	}

	return stats, nil
}
