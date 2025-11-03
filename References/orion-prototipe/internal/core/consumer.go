package core

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/care/orion/internal/types"
)

// consumeFrames consumes frames from the stream and distributes to workers
func (o *Orion) consumeFrames(ctx context.Context) {
	defer o.wg.Done()

	slog.Info("frame consumer started")

	frameCount := uint64(0)
	lastLog := time.Now()
	logInterval := 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			slog.Info("frame consumer stopping", "total_frames", frameCount)
			return

		case frame, ok := <-o.stream.Frames():
			if !ok {
				slog.Info("stream channel closed", "total_frames", frameCount)
				return
			}

			frameCount++

			// Skip distribution if paused
			if !o.isPausedCheck() {
				// Process frame with ROI processor (attaches ROI metadata for intelligent model selection)
				processedFrame := o.roiProcessor.ProcessFrame(frame)

				// Distribute processed frame to all workers via FrameBus
				if err := o.frameBus.Distribute(ctx, processedFrame); err != nil {
					slog.Error("failed to distribute frame", "error", err)
				}
			}

			// Log stats periodically
			if time.Since(lastLog) >= logInterval {
				streamStats := o.stream.Stats()
				busStats := o.frameBus.Stats()

				slog.Debug("pipeline stats",
					"frames_consumed", frameCount,
					"stream_fps_real", float64(int(streamStats.FPSReal*100))/100,
					"bus_distributed", busStats.FramesDistributed,
					"workers_active", busStats.WorkersCount,
					"last_seq", frame.Seq,
				)

				// Log dropped frames if any
				for workerID, dropped := range busStats.DroppedByWorker {
					if dropped > 0 {
						slog.Warn("worker dropping frames",
							"worker_id", workerID,
							"dropped_count", dropped,
						)
					}
				}

				lastLog = time.Now()
			}
		}
	}
}

// consumeInferences consumes inferences from all workers and logs them
// In later iterations, this will publish to MQTT
func (o *Orion) consumeInferences(ctx context.Context) {
	defer o.wg.Done()

	slog.Info("inference consumer started", "workers", len(o.workers))

	inferenceCount := make(map[string]uint64) // count per worker
	var countMu sync.Mutex                    // protect inferenceCount map

	// Local WaitGroup for worker consumer goroutines
	var workerWg sync.WaitGroup

	// Merge all worker result channels
	for _, worker := range o.workers {
		workerID := worker.ID()
		resultsCh := worker.Results()

		workerWg.Add(1)
		go func(id string, ch <-chan types.Inference) {
			defer workerWg.Done()

			for {
				select {
				case <-ctx.Done():
					return

				case inference, ok := <-ch:
					if !ok {
						slog.Debug("worker results channel closed", "worker_id", id)
						return
					}

					countMu.Lock()
					inferenceCount[id]++
					count := inferenceCount[id]
					countMu.Unlock()

					// === HYBRID AUTO-FOCUS FEEDBACK LOOP ===
					// Extract suggested ROI from Python inference and update processor
					if suggestedROI := inference.SuggestedROI(); suggestedROI != nil {
						o.roiProcessor.UpdateSuggestedROI(suggestedROI)
						slog.Debug("hybrid auto-focus: updated suggested ROI from python",
							"worker_id", id,
							"roi", suggestedROI,
						)
					}

					// Skip publishing if paused
					if !o.isPausedCheck() {
						// Publish to MQTT
						if err := o.emitter.Publish(inference); err != nil {
							slog.Error("failed to publish inference",
								"worker_id", id,
								"type", inference.Type(),
								"error", err,
							)
							continue
						}

						slog.Debug("inference published",
							"worker_id", id,
							"type", inference.Type(),
							"count", count,
						)
					} else {
						slog.Debug("inference skipped (paused)", "worker_id", id, "type", inference.Type())
					}
				}
			}
		}(workerID, resultsCh)
	}

	// Wait for context cancellation, then wait for all worker goroutines to finish
	<-ctx.Done()
	slog.Info("inference consumer stopping, waiting for worker consumers")
	workerWg.Wait()
	slog.Info("all worker consumers stopped")
}
