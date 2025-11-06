package main

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/e7canasta/orion-care-sensor/modules/framesupplier"
)

// MockWorker simulates a real inference worker with configurable processing latency.
//
// It demonstrates realistic frame consumption:
// - Validates RGB frame data integrity (dimensions, size)
// - Simulates inference processing time (configurable latency)
// - Collects statistics (frames processed, drops, timing)
//
// Used to validate FrameSupplier's non-blocking distribution and drop policy.
//
// Note: Consumes RGB raw data (like real ONNX workers), not JPEG.
type MockWorker struct {
	id      string
	latency time.Duration
	logger  *slog.Logger

	// Statistics
	processed      atomic.Uint64
	drops          atomic.Uint64
	totalLatencyMs atomic.Uint64
}

// NewMockWorker creates a worker with given ID and processing latency.
func NewMockWorker(id string, latency time.Duration, logger *slog.Logger) *MockWorker {
	return &MockWorker{
		id:      id,
		latency: latency,
		logger:  logger.With("worker", id),
	}
}

// Run starts the worker's consumption loop.
//
// It subscribes to the FrameSupplier and processes frames until context cancellation.
func (w *MockWorker) Run(ctx context.Context, supplier framesupplier.Supplier) error {
	w.logger.Info("Worker started", "latency", w.latency)

	// Subscribe to FrameSupplier
	readFunc := supplier.Subscribe(w.id)
	defer supplier.Unsubscribe(w.id)

	for {
		select {
		case <-ctx.Done():
			w.logger.Info("Worker stopping gracefully")
			return ctx.Err()
		default:
			// Blocking read from worker mailbox
			frame := readFunc()

			// Process frame (decode + simulate inference)
			start := time.Now()
			if err := w.processFrame(frame); err != nil {
				w.logger.Error("Frame processing failed", "error", err)
				continue
			}
			elapsed := time.Since(start)

			// Update statistics
			w.processed.Add(1)
			w.totalLatencyMs.Add(uint64(elapsed.Milliseconds()))

			w.logger.Debug("Frame processed",
				"seq", frame.Seq,
				"elapsed_ms", elapsed.Milliseconds())
		}
	}
}

// processFrame simulates realistic frame processing:
// 1. Validate RGB data integrity (dimensions, size)
// 2. Simulate inference latency (like ONNX forward pass)
func (w *MockWorker) processFrame(frame *framesupplier.Frame) error {
	// Validate RGB data dimensions
	// RGB format: 3 bytes per pixel (R, G, B), no alpha channel
	expectedSize := frame.Width * frame.Height * 3
	if len(frame.Data) != expectedSize {
		return fmt.Errorf("invalid RGB data size: got %d bytes, expected %d (%dx%d*3)",
			len(frame.Data), expectedSize, frame.Width, frame.Height)
	}

	// Validate dimensions are reasonable
	if frame.Width <= 0 || frame.Height <= 0 {
		return fmt.Errorf("invalid frame dimensions: %dx%d", frame.Width, frame.Height)
	}

	// Simulate inference processing time (ONNX forward pass)
	time.Sleep(w.latency)

	return nil
}

// Stats returns current worker statistics.
func (w *MockWorker) Stats() WorkerStats {
	processed := w.processed.Load()
	totalMs := w.totalLatencyMs.Load()

	var avgMs float64
	if processed > 0 {
		avgMs = float64(totalMs) / float64(processed)
	}

	return WorkerStats{
		ID:         w.id,
		Processed:  processed,
		Drops:      w.drops.Load(),
		AvgLatency: time.Duration(avgMs * float64(time.Millisecond)),
	}
}

// WorkerStats holds statistics for a single worker.
type WorkerStats struct {
	ID         string
	Processed  uint64
	Drops      uint64
	AvgLatency time.Duration
}

// DropRate calculates percentage of frames dropped.
func (s WorkerStats) DropRate() float64 {
	total := s.Processed + s.Drops
	if total == 0 {
		return 0.0
	}
	return float64(s.Drops) / float64(total) * 100.0
}
