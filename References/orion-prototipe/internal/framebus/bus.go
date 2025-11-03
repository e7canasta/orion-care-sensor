package framebus

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/care/orion/internal/types"
)

// Bus distributes frames to multiple workers with drop policy
type Bus struct {
	workers []types.InferenceWorker

	mu                sync.RWMutex
	framesDistributed uint64
	droppedByWorker   map[string]uint64
}

// New creates a new FrameBus
func New() *Bus {
	return &Bus{
		workers:         make([]types.InferenceWorker, 0),
		droppedByWorker: make(map[string]uint64),
	}
}

// Register adds a worker to receive frames
func (b *Bus) Register(worker types.InferenceWorker) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.workers = append(b.workers, worker)
	b.droppedByWorker[worker.ID()] = 0

	slog.Info("worker registered to framebus",
		"worker_id", worker.ID(),
		"total_workers", len(b.workers),
	)
}

// Unregister removes a worker from receiving frames
func (b *Bus) Unregister(worker types.InferenceWorker) {
	b.mu.Lock()
	defer b.mu.Unlock()

	workerID := worker.ID()

	// Find and remove worker from list
	for i, w := range b.workers {
		if w.ID() == workerID {
			// Remove by replacing with last element and truncating
			b.workers[i] = b.workers[len(b.workers)-1]
			b.workers = b.workers[:len(b.workers)-1]
			break
		}
	}

	// Remove from dropped stats
	delete(b.droppedByWorker, workerID)

	slog.Info("worker unregistered from framebus",
		"worker_id", workerID,
		"total_workers", len(b.workers),
	)
}

// Distribute sends a frame to all registered workers
// Uses non-blocking send with drop policy (buffer=1 per worker)
func (b *Bus) Distribute(ctx context.Context, frame types.Frame) error {
	b.mu.RLock()
	workers := b.workers
	b.mu.RUnlock()

	for _, worker := range workers {
		// Non-blocking send
		if err := worker.SendFrame(frame); err != nil {
			// Frame was dropped (buffer full)
			b.mu.Lock()
			b.droppedByWorker[worker.ID()]++
			b.mu.Unlock()

			slog.Debug("frame dropped for worker",
				"worker_id", worker.ID(),
				"frame_seq", frame.Seq,
				"trace_id", frame.TraceID,
			)
		}
	}

	b.mu.Lock()
	b.framesDistributed++
	b.mu.Unlock()

	return nil
}

// Stats returns bus statistics
func (b *Bus) Stats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	dropped := make(map[string]uint64)
	for k, v := range b.droppedByWorker {
		dropped[k] = v
	}

	return Stats{
		WorkersCount:      len(b.workers),
		FramesDistributed: b.framesDistributed,
		DroppedByWorker:   dropped,
	}
}

// Stats contains bus statistics
type Stats struct {
	WorkersCount      int
	FramesDistributed uint64
	DroppedByWorker   map[string]uint64
}

// Start starts all registered workers
func (b *Bus) Start(ctx context.Context) error {
	b.mu.RLock()
	workers := b.workers
	b.mu.RUnlock()

	slog.Info("starting framebus workers", "count", len(workers))

	for _, worker := range workers {
		if err := worker.Start(ctx); err != nil {
			return err
		}
	}

	return nil
}

// Stop stops all workers
func (b *Bus) Stop() error {
	b.mu.RLock()
	workers := b.workers
	b.mu.RUnlock()

	slog.Info("stopping framebus workers", "count", len(workers))

	for _, worker := range workers {
		if err := worker.Stop(); err != nil {
			slog.Error("failed to stop worker", "worker_id", worker.ID(), "error", err)
		}
	}

	return nil
}

// StartStatsLogger starts periodic stats logging with drop rate alerting
func (b *Bus) StartStatsLogger(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Track previous stats to calculate delta drop rates
	prevStats := b.Stats()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			stats := b.Stats()

			// Calculate drop rate deltas
			deltaDistributed := stats.FramesDistributed - prevStats.FramesDistributed

			for workerID, dropped := range stats.DroppedByWorker {
				prevDropped := prevStats.DroppedByWorker[workerID]
				deltaDropped := dropped - prevDropped

				// Alert if drop rate > 80% in this interval
				if deltaDistributed > 0 {
					dropRate := float64(deltaDropped) / float64(deltaDistributed)

					if dropRate > 0.80 {
						slog.Warn("worker high drop rate detected",
							"worker_id", workerID,
							"drop_rate_pct", int(dropRate*100),
							"dropped_last_interval", deltaDropped,
							"frames_last_interval", deltaDistributed,
							"action", "check worker health")
					}
				}
			}

			// Log regular stats
			fields := []interface{}{
				"workers", stats.WorkersCount,
				"distributed", stats.FramesDistributed,
			}

			for workerID, dropped := range stats.DroppedByWorker {
				if dropped > 0 {
					fields = append(fields, workerID+"_dropped", dropped)
				}
			}

			slog.Debug("framebus stats", fields...)

			// Update previous stats for next interval
			prevStats = stats
		}
	}
}
