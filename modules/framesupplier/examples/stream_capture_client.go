// Package examples demonstrates how to use framesupplier from client perspective.
//
// This file shows stream-capture module usage (PUBLISHER side).
package examples

import (
	"context"
	"log"
	"time"

	"github.com/e7canasta/orion-care-sensor/modules/framesupplier"
)

// StreamCaptureClient demonstrates how stream-capture module publishes frames.
//
// Context: GStreamer appsink delivers frames @ 30fps.
// Requirement: Publish() must be non-blocking (GStreamer can't wait).
func StreamCaptureClient() {
	// Create supplier (constructor pattern)
	supplier := framesupplier.New()

	// Start distribution goroutine (blocks until stopped)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := supplier.Start(ctx); err != nil {
			log.Fatalf("Supplier failed: %v", err)
		}
	}()

	// Simulate GStreamer callback @ 30fps
	ticker := time.NewTicker(33 * time.Millisecond) // ~30fps
	defer ticker.Stop()

	frameCount := 0
	for {
		select {
		case <-ticker.C:
			// Capture frame from GStreamer (simulated)
			frame := captureFrameFromGStreamer()

			// Publish to supplier
			// REQUIREMENT: Non-blocking, completes in ~1µs
			supplier.Publish(frame)

			frameCount++

		case <-ctx.Done():
			return
		}
	}
}

// captureFrameFromGStreamer simulates frame capture.
// In real code: GStreamer appsink → C.GoBytes → *Frame
func captureFrameFromGStreamer() *framesupplier.Frame {
	// In real implementation:
	// 1. GStreamer appsink emits GstSample (C memory)
	// 2. Extract GstBuffer from sample
	// 3. Map buffer to read pointer
	// 4. C.GoBytes() - THE ONLY COPY (CGo boundary)
	// 5. Construct *Frame with Go-owned []byte

	jpegData := make([]byte, 100*1024) // Simulated 100KB JPEG
	return &framesupplier.Frame{
		Data:      jpegData,
		Width:     1920,
		Height:    1080,
		Timestamp: time.Now(),
		// Seq is assigned by Supplier, not by publisher
	}
}

// MonitoringExample shows how stream-capture monitors supplier health.
func MonitoringExample(supplier framesupplier.Supplier) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		stats := supplier.Stats()

		// Check inbox drops (should be ~0 in healthy system)
		if stats.InboxDrops > 0 {
			log.Printf("WARNING: Inbox drops detected: %d (distribution loop slow?)", stats.InboxDrops)
		}

		// Check worker health
		for workerID, workerStats := range stats.Workers {
			if workerStats.IsIdle {
				log.Printf("WARNING: Worker %s is idle (last consume: %v)",
					workerID, workerStats.LastConsumedAt)
			}

			// Log drops (expected for slow workers)
			dropRate := float64(workerStats.TotalDrops) / float64(workerStats.LastConsumedSeq)
			log.Printf("Worker %s: %.1f%% drops (%d total)",
				workerID, dropRate*100, workerStats.TotalDrops)
		}
	}
}
