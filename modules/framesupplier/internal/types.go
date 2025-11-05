package internal

import "time"

// SupplierStats is a snapshot of supplier operational state.
type SupplierStats struct {
	// InboxDrops counts frames dropped at inbox (distribution loop slow).
	// Should be ~0 in healthy system (distribution is 330× faster than 30fps source).
	// Non-zero indicates: deadlock, CPU starvation, or design bug.
	InboxDrops uint64

	// Workers maps workerID to per-worker statistics.
	// Updated on every Subscribe/Unsubscribe/Publish cycle.
	Workers map[string]WorkerStats
}

// WorkerStats tracks per-worker operational state.
type WorkerStats struct {
	// WorkerID is the unique identifier for this worker.
	WorkerID string

	// LastConsumedAt is the timestamp of last successful consume (readFunc return).
	// Used for idle detection (IsIdle = time.Since > 30s).
	LastConsumedAt time.Time

	// LastConsumedSeq is the sequence number of last consumed frame.
	// Monotonically increasing (same as Frame.Seq).
	// Used for drop rate calculation: TotalDrops / LastConsumedSeq.
	LastConsumedSeq uint64

	// ConsecutiveDrops is the current streak of unconsumed frames.
	// Resets to 0 on successful consume.
	// Use case: detect sudden worker slowdown (was healthy, now struggling).
	ConsecutiveDrops uint64

	// TotalDrops is the lifetime count of dropped frames for this worker.
	// Use case: SLA compliance, trend analysis.
	TotalDrops uint64

	// IsIdle indicates worker hasn't consumed frame in >30s.
	// Calculated: time.Since(LastConsumedAt) > 30s.
	// Use case: health checks, restart policies (critical workers).
	IsIdle bool
}
