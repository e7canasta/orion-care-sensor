# ADR-005: Graceful Shutdown Semantics

**Status**: Accepted
**Date**: 2025-01-05
**Authors**: Ernesto + Gaby
**Trigger**: TestGracefulShutdown detected bug in Stop() implementation

---

## Changelog

| Version | Date       | Author          | Changes                           |
|---------|------------|-----------------|-----------------------------------|
| 0.1     | 2025-01-05 | Ernesto + Gaby  | Initial proposal (bug discovered) |
| 1.0     | 2025-01-05 | Ernesto + Gaby  | Decision: Option A accepted       |

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

## Decision

**We accept Option A: Stop() Closes All Slots**

### Rationale

#### 1. Architectural Symmetry (ADR-004 Alignment)

**ADR-004 established symmetric JIT architecture**:
```
Inbox Lifecycle:
  Create: Start() spawns distributionLoop
  Destroy: Stop() → ctx.Done → inboxCond.Broadcast → distributionLoop exits ✅

Worker Slots Lifecycle:
  Create: Subscribe() creates slot
  Destroy: ??? (was undefined) ❌
```

**Decision restores symmetry**:
- If FrameSupplier creates something (inbox, slots), FrameSupplier destroys it
- "Casa de herrero, cuchillo de acero" applies to lifecycle management, not just JIT semantics

#### 2. Bounded Context Clarification

**Question**: Does closing slots expand FrameSupplier's bounded context?

**Analysis**:
- ✅ **Slot lifecycle** = create/destroy slot data structures → Already FrameSupplier's responsibility (Subscribe creates slots)
- ❌ **Worker lifecycle** = restart policies, health monitoring, SLA enforcement → Different module (worker-lifecycle)

**Conclusion**: Slot lifecycle ≠ Worker lifecycle. Completing the create→destroy cycle is consistency, not scope creep.

---

#### 2.1 Worker Agency (Post-Shutdown Behavior)

**Critical distinction**: FrameSupplier **notifies** shutdown, does NOT **control** worker behavior post-shutdown.

**When readFunc returns nil**:
```go
// Worker has AGENCY here (not controlled by FrameSupplier)
frame := readFunc()
if frame == nil {
    // Option 1: Report to orchestrator, wait for instructions
    // Option 2: Exit immediately (fast fail)
    // Option 3: Retry Subscribe (reconnect attempt)

    // Worker + Worker-Orchestrator decide (not FrameSupplier)
}
```

**System Architecture** (peer bounded contexts, not hierarchical):
```
Stream-Capture ←→ FrameSupplier ←→ Workers ←→ Worker-Orchestrator

Each with independent responsibilities:
- FrameSupplier: Distribution (JIT, mailbox, drop policy)
- Workers: Inference execution
- Worker-Orchestrator: Lifecycle (restart, SLA, health monitoring)
```

**FrameSupplier responsibilities** (✅ what we DO):
- Distribute frames via JIT semantics
- Notify shutdown (close slots → readFunc returns nil)
- Track operational metrics (drops, idle detection)

**FrameSupplier NON-responsibilities** (❌ what we DON'T do):
- Decide if worker should retry (orchestrator's responsibility)
- Guarantee worker resiliency (worker + orchestrator)
- Monitor worker health post-shutdown (orchestrator)

**Implication for Stop()**:
- Closing slots = fulfilling distribution contract ("no more frames")
- NOT controlling worker fate (worker decides: exit, retry, report to orchestrator)
- Resiliency handled by Worker-Orchestrator module (future ADR)

**This validates Option A**: We close slots to complete OUR bounded context (distribution lifecycle), not to manage THEIR bounded context (worker lifecycle).

**Future Work**: Worker-Orchestrator module will handle events like:
```go
orchestrator.OnWorkerEvent(WorkerDisconnected{
    workerID: "person-detector",
    reason: "supplier_stopped",  // Event from FrameSupplier
})

// Orchestrator decides response based on SLA:
// - Critical worker? Escalate to Orion core (restart FrameSupplier)
// - BestEffort worker? Accept degradation, continue
// - Transient issue? Wait and retry Subscribe
```

#### 3. Contract Fulfillment

**Public API contract** (framesupplier.go:97):
```go
// After Stop():
//   - Subscribe() readFunc returns nil (workers detect shutdown)
```

**Only Option A guarantees this**:
- Option B (ctx.Done): Workers blocked IN `cond.Wait()` cannot check ctx between calls
- Option C (hybrid): Solves problem but adds breaking change + complexity
- Option D (timeout): Violates <2s latency requirement

#### 4. Simplicity at Macro Level

**KISS applies at API level**:
- ✅ No breaking changes (Subscribe signature unchanged)
- ✅ Workers use existing pattern: `if frame == nil { break }`
- ✅ No timeout logic, no dual exit paths

**Complexity localized inside Stop()** (50 LOC to close slots) is acceptable.

---

## Implementation

### Core Changes to Stop()

```go
type Supplier struct {
    // Existing fields...
    stopping atomic.Bool  // NEW: Prevent Subscribe during Stop
}

func (s *Supplier) Stop() error {
    // 1. Set stopping flag (blocks new subscriptions)
    s.stopping.Store(true)

    // 2. Cancel context (signals distributionLoop)
    s.cancel()

    // 3. Wake distributionLoop
    s.inboxCond.Broadcast()

    // 4. Close all worker slots (NEW)
    s.slots.Range(func(key, value interface{}) bool {
        slot := value.(*WorkerSlot)
        slot.mu.Lock()
        slot.closed = true
        slot.cond.Broadcast()  // Wake blocked workers
        slot.mu.Unlock()
        return true
    })

    // 5. Wait for distributionLoop to exit
    s.wg.Wait()

    return nil
}
```

### Race Condition: Subscribe During Stop

**Problem**: Subscribe() called concurrently with Stop() creates slot that never closes.

**Solution**: Safe degradation via stopping flag

```go
func (s *Supplier) Subscribe(workerID string) func() *Frame {
    // Check if supplier is stopping
    if s.stopping.Load() {
        // Return readFunc that always returns nil (safe degradation)
        return func() *Frame { return nil }
    }

    // Normal slot creation...
    slot := &WorkerSlot{cond: sync.NewCond(&sync.Mutex{})}
    s.slots.Store(workerID, slot)

    return func() *Frame {
        slot.mu.Lock()
        defer slot.mu.Unlock()

        for slot.frame == nil && !slot.closed {
            slot.cond.Wait()
        }

        if slot.closed {
            return nil  // Stop() or Unsubscribe() called
        }

        frame := slot.frame
        slot.frame = nil
        return frame
    }
}
```

**Rationale**: Safe degradation > panic. Worker subscribing after Stop() receives no-op readFunc (immediately returns nil).

---

### Idempotency: Unsubscribe After Stop

**Scenario**:
```go
readFunc := supplier.Subscribe(workerID)
defer supplier.Unsubscribe(workerID)  // Always deferred

for {
    frame := readFunc()
    if frame == nil { break }  // Stop() closed slot
    process(frame)
}
// defer runs: Unsubscribe on already-closed slot
```

**Current Unsubscribe (already idempotent)**:
```go
func (s *Supplier) Unsubscribe(workerID string) {
    val, ok := s.slots.Load(workerID)
    if !ok { return }  // Idempotent: already unsubscribed

    slot := val.(*WorkerSlot)
    slot.mu.Lock()
    slot.closed = true      // Idempotent: setting true → true (no-op)
    slot.cond.Broadcast()   // Idempotent: multiple broadcasts safe
    slot.mu.Unlock()

    s.slots.Delete(workerID)
}
```

**Conclusion**: No changes needed. Unsubscribe is already safe to call on Stop()-closed slots.

---

## Consequences

### Positive ✅

1. **Contract Compliance**: `readFunc()` returns nil after Stop() (workers exit cleanly)
2. **Architectural Symmetry**: Inbox and slots both have complete create→destroy lifecycle
3. **No Breaking Changes**: Subscribe API unchanged (backward compatible)
4. **Fail-Fast**: Workers exit immediately (<100ms) when Stop() called
5. **Thread-Safe**: atomic.Bool + Range + per-slot mutex prevents races

### Negative ❌

1. **Coupling**: Stop() now knows about worker slots (but slots already managed by Supplier)
2. **Complexity**: +15 LOC in Stop() (Range + close logic)
3. **Testing**: Need tests for Subscribe-during-Stop race condition

### Mitigation

**Complexity**: Localized inside Stop() (API remains simple)
**Testing**: Add TestSubscribeDuringStop, TestUnsubscribeAfterStop

---

## Alternatives Rejected

### Option B: Workers Handle ctx.Done()

**Proposal**: Document pattern where workers check ctx.Done() before readFunc()

**Why Rejected**:
- ❌ Workers blocked IN `cond.Wait()` cannot check ctx
- ❌ Contract violation: readFunc() doesn't return nil (returns NEVER)
- ❌ Requires caller discipline (easy to forget ctx.Done check)

---

### Option C: Hybrid (Stop + ctx-aware readFunc)

**Proposal**: Stop() closes slots + Subscribe(ctx, workerID) signature change

**Why Rejected**:
- ❌ Breaking change (all examples, clients need update)
- ❌ Complexity: readFunc checks TWO conditions (slot.closed AND ctx.Err())
- ❌ No clear benefit over Option A (which already solves problem)

---

### Option D: Stop() Timeout with Force-Close

**Proposal**: Wait 5s for workers to Unsubscribe, then force-close

**Why Rejected**:
- ❌ Violates Orion philosophy: <2s latency requirement (5s timeout too slow)
- ❌ Hides bugs: Workers not calling Unsubscribe pass silently (not fail-fast)
- ❌ Complexity: timeout + polling loop

---

## Testing Strategy

### New Tests Required

1. **TestGracefulShutdown** (update existing):
   - Subscribe worker (blocks in readFunc, no frames)
   - Call Stop()
   - Assert: readFunc returns nil within 100ms
   - Assert: Worker goroutine exits cleanly

2. **TestSubscribeDuringStop** (new):
   - Start Stop() in goroutine
   - Subscribe() concurrently (after stopping=true)
   - Assert: readFunc immediately returns nil (safe degradation)
   - Assert: No goroutine leak

3. **TestUnsubscribeAfterStop** (new):
   - Subscribe worker
   - Call Stop() (closes slot)
   - Call Unsubscribe() (idempotent)
   - Assert: No panic, no double-close issues

4. **TestMultipleStopCalls** (idempotency):
   - Call Stop() twice
   - Assert: Second call is no-op
   - Assert: No panic

---

## Implementation Checklist (Next Coding Session)

- [ ] Add `stopping atomic.Bool` to Supplier struct
- [ ] Update `Stop()`: Set stopping flag, Range over slots, close + broadcast
- [ ] Update `Subscribe()`: Check stopping flag, return nil-readFunc if stopping
- [ ] Update TestGracefulShutdown (ensure it passes)
- [ ] Add TestSubscribeDuringStop
- [ ] Add TestUnsubscribeAfterStop
- [ ] Add TestMultipleStopCalls
- [ ] Update ARCHITECTURE.md (Stop() lifecycle section)

---

## Emergent Insights (Post-Decision)

### Worker Agency Pattern

**Origin**: Café retrospective after accepting Option A

**Insight**: FrameSupplier notifies (via nil readFunc), workers + orchestrator control post-shutdown behavior.

**Portability**:
- Applies to all Orion 2.0 peer modules (no hierarchical control)
- Pattern: "Notification vs Control" - bounded contexts notify each other, don't control
- Future: Worker-Orchestrator design benefits from this clarity

**Why gold**: Prevents scope creep in future (FrameSupplier won't accumulate worker management responsibilities).

**Named Pattern**: **"Notification Contract in Peer Architecture"**
- Module A notifies Module B via contract (readFunc → nil)
- Module B has agency over response (exit, retry, escalate)
- Module C (orchestrator) handles resiliency policies

---

## Related Decisions

- **ADR-001**: sync.Cond for Mailbox Semantics (slot.cond.Broadcast used here)
- **ADR-004**: Symmetric JIT Architecture (this decision restores symmetry)
- **Future ADR** (Worker-Orchestrator): Will leverage "supplier_stopped" events

---

## References

- Test code: `framesupplier_test.go:308-383` (TestGracefulShutdown)
- Implementation: `supplier.go:103` (Stop method)
- Worker pattern: `examples/worker_client.go:18-49`
- Contract: `framesupplier.go:44-52` (Stop documentation)

---

**Review Status**: ✅ Accepted (Ready for Implementation)
