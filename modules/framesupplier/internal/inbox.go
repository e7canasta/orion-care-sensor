package internal

import (
	"sync/atomic"
)

// Publish sends a frame to the distribution loop (implements Supplier.Publish).
//
// Algorithm (ADR-001, ADR-004):
//  1. Lock inbox mutex
//  2. Check if previous frame unconsumed (increment inboxDrops if so)
//  3. Overwrite inboxFrame (JIT semantics: new replaces old)
//  4. Signal inboxCond (wake distributionLoop if blocked)
//  5. Unlock mutex
//
// Semantics:
//   - Non-blocking: Always returns immediately (~1µs)
//   - Overwrite policy: New frame replaces old (JIT principle)
//   - Drop tracking: Increments inboxDrops if overwriting unconsumed frame
//
// Thread-safety:
//   - Safe for concurrent calls (mutex-protected)
//   - Typically 1 publisher (stream-capture), but supports multiple
//
// Contract:
//   - frame MUST NOT be nil (caller responsibility, no validation)
//   - frame.Data MUST NOT be modified after Publish (immutability contract)
//
// Latency: O(1) - Lock + pointer check + assign + signal ≈ 1µs
//
// See: ADR-001 (sync.Cond), ADR-004 (Symmetric JIT), ARCHITECTURE.md (Algorithm 1)
func (s *supplier) Publish(frame *Frame) {
	s.inboxMu.Lock()

	// Check if previous frame unconsumed (distribution loop slow)
	if s.inboxFrame != nil {
		// Increment drop counter atomically (Stats reads this without lock)
		atomic.AddUint64(&s.inboxDrops, 1)
	}

	// Overwrite with new frame (JIT semantics)
	s.inboxFrame = frame

	// Wake distributionLoop if blocked in Wait()
	s.inboxCond.Signal()

	s.inboxMu.Unlock()
}
