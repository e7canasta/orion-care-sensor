# ADR-004: Symmetric JIT Architecture

**Status**: Accepted
**Date**: 2025-01-05
**Authors**: Ernesto + Gaby

---

## Changelog

| Version | Date       | Author          | Changes                           |
|---------|------------|-----------------|-----------------------------------|
| 1.0     | 2025-01-05 | Ernesto + Gaby  | Initial decision - JIT symmetry   |

---

## Context

Orion's philosophy: **"Drop frames, never queue. Latency > Completeness."**

FrameSupplier sits in a multi-stage pipeline:

```
GStreamer (30fps) → stream-capture → FrameSupplier → Workers (1-30fps)
```

**Question**: Should FrameSupplier itself have **buffering** (queue) on its input side, or should it follow the same JIT principles it enforces for workers?

### The Problem: Casa de Herrero, Cuchillo de Palo

**Initial design** (inconsistent):
```go
// stream-capture publishes to buffered channel
gstreamCh := make(chan *Frame, 10)  // 10-frame buffer

// FrameSupplier consumes from channel
for frame := range gstreamCh {
    supplier.Publish(frame)  // But this frame may be 10s old @ 1fps!
}
```

**Problem**: FrameSupplier enforces "latest frame only" for **workers**, but accepts **stale frames** from **publisher**.

**"Casa de herrero, cuchillo de palo"** - We preach JIT but accept stale inventory.

---

## Decision

**FrameSupplier will implement symmetric JIT architecture: mailbox + overwrite at ALL levels.**

### Architecture Layers

```
Layer 1: stream-capture → FrameSupplier Inbox
         Pattern: Non-blocking publish, mailbox overwrite

Layer 2: FrameSupplier Inbox → Distribution Loop
         Pattern: Blocking consume, mailbox semantics

Layer 3: FrameSupplier → Worker Slots
         Pattern: Non-blocking publish, mailbox overwrite

Layer 4: Worker Slots → Worker Goroutines
         Pattern: Blocking consume, mailbox semantics
```

**Each layer uses same primitives**: sync.Cond, single-slot mailbox, overwrite policy.

---

## Implementation

### Inbox Mailbox (Layer 1)

```go
type Supplier struct {
    // Inbox for stream-capture
    inboxMu    sync.Mutex
    inboxCond  *sync.Cond
    inboxFrame *Frame
    inboxDrops uint64  // Operational metric
}

// stream-capture calls this (non-blocking)
func (s *Supplier) Publish(frame *Frame) {
    s.inboxMu.Lock()

    if s.inboxFrame != nil {
        s.inboxDrops++  // Distribution loop hasn't consumed previous frame
    }

    s.inboxFrame = frame  // Overwrite (JIT semantics)
    s.inboxCond.Signal()

    s.inboxMu.Unlock()
}
```

**Symmetry**: Same pattern as worker slots (Layer 3).

---

### Distribution Loop (Layer 2)

```go
func (s *Supplier) distributionLoop() {
    for {
        s.inboxMu.Lock()

        // Wait for frame or shutdown
        for s.inboxFrame == nil {
            if s.ctx.Err() != nil {
                s.inboxMu.Unlock()
                return
            }
            s.inboxCond.Wait()
        }

        frame := s.inboxFrame
        s.inboxFrame = nil  // Mark as consumed
        s.inboxMu.Unlock()

        // Fan-out to workers (may take 100µs)
        s.distributeToWorkers(frame)
    }
}
```

**Symmetry**: Same blocking pattern as worker consume (Layer 4).

---

### Worker Slots (Layer 3 + 4)

**Already designed** (from ADR-001):
```go
func (s *Supplier) publishToSlot(slot *WorkerSlot, frame *Frame) {
    slot.mu.Lock()
    if slot.frame != nil {
        slot.consecutiveDrops++
        slot.totalDrops++
    }
    slot.frame = frame  // Overwrite
    slot.cond.Signal()
    slot.mu.Unlock()
}

func (s *Supplier) Subscribe(workerID string) func() *Frame {
    // Returns blocking read function...
}
```

---

## Consequences

### Positive ✅

1. **Conceptual Simplicity**: Same pattern everywhere (learn once, apply everywhere)
2. **End-to-End JIT**: No hidden buffering anywhere in the chain
3. **Predictable Latency**: Worst-case frame age = max(layer latencies), not sum
4. **Operational Clarity**: Drops at each level are **visible** (inboxDrops, workerDrops)
5. **"Walk the Talk"**: Orion preaches JIT, Orion practices JIT

### Negative ❌

1. **No Buffering Abstraction**: Cannot integrate with buffered channels easily (need adapter)
2. **Lifecycle Management**: Supplier must manage distributionLoop goroutine (Start/Stop)
3. **Complexity**: Inbox adds ~50 LOC to Supplier (mailbox + goroutine)

### Mitigation

**Adapter Utility** (for buffered channel integration):
```go
// jit.go - Utility for JIT-compliant publishers

func ConsumeJIT(ch <-chan *Frame, handler func(*Frame)) {
    for {
        frame := <-ch  // Block for first frame

        // Drain channel to latest
        latest := frame
        drained := 0
        for {
            select {
            case frame = <-ch:
                latest = frame
                drained++
            default:
                goto publish
            }
        }

    publish:
        if drained > 0 {
            log.Debug("Drained stale frames", "count", drained)
        }
        handler(latest)  // Always publish freshest
    }
}
```

**Usage**:
```go
// stream-capture with buffered channel
gstreamCh := make(chan *Frame, 10)  // GStreamer needs decoupling buffer

// Integrate with FrameSupplier (JIT-compliant)
go framesupplier.ConsumeJIT(gstreamCh, func(frame *Frame) {
    supplier.Publish(frame)  // Always fresh
})
```

---

## Drop Semantics Analysis

### Scenario: 30fps source, 1fps workers, 100µs distribution

**Without Inbox** (direct channel):
```
t=0ms:    Frame 0 arrives → buffered channel (size 10)
t=33ms:   Frame 1 arrives → buffered channel
...
t=330ms:  Frame 10 arrives → buffered channel full
t=1000ms: Worker consumes Frame 0 (1 second old!)
```

**With Inbox (JIT)**:
```
t=0ms:    Frame 0 arrives → inbox overwrite
t=33ms:   Frame 1 arrives → inbox overwrite (Frame 0 dropped)
t=66ms:   Frame 2 arrives → inbox overwrite (Frame 1 dropped)
...
t=990ms:  Frame 29 arrives → inbox overwrite (Frame 28 dropped)
t=1000ms: Worker consumes Frame 29 (<10ms old)
```

**Inbox drops**: 29 (expected @ 30fps → 1fps mismatch)
**Worker drops**: 0 (workers consume as fast as distributed)

**Result**: Workers always see **fresh** frames (<100ms old), not 1-second-old frames.

---

## Latency Budget: End-to-End

### Pipeline Stages

```
Stage 1: GStreamer appsink → stream-capture
         Latency: ~1ms (GStreamer pipeline)

Stage 2: stream-capture → FrameSupplier Inbox
         Latency: ~1µs (Publish call)

Stage 3: Inbox → Distribution Loop
         Latency: <33ms @ 30fps (wait for distributionLoop cycle)

Stage 4: Distribution → Worker Slots
         Latency: ~100µs (distributeToWorkers, 64 workers)

Stage 5: Worker Slot → Worker Goroutine
         Latency: <1000ms @ 1fps (wait for worker cycle)

Stage 6: Worker → Python inference
         Latency: 20-50ms (YOLO inference)
```

**Total latency** (30fps source → 1fps inference):
- Best case: 1ms + 1µs + 0 + 100µs + 0 + 20ms ≈ **21ms**
- Worst case: 1ms + 1µs + 33ms + 100µs + 1000ms + 50ms ≈ **1084ms**

**With buffered channel** (10 frames @ 1fps):
- Best case: same
- Worst case: 21ms + 10 seconds (stale buffer) ≈ **10 seconds**

**JIT benefit**: 10× reduction in worst-case staleness.

---

## Symmetry Across Modules

**FrameSupplier** is part of Orion 2.0 modular architecture.

### Module Responsibilities

| Module              | JIT Pattern                           | Bounded Context        |
|---------------------|---------------------------------------|------------------------|
| **stream-capture**  | GStreamer appsink → internal mailbox  | Video acquisition      |
| **framesupplier**   | Inbox + worker slots                  | Frame distribution     |
| **worker-lifecycle**| Go→Python stdin (MsgPack mailbox)     | Worker management      |

**All use same primitives**: Mailbox + overwrite + sync.Cond.

**Benefit**: Developer moving between modules sees **familiar patterns**.

---

## Alternatives Considered

### Alternative A: Accept Buffered Input

```go
// FrameSupplier accepts buffered channel (no inbox)
func (s *Supplier) Start(frameCh <-chan *Frame) {
    go func() {
        for frame := range frameCh {
            s.distributeToWorkers(frame)  // Trust caller for freshness
        }
    }()
}
```

**Pros**:
- ✅ Simpler (no inbox mailbox, 50 LOC saved)
- ✅ Flexible (caller controls buffering)

**Cons**:
- ❌ Violates "casa de herrero" principle (preach JIT, accept stale)
- ❌ Operational ambiguity (are drops in supplier or upstream?)
- ❌ Latency unpredictable (depends on caller's buffering)

**Verdict**: ❌ Reject (inconsistent with Orion philosophy)

---

### Alternative B: Hybrid (Inbox + Channel Mode)

```go
// Support both modes
func (s *Supplier) PublishDirect(frame *Frame)  // Inbox mode (JIT)
func (s *Supplier) Start(ch <-chan *Frame)      // Channel mode (buffered)
```

**Pros**:
- ✅ Flexibility (caller chooses)

**Cons**:
- ❌ Dual API (confusing, when to use which?)
- ❌ Testing burden (2× test matrix)
- ❌ Documentation burden ("which mode should I use?")

**Verdict**: ❌ Reject (YAGNI, adds complexity without clear benefit)

---

### Alternative C: Symmetric JIT (Chosen)

**Pros**:
- ✅ Conceptual consistency (same pattern everywhere)
- ✅ Predictable behavior (JIT guaranteed)
- ✅ Clear operational metrics (drops at each level)

**Cons**:
- ❌ Complexity (inbox + goroutine + lifecycle)
- ❌ Integration burden (need adapter for buffered channels)

**Verdict**: ✅ **Accept** (aligns with Orion philosophy, worth the complexity)

---

## Toyota JIT Analogy

**Traditional Manufacturing** (buffered):
```
Supplier → 100-unit warehouse → Assembly Line → Product
```
- Inventory hides problems (supplier delays not visible)
- Warehouse cost (storage, management, staleness)

**Toyota JIT**:
```
Supplier → Just-in-time delivery → Assembly Line → Product
```
- No inventory (parts arrive as needed)
- Problems visible immediately (supplier delay stops line)
- Cost savings (no warehouse, no stale inventory)

**Orion JIT**:
```
GStreamer → Inbox (1 frame) → Distribution → Workers → Inference
```
- No buffering (frames arrive fresh)
- Problems visible (inbox drops → distribution slow)
- Latency savings (no stale frames, <100ms end-to-end)

**Philosophy**: **Expose problems, don't hide them with buffering.**

---

## Operational Monitoring

### Two-Level Drop Tracking

**Inbox Drops** (should be ~0):
```go
stats := supplier.Stats()
if stats.InboxDrops > 0 {
    log.Error("Distribution loop can't keep up",
        "inboxDrops", stats.InboxDrops)
    // Action: Investigate distributionLoop (deadlock? CPU starvation?)
}
```

**Worker Drops** (expected):
```go
for workerID, stat := range stats.WorkerStats {
    if stat.TotalDrops > threshold {
        log.Warn("Worker slow", "id", workerID, "drops", stat.TotalDrops)
        // Action: Operational decision (acceptable for BestEffort workers)
    }
}
```

**Benefit**: Drops at each level provide **diagnostic signal** (where is the bottleneck?).

---

## Testing Strategy

### Integration Tests

1. **High-rate publish → slow consume**:
   - Publish 100 frames @ 100fps (10ms interval)
   - Worker consumes @ 1fps (1000ms interval)
   - Assert: Worker receives latest frame (frame 100), not first frame (frame 0)
   - Assert: InboxDrops ~0 (distribution keeps up)
   - Assert: WorkerDrops ~99 (expected @ 100:1 ratio)

2. **Graceful shutdown with inbox**:
   - Publish 10 frames
   - Stop() while distributionLoop is blocked (inbox empty)
   - Assert: distributionLoop exits (ctx.Done handled)
   - Assert: No goroutine leaks (wg.Wait completes)

---

## References

- Toyota Production System: https://en.wikipedia.org/wiki/Toyota_Production_System
- Orion philosophy: CLAUDE.md ("Drop frames, never queue")
- ARCHITECTURE.md: Concurrency model
- ADR-001: sync.Cond for Mailbox Semantics (primitive used for inbox)

---

## Related Decisions

- **ADR-001**: sync.Cond (enables inbox mailbox)
- **ADR-002**: Zero-Copy (no amplification of copy cost at inbox)
- **ADR-003**: Batching (distribution latency impacts inbox drop rate)

---

## Future Considerations

### Phase 2: Multi-Stream

**Challenge**: Each stream has different FPS (stream1 @ 30fps, stream2 @ 5fps)

**Solution**: Per-stream inboxes:
```go
type Supplier struct {
    inboxes sync.Map  // streamID → *Inbox
}

func (s *Supplier) Publish(streamID string, frame *Frame) {
    inbox := s.getInbox(streamID)
    inbox.Publish(frame)
}
```

**Symmetry preserved**: Same JIT pattern, just N inboxes instead of 1.

---

**Review Status**: 🟡 Design Proposal (Implementation Pending)
