# ADR-001: sync.Cond for Mailbox Semantics

**Status**: Accepted
**Date**: 2025-01-05
**Authors**: Ernesto + Gaby

---

## Changelog

| Version | Date       | Author          | Changes                           |
|---------|------------|-----------------|-----------------------------------|
| 1.0     | 2025-01-05 | Ernesto + Gaby  | Initial decision - sync.Cond      |

---

## Context

FrameSupplier needs to implement **mailbox semantics** for frame distribution:

1. **Non-blocking publish**: stream-capture calls `Publish(*Frame)` at 30fps, must never block
2. **Blocking consume**: Workers call `readFunc()`, should block when no frame available (efficient waiting)
3. **Overwrite policy**: New frame replaces old unconsumed frame (JIT semantics)

**Question**: What Go synchronization primitive should we use?

### Options Considered

#### Option A: Buffered Channels
```go
ch := make(chan *Frame, 1)

// Publisher
select {
case ch <- frame:
default:
    // Channel full, but old frame still inside!
}

// Worker
frame := <-ch  // Blocks when empty
```

**Problem**: Drop semantics are wrong.
- When channel full, we drop **new** frame (keep old)
- We want to drop **old** frame (keep new)

#### Option B: Unbuffered Channel + select
```go
ch := make(chan *Frame)

// Publisher
select {
case ch <- frame:
default:
    // No receiver ready, drop
}

// Worker
frame := <-ch  // Blocks when no publisher
```

**Problem**: No mailbox (storage).
- If worker slow, every frame drops (no "latest frame" concept)
- Publish only succeeds if worker waiting (tight coupling)

#### Option C: sync.Mutex + Polling
```go
var (
    mu    sync.Mutex
    frame *Frame
)

// Publisher
mu.Lock()
frame = newFrame
mu.Unlock()

// Worker (busy-wait)
for {
    mu.Lock()
    if frame != nil {
        f := frame
        frame = nil
        mu.Unlock()
        return f
    }
    mu.Unlock()
    time.Sleep(1 * time.Millisecond)  // Polling interval
}
```

**Problem**: CPU waste (busy-wait) or high latency (sleep duration).

#### Option D: sync.Cond (Mailbox Pattern)
```go
var (
    mu    sync.Mutex
    cond  *sync.Cond
    frame *Frame
)

func init() {
    cond = sync.NewCond(&mu)
}

// Publisher (non-blocking)
mu.Lock()
frame = newFrame  // Overwrites old frame
cond.Signal()     // Wake waiting worker
mu.Unlock()

// Worker (blocking)
mu.Lock()
for frame == nil {
    cond.Wait()  // Efficient sleep, releases lock
}
f := frame
frame = nil  // Mark as consumed
mu.Unlock()
return f
```

**Benefits**:
- ✅ Non-blocking publish (lock + assign + signal ~1µs)
- ✅ Blocking consume with efficient wait (no busy-wait)
- ✅ Correct drop semantics (new overwrites old)
- ✅ Single-slot mailbox (JIT semantics)

---

## Decision

**We will use `sync.Cond` for mailbox implementation.**

### Implementation Pattern

**Inbox Mailbox** (stream-capture → Supplier):
```go
type Supplier struct {
    inboxMu    sync.Mutex
    inboxCond  *sync.Cond
    inboxFrame *Frame
}

func (s *Supplier) Publish(frame *Frame) {
    s.inboxMu.Lock()
    s.inboxFrame = frame  // Overwrite
    s.inboxCond.Signal()
    s.inboxMu.Unlock()
}

func (s *Supplier) distributionLoop() {
    for {
        s.inboxMu.Lock()
        for s.inboxFrame == nil {
            s.inboxCond.Wait()  // Block until frame
        }
        frame := s.inboxFrame
        s.inboxFrame = nil  // Consume
        s.inboxMu.Unlock()

        s.distributeToWorkers(frame)
    }
}
```

**Worker Slot Mailbox** (Supplier → Worker):
```go
type WorkerSlot struct {
    mu    sync.Mutex
    cond  *sync.Cond
    frame *Frame
}

func (s *Supplier) publishToSlot(slot *WorkerSlot, frame *Frame) {
    slot.mu.Lock()
    slot.frame = frame  // Overwrite
    slot.cond.Signal()
    slot.mu.Unlock()
}

func (s *Supplier) Subscribe(workerID string) func() *Frame {
    slot := &WorkerSlot{}
    slot.cond = sync.NewCond(&slot.mu)

    return func() *Frame {
        slot.mu.Lock()
        for slot.frame == nil {
            slot.cond.Wait()
        }
        f := slot.frame
        slot.frame = nil  // Consume
        slot.mu.Unlock()
        return f
    }
}
```

---

## Consequences

### Positive ✅

1. **Correct Semantics**: New frame overwrites old (JIT principle)
2. **Efficient Blocking**: `cond.Wait()` releases lock, no CPU waste
3. **Low Latency**: Publish() ~1µs (lock + assign + signal)
4. **Standard Pattern**: sync.Cond is designed for this use case

### Negative ❌

1. **Less Familiar**: sync.Cond less common than channels in Go codebases
2. **Manual Locking**: Requires careful mutex management (defer unlock patterns)
3. **No Select**: Cannot use `select` statement (channel-specific feature)

### Mitigation

- **Documentation**: Clear code comments explaining sync.Cond usage
- **Encapsulation**: Mailbox logic isolated in Supplier (not exposed to callers)
- **Testing**: Unit tests for concurrent scenarios (race detector)

---

## Alternatives Considered

| Alternative          | Pros                          | Cons                              | Verdict   |
|----------------------|-------------------------------|-----------------------------------|-----------|
| Buffered Channel     | Familiar, simple              | Wrong drop semantics (keep old)   | ❌ Reject |
| Unbuffered Channel   | Simple                        | No mailbox, tight coupling        | ❌ Reject |
| Mutex + Polling      | Simple locking                | CPU waste (busy-wait)             | ❌ Reject |
| **sync.Cond**        | **Correct semantics + efficient** | **Less familiar**             | ✅ **Accept** |

---

## References

- Go sync.Cond docs: https://pkg.go.dev/sync#Cond
- Mailbox pattern: Actor model literature
- ARCHITECTURE.md: Algorithm details
- C4_MODEL.md: Component views

---

## Related Decisions

- **ADR-002**: Zero-Copy Frame Sharing (impacts mutex critical section)
- **ADR-004**: Symmetric JIT Architecture (reuses this pattern at multiple levels)
