package internal

import (
	"sync/atomic"
	"time"
)

// idleThreshold defines when a worker is considered idle (no consume activity).
//
// Rationale:
//   - Inference @ 1fps: Expected consume every 1 second
//   - Inference @ 0.1fps: Expected consume every 10 seconds
//   - Threshold: 30 seconds (3× slowest expected rate)
//
// Use case:
//   - Critical workers (PersonDetector): IsIdle → restart required
//   - BestEffort workers (VLM): IsIdle → acceptable (low priority)
//
// See: ARCHITECTURE.md (Idle Detection)
const idleThreshold = 30 * time.Second

// Stats returns operational statistics snapshot (implements Supplier.Stats).
//
// Returns:
//   - InboxDrops: Atomic read (safe without lock)
//   - Workers: Map of workerID → WorkerStats (snapshot at call time)
//
// Semantics:
//   - Non-blocking: Returns immediately (snapshot, not live view)
//   - Thread-safe: Safe for concurrent calls with Publish/Subscribe/etc
//   - Consistency: Stats may be slightly stale (acceptable for monitoring)
//
// Use cases:
//   - Monitor inbox drops (should be ~0 in healthy system)
//   - Detect idle workers (health checks, restart policies)
//   - SLA compliance (drop rate thresholds)
//
// Thread-safety: All reads are lock-protected or atomic.
//
// See: ARCHITECTURE.md (Operational Monitoring)
func (s *supplier) Stats() SupplierStats {
	// Read inbox drops (atomic, no lock needed)
	inboxDrops := atomic.LoadUint64(&s.inboxDrops)

	// Collect per-worker stats
	workers := make(map[string]WorkerStats)

	s.slots.Range(func(key, value interface{}) bool {
		workerID := key.(string)
		slot := value.(*WorkerSlot)

		// Lock slot to read stats fields (consistent snapshot)
		slot.mu.Lock()

		// Calculate idle status
		isIdle := time.Since(slot.lastConsumedAt) > idleThreshold

		// Build WorkerStats
		stat := WorkerStats{
			WorkerID:         workerID,
			LastConsumedAt:   slot.lastConsumedAt,
			LastConsumedSeq:  slot.lastConsumedSeq,
			ConsecutiveDrops: slot.consecutiveDrops,
			TotalDrops:       slot.totalDrops,
			IsIdle:           isIdle,
		}

		slot.mu.Unlock()

		workers[workerID] = stat
		return true // Continue iteration
	})

	return SupplierStats{
		InboxDrops: inboxDrops,
		Workers:    workers,
	}
}
