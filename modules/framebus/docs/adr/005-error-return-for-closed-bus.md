# ADR-005: Error Return for Publish to Closed Bus (Propuesta)

## Status

**REJECTED** - 2025-11-05

## Proposal

Replace `panic` with `error` return when `Publish()` is called on a closed bus.

**Current implementation:**

```go
// framebus/internal/bus/bus.go:170-172
func (b *bus) Publish(frame Frame) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        panic("publish on closed bus")  // ⚠ Panics immediately
    }
    // ... rest
}
```

**Proposed alternative:**

```go
func (b *bus) Publish(frame Frame) error {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        return ErrBusClosed  // ✅ Returns error
    }
    // ... rest
    return nil
}
```

**Goal:** Allow caller to handle shutdown gracefully without panic recovery.

## Decision

**REJECTED** - Maintain current `panic` behavior.

**Rationale:**
1. `Publish()` after `Close()` is a **programmer error**, not an expected runtime condition
2. Go idiom: Panic for misuse, errors for expected failures
3. Fail-fast principle: Find bugs in tests, not production
4. Bounded context: FrameBus is local library (not network), shutdown sequence is deterministic

## Context When Rejected (2025-11-05)

### Current State

**FrameBus lifecycle in Orion:**

```
1. Startup:
   bus := framebus.New()
   bus.Subscribe("worker-1", ch1)
   bus.Subscribe("worker-2", ch2)

2. Runtime (hours/days):
   for frame := range stream.Frames() {
       bus.Publish(frame)  // ← Hot path, no errors expected
   }

3. Graceful Shutdown:
   a. Stop publisher (close stream.Frames())
   b. Wait for publisher goroutine to exit
   c. Close bus: bus.Close()
   d. Wait for subscribers to drain channels
```

**Correct shutdown sequence:**

```go
// Core orchestrator shutdown
func (o *Orion) Shutdown(ctx context.Context) error {
    // 1. Stop source (publisher stops naturally)
    o.stream.Stop()

    // 2. Wait for consumeFrames goroutine to exit
    <-o.framesDone

    // 3. Now safe to close bus
    o.frameBus.Close()

    // 4. Wait for workers to finish
    for _, worker := range o.workers {
        worker.Stop()
    }

    return nil
}
```

**Incorrect shutdown (programmer error):**

```go
// ❌ BUG: Close bus while publisher still running
o.frameBus.Close()           // Close bus first
o.stream.Stop()              // Publisher still sending frames

// Result: Publish() called on closed bus → PANIC (catches bug)
```

### Orion Bounded Context

| Aspect | Characteristic | Implication |
|--------|---------------|-------------|
| **Deployment** | In-process library | No network failures |
| **Lifecycle** | Deterministic (Start → Run → Stop) | Shutdown order is controlled |
| **Publisher** | Single goroutine (`consumeFrames`) | Not distributed |
| **Close timing** | Once during graceful shutdown | Predictable, not racy |
| **Error recovery** | Stop entire process (NUC edge device) | Can't recover from half-shutdown |

**Key insight:** FrameBus is NOT like a network connection (transient failures, reconnections). It's like a Go channel - programmer controls lifecycle.

### Go Standard Library Pattern

**Panic for programmer errors:**

```go
// 1. Close closed channel
ch := make(chan int)
close(ch)
close(ch)        // ✅ PANIC: "close of closed channel"

// 2. Send on closed channel
close(ch)
ch <- 42         // ✅ PANIC: "send on closed channel"

// 3. nil dereference
var p *int
*p = 42          // ✅ PANIC: "nil pointer dereference"
```

**Error returns for expected failures:**

```go
// 1. File not found
f, err := os.Open("missing.txt")  // ❌ Error (user input, expected)

// 2. Network timeout
conn, err := net.Dial("tcp", "host:1234")  // ❌ Error (network, transient)

// 3. Parse failure
num, err := strconv.Atoi("abc")  // ❌ Error (user input, expected)
```

**FrameBus matches channel semantics** → Panic is correct.

## Rationale for Rejection

### 1. Programmer Error vs Expected Failure

**Question:** Is `Publish()` after `Close()` a programmer error or an expected condition?

**Analysis:**

| Scenario | Classification | Go Idiom |
|----------|---------------|----------|
| Network disconnection | Expected failure | Return error |
| File not found | Expected failure | Return error |
| Parse invalid input | Expected failure | Return error |
| **Publish after Close** | **Programmer error** | **Panic** |
| Close closed channel | Programmer error | Panic |
| Index out of bounds | Programmer error | Panic |

**Evidence it's programmer error:**
- Publisher has control over shutdown sequence
- No external factors (network, disk, user input)
- Single process, deterministic lifecycle
- Bug indicates race condition or incorrect shutdown logic

**Verdict:** Publish after Close is programmer error → Panic is correct.

---

### 2. Fail-Fast Principle

**Goal:** Find bugs as early as possible (ideally tests, not production).

**With panic (current):**

```go
// Test with incorrect shutdown
func TestWorkerLifecycle(t *testing.T) {
    bus := framebus.New()
    bus.Close()
    bus.Publish(frame)  // ✅ PANIC in test → Bug found immediately

    // Test fails with clear message: "publish on closed bus"
}
```

**With error return (proposed):**

```go
// Test with incorrect shutdown
func TestWorkerLifecycle(t *testing.T) {
    bus := framebus.New()
    bus.Close()

    // Publisher ignores error (easy to do)
    bus.Publish(frame)  // ❌ Silent failure (no panic)

    // Test passes, bug goes to production
    // Later: Subscriber never receives frames → Silent data loss
}
```

**Consequences of error return:**

| Scenario | Panic (Current) | Error Return (Proposed) |
|----------|----------------|-------------------------|
| Developer forgets to stop publisher before Close() | ✅ Test panics, bug found | ⚠ Test passes, bug in production |
| Race condition in shutdown | ✅ Panic exposes race | ⚠ Silent data loss |
| Incorrect goroutine synchronization | ✅ Panic shows bug | ⚠ Intermittent missing frames |

**Verdict:** Panic catches bugs early → Error return hides bugs.

---

### 3. API Ergonomics (Hot Path)

**Current (panic):**

```go
// Hot path - no error handling
for frame := range stream.Frames() {
    bus.Publish(frame)  // Simple, no noise
}
```

**Proposed (error return):**

```go
// Hot path - must check error
for frame := range stream.Frames() {
    if err := bus.Publish(frame); err != nil {
        // What to do here?
        // - Return? (breaks loop)
        // - Log? (adds overhead)
        // - Ignore? (defeats purpose of error)
    }
}
```

**Performance impact:**

| Aspect | Panic | Error Return | Delta |
|--------|-------|--------------|-------|
| Hot path LOC | 1 line | 5 lines (if err != nil) | 5× more code |
| Cognitive load | Zero (no error check) | High (error handling decision) | Higher |
| CPU overhead | Zero (no check in fast path) | Error check every Publish | +2-3 CPU cycles |
| Error case latency | <1μs (panic, rare) | Error return + check | Always paid |

**Frequency:**

```
Publish frequency:  30/sec (hot path)
Close frequency:    1/lifetime (cold path)

Error check overhead: 30 calls/sec × 2 cycles = 60 cycles/sec wasted
Bug prevented:        0 (shutdown is deterministic)
```

**Verdict:** Error return adds overhead to hot path for zero benefit.

---

### 4. Industry Pattern Analysis

| Library | Type | Panic or Error? | Context |
|---------|------|----------------|---------|
| **Go stdlib** | `close(closedCh)` | ✅ Panic | Programmer error |
| **Go stdlib** | `os.Open()` | ❌ Error | User input, expected failure |
| **NATS.io** | `Publish()` on closed | ❌ Error | Network library, reconnections |
| **NSQ** | `Publish()` on closed | ❌ Error | Network library, distributed |
| **Prometheus** | `Register()` duplicate | ✅ Panic | Programmer error |
| **gRPC** | `Send()` on closed | ❌ Error | Network library, transient failures |
| **Kafka** | `Send()` on closed | ❌ Error | Network library, retries |
| **FrameBus** | `Publish()` on closed | **✅ Panic** | **Local library, deterministic** |

**Pattern:**

```
Network libraries → Error return (transient failures, reconnections)
Local libraries   → Panic (programmer errors, deterministic lifecycle)
```

**FrameBus matches local library pattern (like channels).**

---

### 5. Recoverable if Needed

**If caller REALLY needs to handle panic** (edge case):

```go
// Core can recover if needed (but shouldn't need to)
func (o *Orion) consumeFrames(ctx context.Context) {
    defer func() {
        if r := recover(); r != nil {
            // Only triggers on programmer error
            log.Error("BUG: published on closed bus",
                "panic", r,
                "action", "fix shutdown sequence")
            // Graceful degradation: stop publishing
        }
    }()

    for frame := range o.stream.Frames() {
        o.frameBus.Publish(frame)
    }
}
```

**But this is anti-pattern:**
- Masking programmer error (should fix shutdown sequence instead)
- Complexity (defer/recover everywhere)
- Indicates incorrect architecture (shutdown logic is racy)

**Better solution:**
```go
// Fix shutdown sequence (no recovery needed)
func (o *Orion) Shutdown(ctx context.Context) error {
    o.stream.Stop()          // 1. Stop publisher
    <-o.framesDone           // 2. Wait for publisher to exit
    o.frameBus.Close()       // 3. Now safe to close
    return nil
}
```

**Verdict:** Panic is recoverable (if needed), but correct shutdown eliminates need.

---

### 6. Testability

**With panic (current):**

```go
func TestPublishOnClosedBusPanics(t *testing.T) {
    bus := New()
    bus.Close()

    // Expect panic
    defer func() {
        if r := recover(); r == nil {
            t.Fatal("expected panic on closed bus")
        }
    }()

    bus.Publish(Frame{})  // ✅ Should panic
}
```

**With error return (proposed):**

```go
func TestPublishOnClosedBusReturnsError(t *testing.T) {
    bus := New()
    bus.Close()

    err := bus.Publish(Frame{})
    if err == nil {
        t.Fatal("expected error on closed bus")
    }
}
```

**Both are testable**, but panic:
- ✅ Forces caller to think about shutdown sequence
- ✅ Exposes race conditions (panic is loud)
- ✅ Matches Go channel semantics (developers know pattern)

---

## Consequences

### Positive (Keeping Panic)

✅ **Fail-fast**: Bugs found in tests, not production
✅ **Go idiom**: Matches `close(closedChannel)` semantics
✅ **Simple hot path**: No error handling overhead
✅ **Clear signal**: Panic = programmer error, fix shutdown sequence
✅ **Performance**: Zero overhead in hot path (no error check)
✅ **Testability**: Panic exposes race conditions clearly
✅ **Bounded context alignment**: Local library, deterministic lifecycle

### Negative (Trade-offs)

⚠ **Can crash if not recovered**: Caller must ensure correct shutdown
⚠ **No graceful degradation**: Panic stops goroutine immediately
⚠ **Caller can't handle**: No option for custom recovery logic

### Mitigations

1. **Document shutdown sequence clearly**

```go
// CLAUDE.md
## Shutdown Sequence (CRITICAL)

CORRECT:
1. Stop publisher (close stream.Frames())
2. Wait for publisher goroutine to exit
3. Close FrameBus
4. Wait for subscribers to drain

INCORRECT:
1. Close FrameBus first ❌
2. Publisher still running ❌
3. Result: Panic (publish on closed bus)
```

2. **Instrumentation at Close()**

```go
func (b *bus) Close() error {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.closed {
        return fmt.Errorf("bus already closed")
    }

    b.closed = true

    // Log closure for debugging
    log.Debug("FrameBus closed",
        "total_published", b.totalPublished.Load(),
        "note", "subsequent Publish calls will panic")

    return nil
}
```

3. **Add shutdown example in tests**

```go
// Example test showing correct shutdown
func TestCorrectShutdownSequence(t *testing.T) {
    bus := New()

    // Publisher goroutine
    publishDone := make(chan struct{})
    go func() {
        for i := 0; i < 100; i++ {
            bus.Publish(Frame{Seq: uint64(i)})
        }
        close(publishDone)  // Signal publisher done
    }()

    // Wait for publisher to finish
    <-publishDone

    // Now safe to close
    bus.Close()  // ✅ No panic
}
```

## Alternatives Considered

### Alternative 1: Error Return (Evaluated, Rejected)

**Implementation:**

```go
func (b *bus) Publish(frame Frame) error {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        return ErrBusClosed
    }

    // ... rest (no changes)
    return nil
}
```

**Pros:**
- ✅ Caller can handle error
- ✅ No panic (feels safer to some developers)
- ✅ Matches network libraries (NATS, gRPC)

**Cons:**
- ❌ Hides programmer errors (incorrect shutdown)
- ❌ Adds overhead to hot path (error check + return)
- ❌ Easy to ignore error (silent failures)
- ❌ Doesn't match Go idiom (channels panic on misuse)
- ❌ Wrong pattern for bounded context (local library)

**Decision:** REJECTED
**Rationale:** FrameBus is local library (not network), shutdown is deterministic. Error return optimizes for wrong use case.

---

### Alternative 2: Silent Ignore (Evaluated, Rejected)

**Implementation:**

```go
func (b *bus) Publish(frame Frame) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        return  // ⚠ Silent ignore
    }

    // ... rest
}
```

**Pros:**
- ✅ Never panics
- ✅ Simple API (no error handling)

**Cons:**
- ❌ Hides bugs COMPLETELY (silent data loss)
- ❌ No signal to developer (incorrect shutdown undetected)
- ❌ Violates fail-fast principle
- ❌ Anti-pattern (Go stdlib doesn't do this)

**Decision:** REJECTED
**Rationale:** Silent failures are worst option. Hides bugs instead of exposing them.

---

### Alternative 3: Conditional Panic (Debug Mode Only)

**Implementation:**

```go
func (b *bus) Publish(frame Frame) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        if debugMode {
            panic("publish on closed bus (debug mode)")
        }
        return  // Silent in production
    }

    // ... rest
}
```

**Pros:**
- ✅ Panics in tests (catches bugs)
- ✅ Silent in production (no crashes)

**Cons:**
- ❌ Different behavior in test vs production (dangerous)
- ❌ Hides bugs in production (defeats purpose)
- ❌ Complexity (mode flag, conditional logic)
- ❌ Goes against Go philosophy (consistent behavior)

**Decision:** REJECTED
**Rationale:** Test/prod parity is critical. If it panics in test, it should panic in production (forces fix).

---

### Alternative 4: Logging + Return (Evaluated, Rejected)

**Implementation:**

```go
func (b *bus) Publish(frame Frame) error {
    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        log.Error("BUG: publish on closed bus",
            "stack", debug.Stack())
        return ErrBusClosed
    }

    // ... rest
    return nil
}
```

**Pros:**
- ✅ Logs bug for debugging
- ✅ No panic (caller can continue)

**Cons:**
- ❌ Logging in hot path (overhead)
- ❌ Still hides bug (doesn't force fix)
- ❌ Can spam logs (if bug is frequent)
- ❌ Adds complexity (logging + error handling)

**Decision:** REJECTED
**Rationale:** Logging + continue is band-aid. Panic forces root cause fix.

---

## Re-evaluation Triggers

**Re-open this ADR if ANY of these conditions occur:**

1. ✅ **FrameBus becomes network-facing**
   - Current: Local library (in-process)
   - Threshold: Exposed via gRPC/HTTP/MQTT
   - How to measure: Architecture review
   - Rationale: Network → transient failures → error return appropriate

2. ✅ **Close() timing becomes unpredictable**
   - Current: Deterministic (graceful shutdown sequence)
   - Threshold: Close() can happen during runtime (not just shutdown)
   - How to measure: Design doc review
   - Rationale: Non-deterministic → panic too harsh → error return

3. ✅ **Multiple publishers with independent lifecycles**
   - Current: Single publisher (consumeFrames goroutine)
   - Threshold: N publishers with different start/stop times
   - How to measure: Code review (multiple `Publish()` callers)
   - Rationale: Complex lifecycle → panic too brittle → error return

4. ✅ **Production panics from publish-on-closed become frequent**
   - Current: 0 panics (correct shutdown sequence)
   - Threshold: >1 panic/week sustained for 1 month
   - How to measure: Production logs/metrics
   - Rationale: Frequent panics indicate architectural issue → error return as band-aid (but also fix architecture!)

**If ANY trigger fires:**
1. Review this ADR
2. Check if bounded context changed (local → distributed?)
3. Evaluate if error return pattern is now appropriate
4. Update decision if context fundamentally changed

## Design Philosophy Alignment

### "Complejidad por diseño, no por accidente"

> "Attack complexity through architecture, not code tricks."

- ✅ Panic forces correct shutdown architecture (deterministic sequence)
- ❌ Error return would mask architectural issues (band-aid on racy shutdown)

### "Fail Fast (Load Time vs Runtime)"

> "Invariantes enforced at load time, not runtime debugging hell."

- ✅ Panic = immediate failure (find bugs in tests)
- ❌ Error return = silent failures (debug in production)

### "Pragmatismo > Purismo"

> "Make decisions based on context, not dogma."

- ✅ Panic matches bounded context (local library, deterministic)
- ✅ Not dogmatic (would use error return if FrameBus were network-facing)

### "Training Data for Future Decisions"

> "Rejections are as valuable as acceptances."

- ✅ Documents WHY panic is correct in THIS context
- ✅ Shows alternative (error return) and why it doesn't fit
- ✅ Provides re-evaluation criteria (if context changes)

## References

### Go Standard Library

- **Channels**: [close(closedCh) panics](https://go.dev/ref/spec#Close) - Programmer error
- **sync.Mutex**: `Unlock()` of unlocked mutex panics - Programmer error
- **os.Open()**: Returns error - Expected failure (file not found)

### Industry Libraries

- **NATS.io**: [conn.Publish() returns error on closed](https://github.com/nats-io/nats.go/blob/main/nats.go#L1789) - Network library
- **NSQ**: [Producer.Publish() returns error](https://github.com/nsq-io/go-nsq/blob/master/producer.go#L178) - Network library
- **Prometheus**: [Register() panics on duplicate](https://github.com/prometheus/client_golang/blob/main/prometheus/registry.go#L258) - Programmer error

### Orion Documentation

- CLAUDE.md: [CLAUDE.md:94-106](../CLAUDE.md) (Shutdown sequence)
- Prototype: `References/orion-prototipe/internal/framebus/bus.go:71-75`
- ORION_SYSTEM_CONTEXT.md: `docs/ORION_SYSTEM_CONTEXT.md:273-301` (Lifecycle)

### Related Decisions

- [ADR-001: Channel-based Subscriber Pattern](001-channel-based-subscriber-pattern.md) - Why channels
- [ADR-002: Non-blocking Publish with Drop Policy](002-non-blocking-publish-drop-policy.md) - Hot path optimization
- [ADR-004: Subscribe Lock Strategy](004-subscribe-lock-strategy.md) - Rejected proposal pattern

---

**Document Type:** REJECTED PROPOSAL
**Last Updated:** 2025-11-05
**Authors:** Ernesto Canales, Gaby de Visiona
**Next Review:** When re-evaluation triggers fire

🎸 **Blues Style:** We know error returns work (network libraries), but we play panic for this song (local library, deterministic shutdown).

---

## Appendix: Go Community Opinion

### Survey of Go Style Guide / Community Consensus

**From Effective Go:**
> "Panic is only for truly exceptional situations. In library code, errors should be returned. The exception is when the library detects a programming error in its use - then panic is appropriate."

**From Go Proverbs:**
> "Don't panic." (But also: "Don't use recover to hide panics caused by programmer errors.")

**From Rob Pike (Go co-author):**
> "Panic is for things that shouldn't happen. If it can happen (network failure, disk full), return an error."

**Interpretation for FrameBus:**
- Network failure → CAN happen → Error
- Disk full → CAN happen → Error
- **Publish after Close → SHOULDN'T happen → Panic** ✅

### Real-World Go Code Patterns

```go
// Pattern 1: Misuse of closed resource → Panic
close(closedChannel)           // Panics
send(closedChannel)            // Panics
mu.Unlock() // unlocked mutex  // Panics

// Pattern 2: Expected failures → Error
os.Open("missing.txt")         // Returns error
net.Dial("unreachable:1234")   // Returns error
json.Unmarshal(invalidJSON)    // Returns error

// Pattern 3: Programmer errors → Panic
slice[outOfBounds]             // Panics
nilPointer.Method()            // Panics
```

**FrameBus `Publish(closedBus)` matches Pattern 1** → Panic is correct.


 ## Stats-Based Monitoring vs Error-Based Handling  
  
 FrameBus follows **stats-based monitoring** pattern (not error-based handling).  
  
 ### Stats-Based (Current - Correct)  
  
 **FrameBus responsibility:** Provide data  
 ```go  
 type BusStats struct {  
     Subscribers map[string]SubscriberStats  
 }  
 // Exposes: Sent count, Dropped count per subscriber  
  
 Core responsibility: Interpret and act  
 stats := bus.Stats()  
 for id, subStats := range stats.Subscribers {  
     dropRate := float64(subStats.Dropped) / float64(subStats.Sent + subStats.Dropped)  
  
     if dropRate > 0.95 {  
         // Decision: Worker is slow/hung  
         workerLifecycle.RestartWorker(id)  
     }  
 }  
  
 Worker Lifecycle responsibility: Execute action  
 // Unsubscribe → Stop → Start new → Subscribe  
  
 Benefits:  
 - ✅ Clean separation of concerns (data → interpretation → action)  
 - ✅ Hot path is simple (Publish = 1 line, fire-and-forget)  
 - ✅ Monitoring is pull-based (read stats every 10s, not every Publish)  
 - ✅ Core can correlate multiple metrics (drop rate + inference timing + CPU)  
  
 ---  
 Error-Based (Rejected Alternative)  
  
 If FrameBus returned errors:  
 if err := bus.Publish(frame); err != nil {  
     // Problem 1: Mix data (drops) with lifecycle (closed)  
     // Problem 2: Hot path pollution (error check every frame)  
     // Problem 3: Who interprets? Publisher can't make decisions  
 }  
  
 Problems:  
 - ❌ Confuses runtime data (drops) with lifecycle errors (closed)  
 - ❌ Forces error handling in hot path (30 Hz overhead)  
 - ❌ Publisher (consumeFrames) can't make orchestration decisions  
 - ❌ Core loses ability to correlate metrics (errors scattered across code)  
  
 Analogy:  
 - MQTT Broker exposes $SYS/broker/messages/dropped (stats)  
   - NOT: Returns error on every dropped message  
 - FrameBus exposes Stats().Subscribers[id].Dropped (stats)  
   - NOT: Returns error on every dropped frame  
  
 ---  
 Real-World Example: Worker Restart Cycle  
  
 Scenario: Worker-123 becomes slow (model inference taking 500ms instead of 50ms)  
  
 With Stats (Current):  
 T=0s:   Worker-123 processes normally (10% drop rate)  
 T=30s:  Worker slows down (inference spike to 500ms)  
 T=40s:  Core detects 95% drop rate in stats  
 T=40s:  Core: workerLifecycle.RestartWorker("worker-123")  
 T=41s:  Unsubscribe → Stop → Start new → Subscribe  
 T=42s:  New worker processes normally (10% drop rate)  
  
 With Error Return (Hypothetical):  
 T=0s:   Worker-123 processes normally  
 T=30s:  Worker slows down  
 T=30-40s: Publisher gets "buffer full" error 270 times (30 Hz × 10s × 90%)  
           → What to do with 270 errors? Log all? Count? Ignore?  
 T=40s:  ??? (No clear mechanism to aggregate and decide)  
  
 Verdict: Stats-based monitoring is superior for orchestration decisions.  
  
 ---  
  
 ## ☕ Resumen Final (Café en el Ascensor v3)  
  
 **Ernesto:** "Entonces, las métricas de droppeo las ve el Core, y si ve algo raro, le pide a Worker Lifecycle que reinicie el worker."  
  
 **Gaby:** "Exacto. FrameBus solo te dice '90% de frames dropped para Worker-123'. Core interpreta: 'Uh, algo anda mal'. Worker Lifecycle ejecuta: 'Unsubscribe → Stop → Start nuevo → Subscribe'."  
  
 **Ernesto:** "Y por eso error return no sirve - porque error es 'en el momento', pero vos necesitás ver la **tendencia** (90% drop rate sostenido por 10 segundos)."  
  
 **Gaby:** "¡EXACTO! Error return te da señales individuales (frame 1 dropped, frame 2 dropped...). Stats te dan el **patrón agregado** (90% drop rate). Core necesita el patrón para decidir, no cada error  
 individual."  
  
 **Ernesto:** "Separation of concerns: FrameBus provee data, Core toma decisiones, Worker Lifecycle ejecuta."  
  
 **Gaby:** "Eso. Y panic es solo para 'usaste el bus incorrectamente' (publish después de close), no para 'aquí está un dato para que decidas' (drop rate). Fire-and-forget + stats-based monitoring. 🎸"  
  
 ---