package internal

import (
	"sync"
	"time"
)

// WorkerSlot represents a per-worker mailbox with sync.Cond blocking semantics.
//
// Architecture (ADR-001):
//   - Single-slot buffer (frame *Frame)
//   - Overwrite policy (new frame replaces old)
//   - Blocking consume (sync.Cond.Wait)
//   - Drop tracking (consecutiveDrops, totalDrops)
//
// Thread-safety:
//   - All fields protected by mu
//   - publishToSlot: called by distributionLoop (or batch goroutines)
//   - readFunc: called by worker goroutine (single consumer)
//
// See: ADR-001 (sync.Cond for Mailbox Semantics), ARCHITECTURE.md (Algorithm 4)
type WorkerSlot struct {
	// --- Mailbox State ---

	mu    sync.Mutex // Protects all fields
	cond  *sync.Cond // Signals worker goroutine
	frame *Frame     // Single-slot buffer (nil = consumed, non-nil = unconsumed)

	// --- Operational Stats ---

	lastConsumedAt   time.Time // Timestamp of last successful consume (for idle detection)
	lastConsumedSeq  uint64    // Sequence number of last consumed frame
	consecutiveDrops uint64    // Current streak of unconsumed frames (resets on consume)
	totalDrops       uint64    // Lifetime count of dropped frames

	// --- Lifecycle ---

	closed bool // True after Unsubscribe (signals readFunc to return nil)
}

// publishToSlot publishes a frame to a worker slot (non-blocking).
//
// Algorithm (ADR-001):
//  1. Lock slot mutex
//  2. Check if slot closed (worker unsubscribed)
//  3. Check if previous frame unconsumed (increment drop counters)
//  4. Overwrite slot.frame (JIT semantics)
//  5. Signal slot.cond (wake worker if blocked)
//  6. Unlock mutex
//
// Called by: distributeToWorkers (sequentially or in batch goroutines)
//
// Thread-safety: Safe for concurrent calls (mutex-protected).
//
// Latency: O(1) - Lock + checks + assign + signal ≈ 1µs per slot
func (s *supplier) publishToSlot(slot *WorkerSlot, frame *Frame) {
	slot.mu.Lock()
	defer slot.mu.Unlock()

	// Check if worker unsubscribed (graceful skip)
	if slot.closed {
		return
	}

	// Check if previous frame unconsumed (worker slow)
	if slot.frame != nil {
		slot.consecutiveDrops++
		slot.totalDrops++
	}

	// Overwrite with new frame (JIT semantics)
	slot.frame = frame

	// Wake worker if blocked in Wait()
	slot.cond.Signal()
}

// Subscribe registers a worker and returns a blocking read function (implements Supplier.Subscribe).
//
// Returns: func() *Frame that blocks until frame available or shutdown.
//
// Semantics:
//   - Mailbox pattern: single-slot buffer, overwrite on publish
//   - Blocking consume: readFunc() blocks until frame available
//   - Graceful shutdown: returns nil when closed (Unsubscribe or Stop)
//   - Safe degradation: returns nil-readFunc if called during/after Stop()
//
// Thread-safety:
//   - Subscribe: Safe for concurrent calls (sync.Map.Store)
//   - readFunc: MUST be called from single worker goroutine only
//
// Contract:
//   - Worker MUST call Unsubscribe when done (defer pattern recommended)
//   - Worker MUST NOT call readFunc concurrently (single consumer only)
//
// See: ADR-001 (sync.Cond), ADR-005 (Graceful Shutdown), ARCHITECTURE.md (Algorithm 4)
func (s *supplier) Subscribe(workerID string) func() *Frame {
	// Check if supplier is stopping (fail-fast)
	if s.stopping.Load() {
		// Return nil-readFunc (immediate exit, no goroutine leak)
		return func() *Frame { return nil }
	}

	// Create new slot for this worker
	slot := &WorkerSlot{}
	slot.cond = sync.NewCond(&slot.mu)
	slot.lastConsumedAt = time.Now() // Initialize (avoid IsIdle on first Stats call)

	// Register slot in slots map
	s.slots.Store(workerID, slot)

	// Return blocking read function (closured slot reference)
	return func() *Frame {
		slot.mu.Lock()
		defer slot.mu.Unlock()

		// Wait until frame available or closed
		for slot.frame == nil && !slot.closed {
			slot.cond.Wait() // Blocks here, releases lock
		}

		// Check shutdown condition
		if slot.closed {
			return nil // Signal worker to exit
		}

		// Consume frame
		frame := slot.frame
		slot.frame = nil // Mark as consumed
		slot.lastConsumedAt = time.Now()
		slot.lastConsumedSeq = frame.Seq
		slot.consecutiveDrops = 0 // Reset streak (worker alive and responsive)

		return frame
	}
}

// Unsubscribe removes a worker and signals its readFunc to return nil (implements Supplier.Unsubscribe).
//
// Behavior:
//  1. Load slot from slots map (no-op if not found)
//  2. Lock slot, set closed=true
//  3. Signal slot.cond (wake worker if blocked)
//  4. Delete slot from map
//
// After Unsubscribe:
//   - publishToSlot becomes no-op for this slot (closed check)
//   - readFunc returns nil (worker detects shutdown)
//   - Stats() excludes this worker
//
// Idempotent: Safe to call multiple times (subsequent calls no-op).
//
// Thread-safety: Safe for concurrent calls.
func (s *supplier) Unsubscribe(workerID string) {
	// Load slot (no-op if not found)
	val, ok := s.slots.Load(workerID)
	if !ok {
		return // Already unsubscribed (idempotent)
	}

	slot := val.(*WorkerSlot)

	// Mark closed and wake worker
	slot.mu.Lock()
	slot.closed = true
	slot.cond.Signal() // Wake readFunc if blocked
	slot.mu.Unlock()

	// Remove from map
	s.slots.Delete(workerID)
}
