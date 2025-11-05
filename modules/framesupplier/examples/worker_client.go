// Package examples demonstrates how to use framesupplier from client perspective.
//
// This file shows worker module usage (CONSUMER side).
package examples

import (
	"context"
	"log"
	"time"

	"github.com/e7canasta/orion-care-sensor/modules/framesupplier"
)

// WorkerClient demonstrates how a worker (PersonDetector, PoseDetector, etc) consumes frames.
//
// Context: Worker runs inference at variable rate (1-30fps depending on model complexity).
// Requirement: Subscribe() returns blocking function (efficient wait, no busy-loop).
func WorkerClient(supplier framesupplier.Supplier, workerID string) {
	// Subscribe to frame stream
	// Returns: blocking read function (mailbox semantics)
	readFunc := supplier.Subscribe(workerID)
	defer supplier.Unsubscribe(workerID)

	log.Printf("[%s] Worker started, waiting for frames...", workerID)

	// Worker loop: blocking read → inference → repeat
	for {
		// BLOCKING CALL: Waits until frame available
		// Returns nil on graceful shutdown (Unsubscribe or Stop)
		frame := readFunc()
		if frame == nil {
			log.Printf("[%s] Worker shutting down (nil frame)", workerID)
			break
		}

		// Process frame (simulate inference)
		startTime := time.Now()
		result := runInference(workerID, frame)
		inferenceTime := time.Since(startTime)

		log.Printf("[%s] Processed frame seq=%d, inference=%v",
			workerID, frame.Seq, inferenceTime)

		// Publish result (to MQTT, etc - not FrameSupplier's concern)
		publishResult(workerID, result)
	}

	log.Printf("[%s] Worker stopped", workerID)
}

// CriticalWorkerClient shows a life-critical worker (PersonDetector for fall detection).
//
// Difference: Cannot tolerate drops, needs alerting on high drop rate.
func CriticalWorkerClient(supplier framesupplier.Supplier, workerID string) {
	readFunc := supplier.Subscribe(workerID)
	defer supplier.Unsubscribe(workerID)

	// Monitor drops every 10 frames
	frameCount := 0
	alertThreshold := 0.05 // 5% drop rate = alert

	for {
		frame := readFunc()
		if frame == nil {
			break
		}

		frameCount++

		// Check drops periodically
		if frameCount%10 == 0 {
			stats := supplier.Stats()
			if workerStats, ok := stats.Workers[workerID]; ok {
				dropRate := float64(workerStats.TotalDrops) / float64(workerStats.LastConsumedSeq)
				if dropRate > alertThreshold {
					log.Printf("[%s] CRITICAL: Drop rate %.1f%% exceeds threshold %.1f%%",
						workerID, dropRate*100, alertThreshold*100)
					// In real system: trigger alert to EdgeExpert
				}
			}
		}

		// Process frame
		runInference(workerID, frame)
	}
}

// BestEffortWorkerClient shows a low-priority worker (VLM, analytics, etc).
//
// Difference: Tolerates drops, no alerting needed.
func BestEffortWorkerClient(supplier framesupplier.Supplier, workerID string) {
	readFunc := supplier.Subscribe(workerID)
	defer supplier.Unsubscribe(workerID)

	for {
		frame := readFunc()
		if frame == nil {
			break
		}

		// Heavy inference (50-200ms)
		// Drops expected under high source FPS
		runHeavyInference(workerID, frame)
	}
}

// MultiStreamWorkerClient shows future multi-stream scenario (Phase 2).
//
// Note: Current design doesn't have streamID, but API should accommodate future extension.
func MultiStreamWorkerClient(supplier framesupplier.Supplier, workerID string, streamID string) {
	// Future API (Phase 2):
	// readFunc := supplier.SubscribeToStream(workerID, streamID)

	// For now: Subscribe gets all streams, worker filters
	readFunc := supplier.Subscribe(workerID)
	defer supplier.Unsubscribe(workerID)

	for {
		frame := readFunc()
		if frame == nil {
			break
		}

		// TODO Phase 2: frame.StreamID for filtering
		// if frame.StreamID != streamID {
		//     continue
		// }

		runInference(workerID, frame)
	}
}

// GracefulShutdownExample shows worker responding to context cancellation.
func GracefulShutdownExample(ctx context.Context, supplier framesupplier.Supplier, workerID string) {
	readFunc := supplier.Subscribe(workerID)
	defer supplier.Unsubscribe(workerID)

	for {
		// Pattern: Check ctx.Done before blocking read
		select {
		case <-ctx.Done():
			log.Printf("[%s] Context cancelled, unsubscribing...", workerID)
			return
		default:
		}

		// readFunc() is blocking, but Unsubscribe (via defer) will wake it
		frame := readFunc()
		if frame == nil {
			return
		}

		runInference(workerID, frame)
	}
}

// --- Simulated inference functions ---

func runInference(workerID string, frame *framesupplier.Frame) interface{} {
	// Simulate inference latency (20-50ms for YOLO)
	time.Sleep(20 * time.Millisecond)
	return map[string]interface{}{
		"detections":    []string{"person_1", "person_2"},
		"person_count":  2,
		"inference_ms":  20.0,
		"frame_seq":     frame.Seq,
		"frame_ts":      frame.Timestamp,
	}
}

func runHeavyInference(workerID string, frame *framesupplier.Frame) interface{} {
	// Simulate heavy inference (VLM, pose estimation: 100-200ms)
	time.Sleep(100 * time.Millisecond)
	return map[string]interface{}{
		"pose":         "sitting",
		"confidence":   0.92,
		"inference_ms": 100.0,
	}
}

func publishResult(workerID string, result interface{}) {
	// In real system: publish to MQTT, write to database, etc
	log.Printf("[%s] Result: %+v", workerID, result)
}
