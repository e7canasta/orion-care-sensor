package internal

import (
	"sync/atomic"
)

// publishBatchSize is the threshold for switching from sequential to parallel distribution.
//
// Rationale (ADR-003):
//   - Sequential cost: N workers × 1µs (acceptable for small N)
//   - Goroutine spawn cost: ~2µs + 2KB stack
//   - Break-even: ~12 workers (sequential = parallel)
//   - Threshold: 8 workers (conservative, before break-even)
//
// Context:
//   - POC deployments: ≤5 workers (sequential optimal)
//   - Expansion: ≤10 workers (sequential still good)
//   - Full deployment: ≤64 workers (batching kicks in)
//
// Guardrail: Prevents premature optimization (YAGNI) while enabling scale.
//
// See: ADR-003 (Batching with Threshold=8), ARCHITECTURE.md (Algorithm 3)
const publishBatchSize = 8

// distributeToWorkers fans out a frame to all worker slots with threshold-based batching.
//
// Algorithm (ADR-003):
//  1. Assign global sequence number to frame (atomic increment)
//  2. Snapshot worker slots (sync.Map → slice)
//  3. Decision tree:
//     - If ≤8 workers: Sequential for-loop (0 goroutines spawned)
//     - If >8 workers: Parallel batching (spawn ⌈N/8⌉ goroutines)
//  4. Fire-and-forget: No wg.Wait (ordering guaranteed by physics)
//
// Ordering Guarantee (Physical Invariant):
//   - Distribution latency: ~100µs @ 64 workers
//   - Inter-frame @ 1fps: 1,000,000µs
//   - Ratio: 10,000× (distribution completes 10,000× faster than next frame arrives)
//   - Result: Frame N+1 cannot overtake N (impossible by timing)
//
// Thread-safety:
//   - Called by distributionLoop (single goroutine)
//   - Spawns transient goroutines for batching (0-N/8)
//
// Latency:
//   - Sequential (≤8w): N × 1µs (e.g., 8w = 8µs)
//   - Parallel (>8w): ~20µs spawn + max(batch) (e.g., 64w = 30µs total)
//
// See: ADR-003 (Batching), ARCHITECTURE.md (Algorithm 3, Fire-and-forget rationale)
func (s *supplier) distributeToWorkers(frame *Frame) {
	// Assign global sequence number (monotonically increasing)
	frame.Seq = atomic.AddUint64(&s.publishSeq, 1)

	// Snapshot worker slots (sync.Map → slice)
	// Reason: sync.Map.Range is safe but not suitable for nested iteration
	var slots []*WorkerSlot
	s.slots.Range(func(key, value interface{}) bool {
		slots = append(slots, value.(*WorkerSlot))
		return true // Continue iteration
	})

	workerCount := len(slots)

	// Fast path: No workers registered (no-op)
	if workerCount == 0 {
		return
	}

	// Sequential path: ≤8 workers (0 goroutines spawned)
	if workerCount <= publishBatchSize {
		for _, slot := range slots {
			s.publishToSlot(slot, frame)
		}
		return
	}

	// Parallel path: >8 workers (fire-and-forget batching)
	// Spawn ⌈workerCount / publishBatchSize⌉ goroutines
	// Each goroutine processes publishBatchSize slots (last batch may be smaller)
	for i := 0; i < workerCount; i += publishBatchSize {
		end := i + publishBatchSize
		if end > workerCount {
			end = workerCount
		}

		batch := slots[i:end]

		// Fire-and-forget: No wg.Wait (ordering guaranteed by physical invariant)
		// Closure captures batch slice (safe: frame and batch immutable after capture)
		go func(b []*WorkerSlot) {
			for _, slot := range b {
				s.publishToSlot(slot, frame)
			}
		}(batch)
	}

	// No wg.Wait: Fire-and-forget
	// Rationale: Distribution latency (100µs) << inter-frame interval (33ms @ 30fps)
	// Physical invariant: Frame N+1 cannot arrive before N's distribution completes
}
