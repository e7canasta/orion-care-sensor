# ADR-005: Graceful Shutdown Semantics

**Status**: Proposed (Pending Discovery Session)
**Date**: 2025-01-05
**Authors**: Ernesto + Gaby
**Trigger**: TestGracefulShutdown detected bug in Stop() implementation

---

## Changelog

| Version | Date       | Author          | Changes                           |
|---------|------------|-----------------|-----------------------------------|
| 0.1     | 2025-01-05 | Ernesto + Gaby  | Initial proposal (bug discovered) |

---

## Context

### Bug Discovery

During coding session, `TestGracefulShutdown` detected that `Stop()` does **not** wake workers blocked in `readFunc()`.

**Test scenario**:
1. Start supplier
2. Subscribe worker (blocks in `readFunc()`, no frames published)
3. Call `Stop()`
4. **Expected**: Worker wakes, `readFunc()` returns `nil`
5. **Actual**: Worker remains blocked indefinitely

**Test result**:
```
framesupplier_test.go:363: Worker didn't exit after Stop()
```

---

### Current Implementation

**`supplier.go:103` (Stop method)**:
```go
func (s *supplier) Stop() error {
    s.cancel()               // Cancel ctx
    s.inboxCond.Broadcast()  // ✅ Wake distributionLoop
    s.wg.Wait()              // Wait for distributionLoop to exit
    return nil
}
```

**Problem**: Does NOT signal worker slots to wake.

**`worker_slot.go:106` (readFunc closure)**:
```go
return func() *Frame {
    slot.mu.Lock()
    defer slot.mu.Unlock()

    // Wait until frame available or closed
    for slot.frame == nil && !slot.closed {
        slot.cond.Wait()  // ❌ Blocked here if no frames and Stop() called
    }

    if slot.closed {
        return nil  // Never reached if cond.Wait not signaled
    }
    // ...
}
```

**Gap**: `Stop()` sets `s.cancel()` but never sets `slot.closed = true` or calls `slot.cond.Broadcast()`.

---

### Contract Expectation

**Public API contract** (`framesupplier.go:97`):
```go
// After Stop():
//   - Publish() becomes no-op (safe, but frames dropped)
//   - Subscribe() readFunc returns nil (workers detect shutdown)
```

**Worker pattern** (`examples/worker_client.go:21`):
```go
readFunc := supplier.Subscribe(workerID)
defer supplier.Unsubscribe(workerID)

for {
    frame := readFunc()  // Blocks here
    if frame == nil { break }  // Only exits if nil received
    process(frame)
}
```

**Expectation**: Workers exit gracefully when `Stop()` called, without requiring manual `Unsubscribe()`.

---

## Problem Statement

**Question**: How should `Stop()` ensure workers wake and exit cleanly?

### Current Behavior

| Action              | Inbox (distributionLoop) | Worker Slots (readFunc) |
|---------------------|--------------------------|-------------------------|
| `Stop()` called     | ✅ Wakes (inboxCond.Broadcast) | ❌ Remains blocked     |
| `Unsubscribe()` called | N/A                      | ✅ Wakes (slot.cond.Signal) |

**Result**: Workers require **explicit** `Unsubscribe()` to exit. `Stop()` alone is insufficient.

---

## Options to Explore (Discovery Session)

### Option A: Stop() Closes All Slots

**Proposal**: `Stop()` iterates all slots, sets `closed=true`, broadcasts `cond`.

```go
func (s *supplier) Stop() error {
    s.cancel()
    s.inboxCond.Broadcast()

    // NEW: Close all worker slots
    s.slots.Range(func(key, value interface{}) bool {
        slot := value.(*WorkerSlot)
        slot.mu.Lock()
        slot.closed = true
        slot.cond.Broadcast()  // Wake worker
        slot.mu.Unlock()
        return true
    })

    s.wg.Wait()
    return nil
}
```

**Pros**:
- ✅ Workers wake automatically (no manual Unsubscribe needed)
- ✅ Matches contract expectation ("readFunc returns nil")
- ✅ Symmetric with `Unsubscribe()` (both set closed + broadcast)

**Cons**:
- ❌ `Stop()` now couples to worker lifecycle (bounded context leak?)
- ❌ Race condition: New Subscribe() during Stop() (need `started` flag check?)
- ❌ Workers may still have pending `defer Unsubscribe()` (double-close safe?)

**Questions**:
- Should `Stop()` delete slots from map (like Unsubscribe)?
- Should `Subscribe()` check if supplier stopped (fail fast)?

---

### Option B: Workers Must Handle ctx.Done()

**Proposal**: Document that workers MUST check `ctx.Done()` before `readFunc()`.

```go
// Recommended pattern (add to examples/worker_client.go)
for {
    select {
    case <-ctx.Done():
        return  // Exit on context cancellation
    default:
    }

    frame := readFunc()  // Still blocks if ctx.Done() not checked
    if frame == nil { break }
    process(frame)
}
```

**Pros**:
- ✅ Explicit control to caller (workers decide when to exit)
- ✅ No changes to `Stop()` implementation
- ✅ Bounded context preserved (Stop only manages distributionLoop)

**Cons**:
- ❌ Workers still blocked in `readFunc()` (select only checks between calls)
- ❌ Contract violated ("readFunc returns nil" not guaranteed)
- ❌ Requires caller discipline (easy to forget ctx.Done check)

**Questions**:
- How do workers exit if already blocked in `readFunc()`?
- Should `readFunc()` internally check `ctx.Done()`? (but ctx not passed to Subscribe)

---

### Option C: Hybrid (Stop + ctx-aware readFunc)

**Proposal**:
1. `Stop()` closes slots (Option A)
2. `Subscribe()` takes `ctx` parameter (new signature)
3. `readFunc()` checks `ctx.Done()` internally

```go
// New signature
func (s *supplier) Subscribe(ctx context.Context, workerID string) func() *Frame {
    slot := &WorkerSlot{}
    slot.cond = sync.NewCond(&slot.mu)
    s.slots.Store(workerID, slot)

    return func() *Frame {
        slot.mu.Lock()
        defer slot.mu.Unlock()

        for slot.frame == nil && !slot.closed {
            // Check ctx before blocking
            if ctx.Err() != nil {
                return nil  // Context cancelled
            }
            slot.cond.Wait()
        }

        if slot.closed || ctx.Err() != nil {
            return nil
        }
        // ... consume frame
    }
}
```

**Pros**:
- ✅ Workers wake on both `Stop()` and `ctx.Done()`
- ✅ Explicit lifecycle management (ctx passed to Subscribe)
- ✅ Fail-fast on context cancellation

**Cons**:
- ❌ **Breaking change** (Subscribe signature changes)
- ❌ Complexity (dual exit paths: slot.closed vs ctx.Done)
- ❌ `ctx.Done()` check inside tight loop (performance?)

**Questions**:
- Is breaking change acceptable? (v1.0 not released yet)
- Should we check `ctx.Done()` before every `cond.Wait()`?
- Does this over-engineer the problem?

---

### Option D: Stop() Timeout with Force-Close

**Proposal**: `Stop()` waits N seconds for workers to Unsubscribe, then force-closes.

```go
func (s *supplier) Stop() error {
    s.cancel()
    s.inboxCond.Broadcast()
    s.wg.Wait()

    // Wait 5s for workers to Unsubscribe gracefully
    deadline := time.Now().Add(5 * time.Second)
    for {
        if s.slots.Len() == 0 {
            return nil  // All workers unsubscribed
        }
        if time.Now().After(deadline) {
            break  // Timeout
        }
        time.Sleep(100 * time.Millisecond)
    }

    // Force-close remaining workers
    s.slots.Range(func(key, value interface{}) bool {
        slot := value.(*WorkerSlot)
        slot.mu.Lock()
        slot.closed = true
        slot.cond.Broadcast()
        slot.mu.Unlock()
        return true
    })

    return nil
}
```

**Pros**:
- ✅ Graceful by default (workers have time to Unsubscribe)
- ✅ Force-close fallback (guarantees eventual exit)
- ✅ Observable (log warning if force-close triggered)

**Cons**:
- ❌ Adds latency (5s wait)
- ❌ Complexity (timeout + polling)
- ❌ Hides worker bugs (should workers hang, Stop succeeds anyway)

---

## Open Questions (For Discovery Session)

### Architectural Questions

1. **Bounded Context**: Is worker lifecycle part of FrameSupplier's responsibility?
   - Current: FrameSupplier only distributes frames
   - Proposed: FrameSupplier also manages worker exit semantics

2. **Coupling**: Should `Stop()` know about workers, or only distributionLoop?
   - Philosophy: "Inbox and slots are symmetric JIT mailboxes"
   - But: Inbox wake is internal (distributionLoop), slot wake is external (workers)

3. **Contract Clarity**: What does "graceful shutdown" mean?
   - A: Stop() blocks until all workers Unsubscribe (caller ensures)
   - B: Stop() forces workers to exit (supplier ensures)
   - C: Stop() + ctx.Done (hybrid, both participate)

### Implementation Questions

4. **Race Conditions**: What if `Subscribe()` called during `Stop()`?
   - Should Subscribe() check `s.started` flag?
   - Should Subscribe() fail if supplier stopping?

5. **Idempotency**: If worker calls `Unsubscribe()` after `Stop()` closed slot?
   - Current Unsubscribe: Idempotent (safe)
   - With Stop() closing: Still idempotent? (need test)

6. **Breaking Changes**: Is Subscribe(ctx, workerID) acceptable?
   - We're pre-v1.0 (backward compat not critical)
   - But: All examples/clients need update

### Testing Questions

7. **Deterministic Testing**: How do we test without timing dependencies?
   - Current test: Time.Sleep + timeout (brittle)
   - Better: Controllable distributionLoop (inject pause?)

8. **Race Detector**: Will Option A introduce new races?
   - `Stop()` iterates slots while `Subscribe()`/`Unsubscribe()` mutate
   - sync.Map is safe, but `slot.mu` locking order?

---

## Context from Other Systems

### Orion v1.x Pattern

**Orion v1** (core/orion.go) uses **explicit Unsubscribe**:
```go
// In worker goroutine
defer worker.Stop()  // Calls Unsubscribe internally

for {
    frame := readFunc()
    if frame == nil { break }
    process(frame)
}
```

**Pattern**: Workers own their lifecycle, not supplier.

**Question**: Should FrameSupplier follow this pattern, or improve it?

---

### Actor Model (Erlang/Akka)

**Erlang**: Supervisor kills actors on shutdown (forced).

**Akka**: Graceful stop sends PoisonPill, waits timeout, then kills.

**Question**: Should we adopt timeout + force-close (Option D)?

---

### Go Patterns

**http.Server.Shutdown()**:
```go
func (s *Server) Shutdown(ctx context.Context) error {
    // 1. Stop accepting new connections
    // 2. Wait for active requests to complete (with ctx timeout)
    // 3. Force-close after timeout
}
```

**Pattern**: Graceful with timeout + force fallback.

**Question**: Should FrameSupplier.Stop() take `context.Context` parameter?

---

## Constraints

### Non-Negotiable

1. **Thread-safety**: Stop() must be safe concurrent with Subscribe/Unsubscribe/Publish
2. **No panics**: Stop() must never panic (even if workers misbehave)
3. **Idempotency**: Multiple Stop() calls must be safe

### Desired

4. **Simplicity**: Prefer simple solution (KISS at macro level)
5. **Backward compat**: Minimize breaking changes (but not critical pre-v1.0)
6. **Fail-fast**: Workers should exit quickly (<100ms) on Stop()

---

## Success Criteria (Post-Discovery)

After discovery session, we should be able to:

1. ✅ Choose one option (A/B/C/D) with clear rationale
2. ✅ Document decision in this ADR (update to "Accepted")
3. ✅ Update implementation (coding session)
4. ✅ Update TestGracefulShutdown to pass
5. ✅ Add new tests for edge cases (race conditions, idempotency)

---

## Related Decisions

- **ADR-001**: sync.Cond for Mailbox Semantics (affects wake mechanism)
- **ADR-004**: Symmetric JIT Architecture (inbox vs slots symmetry)
- **Future ADR-006?**: Worker Lifecycle Management (if coupling increases)

---

## References

- Test code: `framesupplier_test.go:308-383` (TestGracefulShutdown)
- Implementation: `supplier.go:103` (Stop method)
- Worker pattern: `examples/worker_client.go:18-49`
- Contract: `framesupplier.go:44-52` (Stop documentation)

---

## Notes for Discovery Session

**Entry point (Point Silla)**:
> "TestGracefulShutdown found that Stop() doesn't wake workers.
> ¿Stop() debe cerrar slots (Option A), o workers manejan ctx (Option B), o hybrid?
> Pensaba en Option A (más simple)... ¿qué te parece?"

**Key tradeoffs to explore**:
- Bounded context (distribution only vs distribution + lifecycle)
- Coupling (Stop knows about workers vs workers autonomous)
- Breaking changes (Subscribe signature vs backward compat)
- Complexity (simple force-close vs timeout + fallback)

**Expected output**:
- Chosen option with rationale
- Edge cases identified
- Implementation plan
- Test strategy

---

**Status**: Pending discovery session (scheduled for next pairing)
