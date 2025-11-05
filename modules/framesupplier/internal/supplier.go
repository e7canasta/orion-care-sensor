// Package internal implements FrameSupplier with symmetric JIT architecture.
//
// This package is INTERNAL - clients MUST use public API in parent package.
// Reason: Allows internal refactoring without breaking changes.
package internal

import (
	"context"
	"fmt"
	"sync"
)

// supplier is the concrete implementation of framesupplier.Supplier interface.
//
// Goroutine topology:
//   - 1 fixed: distributionLoop (spawned by Start, stopped by Stop)
//   - 0-N/8 transient: batch goroutines (spawned by distributeToWorkers if >8 workers)
//   - N external: worker goroutines (NOT managed by supplier, workers own them)
//
// Thread-safety: All public methods safe for concurrent use.
type supplier struct {
	// --- Inbox Mailbox (ADR-001, ADR-004) ---
	// Publisher → Supplier communication

	inboxMu    sync.Mutex // Protects inboxFrame
	inboxCond  *sync.Cond // Signals distributionLoop
	inboxFrame *Frame     // Single-slot buffer (nil = consumed, non-nil = unconsumed)
	inboxDrops uint64     // Atomic counter (incremented when overwriting unconsumed frame)

	// --- Worker Slots (ADR-001) ---
	// Supplier → Workers communication

	slots sync.Map // Concurrent map: workerID (string) → *WorkerSlot

	// --- Distribution State ---

	publishSeq uint64 // Atomic counter: global frame sequence (assigned during distribution)

	// --- Lifecycle ---

	ctx    context.Context    // Lifecycle context (cancelled on Stop)
	cancel context.CancelFunc // Cancel function for ctx
	wg     sync.WaitGroup     // Tracks distributionLoop goroutine

	startedMu sync.Mutex // Protects started flag
	started   bool       // True after Start() called (idempotency guard)
}

// NewSupplier creates a new supplier instance (called by public New() in parent package).
// Exported to allow parent package to construct, but returns unexported *supplier type.
func NewSupplier() *supplier {
	s := &supplier{}
	s.inboxCond = sync.NewCond(&s.inboxMu)
	return s
}

// Start begins the distribution loop (implements Supplier.Start).
//
// Lifecycle:
//  1. Validates not already started (idempotency)
//  2. Sets ctx from caller
//  3. Spawns distributionLoop goroutine
//  4. Returns immediately (non-blocking)
//
// The distributionLoop runs until:
//   - ctx.Done() (caller cancellation)
//   - Stop() called (internal cancellation)
//
// Thread-safety: Safe for concurrent calls (only first call succeeds).
func (s *supplier) Start(ctx context.Context) error {
	s.startedMu.Lock()
	defer s.startedMu.Unlock()

	if s.started {
		return fmt.Errorf("supplier already started")
	}

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.started = true

	// Spawn distribution goroutine
	s.wg.Add(1)
	go s.distributionLoop()

	return nil
}

// Stop gracefully shuts down the distribution loop (implements Supplier.Stop).
//
// Behavior:
//  1. Cancels ctx (signals distributionLoop to exit)
//  2. Signals inboxCond (wakes distributionLoop if blocked)
//  3. Waits for distributionLoop to exit (wg.Wait)
//  4. Returns when shutdown complete
//
// After Stop():
//   - Publish() becomes no-op (frames silently dropped)
//   - Subscribe() readFunc returns nil (workers detect shutdown)
//
// Idempotent: Safe to call multiple times (subsequent calls no-op).
//
// Thread-safety: Safe for concurrent calls.
func (s *supplier) Stop() error {
	s.startedMu.Lock()
	if !s.started {
		s.startedMu.Unlock()
		return nil // Already stopped (idempotent)
	}
	s.startedMu.Unlock()

	// Signal shutdown
	s.cancel()

	// Wake distributionLoop if blocked in inboxCond.Wait
	s.inboxCond.Broadcast()

	// Wait for distributionLoop to exit
	s.wg.Wait()

	return nil
}

// distributionLoop is the core goroutine that consumes inbox and distributes to workers.
//
// Algorithm (ADR-001, ADR-004):
//  1. Wait for frame in inbox (sync.Cond.Wait - efficient blocking)
//  2. Check ctx.Done (graceful shutdown)
//  3. Consume frame from inbox (mark as nil)
//  4. Distribute to all worker slots (with batching if >8 workers)
//  5. Repeat
//
// Exits on: ctx.Done() or Stop() called.
//
// See: ADR-004 (Symmetric JIT Architecture), ARCHITECTURE.md (Algorithm 2)
func (s *supplier) distributionLoop() {
	defer s.wg.Done()

	for {
		// Wait for frame or shutdown
		s.inboxMu.Lock()

		for s.inboxFrame == nil {
			// Check shutdown before blocking
			if s.ctx.Err() != nil {
				s.inboxMu.Unlock()
				return
			}

			// Block until frame available or signal
			s.inboxCond.Wait()

			// Re-check shutdown after wake
			if s.ctx.Err() != nil {
				s.inboxMu.Unlock()
				return
			}
		}

		// Consume frame
		frame := s.inboxFrame
		s.inboxFrame = nil // Mark as consumed
		s.inboxMu.Unlock()

		// Distribute to workers (implemented in distribution.go)
		s.distributeToWorkers(frame)
	}
}
