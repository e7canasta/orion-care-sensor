package streamcapture

import "context"

// StreamProvider defines the contract for video stream acquisition
//
// Implementations must guarantee:
//   - Start() blocks until warm-up completes (~5 seconds)
//   - Start() returns a channel that never closes until Stop()
//   - Stop() is idempotent (safe to call multiple times)
//   - Stats() is thread-safe (can be called from any goroutine)
//   - SetTargetFPS() does not require restart (hot-reload)
type StreamProvider interface {
	// Start initializes the stream and returns a read-only channel of frames.
	//
	// This method blocks for approximately 5 seconds during warm-up to measure
	// FPS stability. Once warm-up completes, the returned channel will deliver
	// frames at the configured target FPS.
	//
	// The returned channel will remain open until Stop() is called. Frames are
	// sent using a non-blocking pattern - if the channel buffer is full, frames
	// are dropped rather than queued to maintain low latency.
	//
	// Returns an error if:
	//   - The stream cannot be established
	//   - GStreamer is not available
	//   - Warm-up fails to complete
	//
	// Example:
	//   stream, _ := NewRTSPStream(cfg)
	//   frameChan, err := stream.Start(ctx)
	//   if err != nil {
	//       log.Fatal(err)
	//   }
	//   for frame := range frameChan {
	//       // Process frame...
	//   }
	Start(ctx context.Context) (<-chan Frame, error)

	// Stop gracefully shuts down the stream.
	//
	// This method:
	//   1. Cancels the internal context to signal shutdown
	//   2. Waits up to 3 seconds for goroutines to finish
	//   3. Closes the frame channel
	//   4. Cleans up GStreamer resources
	//
	// Safe to call multiple times (idempotent). If called when the stream
	// is not running, returns nil immediately.
	//
	// Returns an error if shutdown timeout is exceeded (3 seconds).
	Stop() error

	// Stats returns current stream statistics.
	//
	// This method is thread-safe and can be called from any goroutine.
	// Statistics are updated atomically during stream operation.
	//
	// Returns StreamStats with current values. If the stream is not running,
	// some fields (e.g., FPSReal, LatencyMS) may be zero or stale.
	Stats() StreamStats

	// SetTargetFPS updates the target FPS dynamically without restarting the stream.
	//
	// This triggers a hot-reload of the GStreamer pipeline, causing approximately
	// 2 seconds of interruption while the pipeline adjusts its caps. This is
	// significantly faster than a full restart (5-10 seconds).
	//
	// The FPS value must be between 0.1 and 30.0. Values outside this range
	// will return an error.
	//
	// If the update fails (e.g., GStreamer error), the previous FPS value is
	// restored (rollback).
	//
	// Returns an error if:
	//   - FPS is out of range (< 0.1 or > 30.0)
	//   - GStreamer caps update fails
	//   - Stream is not currently running
	//
	// Example:
	//   err := stream.SetTargetFPS(0.5)  // Change to 0.5 Hz (1 frame every 2 seconds)
	SetTargetFPS(fps float64) error
}
