// Package framesupplier implements just-in-time frame distribution
// with symmetric mailbox architecture.
//
// Philosophy: "Drop frames, never queue. Latency > Completeness."
//
// Design:
//   - Non-blocking Publish() (~1µs latency)
//   - Blocking Subscribe() with mailbox semantics (efficient waiting)
//   - Zero-copy frame sharing (immutability contract)
//   - Symmetric JIT architecture at all levels
//
// See: docs/ARCHITECTURE.md, docs/C4_MODEL.md, docs/ADR/
package framesupplier

import (
	"context"

	"github.com/e7canasta/orion-care-sensor/modules/framesupplier/internal"
)

// Frame is re-exported from internal package to avoid import cycles.
// See internal/frame.go for full documentation.
type Frame = internal.Frame

// Supplier is the public interface for frame distribution.
//
// Design:
//   - Interface (not concrete type) for future extensibility
//   - Lifecycle: New() → Start() → Publish()/Subscribe() → Stop()
//   - Thread-safe: all methods safe for concurrent use
//
// Implementation is in internal/supplier.go (hidden from clients).
type Supplier interface {
	// Start begins the distribution loop (blocks until ctx.Done or Stop).
	// Must be called before Publish() or Subscribe().
	//
	// Goroutine management:
	//   - Spawns 1 goroutine (distributionLoop)
	//   - Caller typically runs Start() in separate goroutine
	//
	// Returns: error if already started or internal failure.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the distribution loop.
	// Blocks until distributionLoop exits.
	//
	// After Stop():
	//   - Publish() becomes no-op (safe, but frames dropped)
	//   - Subscribe() readFunc returns nil (workers detect shutdown)
	//
	// Idempotent: safe to call multiple times.
	Stop() error

	// Publish sends a frame to the distribution loop (non-blocking).
	//
	// Semantics:
	//   - Non-blocking: always returns immediately (~1µs)
	//   - Overwrite policy: new frame replaces old unconsumed frame (JIT)
	//   - Drop tracking: increments InboxDrops if previous frame unconsumed
	//
	// Thread-safety: safe for concurrent calls (typically 1 publisher)
	//
	// Contract:
	//   - frame MUST NOT be nil (caller responsibility)
	//   - frame.Data MUST NOT be modified after Publish (immutability contract)
	//
	// See: ADR-001 (sync.Cond), ADR-004 (Symmetric JIT)
	Publish(frame *Frame)

	// Subscribe registers a worker and returns a blocking read function.
	//
	// Returned function:
	//   - Blocks until frame available (efficient, no busy-wait)
	//   - Returns nil on graceful shutdown (Unsubscribe or Stop)
	//   - Thread-safety: safe to call from single worker goroutine
	//
	// Semantics:
	//   - Mailbox pattern: single-slot buffer, overwrite on publish
	//   - Drop tracking: increments worker's TotalDrops on overwrite
	//   - Worker must call Unsubscribe when done (defer pattern recommended)
	//
	// Example:
	//   readFunc := supplier.Subscribe("PersonDetector")
	//   defer supplier.Unsubscribe("PersonDetector")
	//   for {
	//       frame := readFunc()  // Blocks here
	//       if frame == nil { break }
	//       process(frame)
	//   }
	//
	// See: ADR-001 (sync.Cond), ARCHITECTURE.md (Worker Slot Mailbox)
	Subscribe(workerID string) func() *Frame

	// Unsubscribe removes a worker and signals its readFunc to return nil.
	//
	// Behavior:
	//   - Safe to call even if workerID not subscribed (idempotent)
	//   - Wakes worker if blocked in readFunc (returns nil)
	//   - After Unsubscribe, workerID's stats removed from Stats()
	//
	// Thread-safety: safe for concurrent calls.
	Unsubscribe(workerID string)

	// Stats returns operational statistics (non-blocking snapshot).
	//
	// Use cases:
	//   - Monitor inbox drops (should be ~0 in healthy system)
	//   - Detect idle workers (health checks, restart policies)
	//   - SLA compliance (drop rate thresholds)
	//
	// Thread-safety: safe for concurrent calls, returns snapshot (not live view).
	//
	// See: ARCHITECTURE.md (Operational Monitoring)
	Stats() SupplierStats
}

// SupplierStats is re-exported from internal package to avoid import cycles.
// See internal/types.go for full documentation.
type SupplierStats = internal.SupplierStats

// WorkerStats is re-exported from internal package to avoid import cycles.
// See internal/types.go for full documentation.
type WorkerStats = internal.WorkerStats

// New creates a new Supplier instance with default configuration.
//
// Lifecycle:
//  1. supplier := framesupplier.New()
//  2. go supplier.Start(ctx)  // Start distribution loop
//  3. supplier.Publish(frame)  // Publisher side
//     worker := supplier.Subscribe("id")  // Consumer side
//  4. supplier.Stop()  // Graceful shutdown
//
// Returns: Supplier interface (implementation is internal).
func New() Supplier {
	return internal.NewSupplier()
}
