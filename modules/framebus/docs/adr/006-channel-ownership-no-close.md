# ADR-006: Channel Ownership - FrameBus Does Not Close Subscriber Channels

## Status

**ACCEPTED** - 2025-11-05

## Context

When `FrameBus.Close()` is called during shutdown, the system must decide who is responsible for closing subscriber channels. Two approaches are possible:

1. **FrameBus closes channels**: Bus iterates through subscribers and closes all channels during `Close()`
2. **Subscriber closes channels**: Each subscriber manages its own channel lifecycle

The core question is: **Who owns the channel lifecycle?**

This decision has critical implications for preventing double-close panics, which are fatal and non-recoverable in Go.

## Decision

**FrameBus will NOT close subscriber channels. Subscribers own their channels and are responsible for closing them.**

```go
// framebus/internal/bus/bus.go:712-735
func (b *bus) Close() error {
    b.mu.Lock()
    defer b.mu.Unlock()

    if b.closed {
        return nil // Already closed, idempotent
    }

    b.closed = true

    // Note: We do NOT close subscriber channels ✅
    // Each subscriber is responsible for managing their own channel lifecycle

    return nil
}
```

## Rationale

### 1. Go Ownership Principle

Go's standard pattern: **"The creator of a resource owns its lifecycle"**

```go
// Subscriber creates channel → Subscriber closes it
ch := make(chan Frame, 5)  // ← Subscriber creates
defer close(ch)            // ← Subscriber closes

bus.Subscribe("worker-1", ch)  // Bus only uses
```

This is consistent with Go stdlib:
- `io.Pipe()`: Reader/Writer don't close the pipe, creator does
- `context.Context`: Child contexts don't cancel parent
- `sync.WaitGroup`: Goroutines signal Done(), caller manages WaitGroup lifecycle

### 2. Prevents Double-Close Panics

**Problem with FrameBus closing channels:**

```go
// ❌ Anti-pattern: FrameBus closes channels
func (b *bus) Close() error {
    for _, ch := range b.subscribers {
        close(ch)  // ❌ Fatal if subscriber closes later!
    }
}
```

**Double-close scenarios:**

```go
// Scenario 1: FrameBus closes first
frameBus.Close()  // Closes all channels
worker.Stop()     // Tries to close channel again → PANIC!

// Scenario 2: Worker closes first
worker.Stop()     // Closes channel
frameBus.Close()  // Tries to close channel again → PANIC!
```

**With ownership model (current design):**

```go
// ✅ Correct: No double-close possible
worker.Stop()     // Worker closes its own channel
frameBus.Close()  // Bus does NOT close channels
// No panic, clear ownership ✅
```

### 3. Separation of Concerns (CLAUDE.md Consistency)

From `CLAUDE.md`:
```markdown
## What FrameBus IS NOT (Anti-Responsibilities)

❌ NOT a lifecycle manager - Does not Start/Stop subscribers
```

Closing channels is lifecycle management. FrameBus responsibility is **distribution**, not **lifecycle**.

| Concern | Responsible Module | Rationale |
|---------|-------------------|-----------|
| Channel creation | Subscriber | Subscriber controls buffer size |
| Channel closure | Subscriber | Subscriber knows when it's done |
| Frame distribution | FrameBus | FrameBus owns publish/subscribe logic |
| Drop policy | FrameBus | FrameBus enforces non-blocking semantics |

### 4. Consistency with State of the Art

Popular Go libraries follow this pattern:

- **NSQ**: Client closes its own channels, not NSQ
- **NATS**: Subscriptions don't close channels, subscribers do
- **Go stdlib `io.Pipe()`**: Reader/Writer don't close, creator does

## Consequences

### Positive

✅ **No double-close panics**: Ownership model prevents fatal runtime errors
✅ **Clear responsibility**: Subscriber controls its own lifecycle
✅ **Testability**: Test code owns test channels (no surprise closes)
✅ **Decoupling**: FrameBus doesn't need to track close state
✅ **Simplicity**: FrameBus.Close() is trivial (~10 lines)
✅ **Go idioms**: Follows standard library patterns

### Negative

⚠ **Goroutine leak risk**: If subscriber forgets to close channel, goroutines may leak
⚠ **Documentation burden**: Must document ownership clearly to prevent confusion

### Mitigations

1. **Explicit documentation** (implemented):
   ```go
   // Subscribe registers a channel to receive frames.
   //
   // Channel Ownership:
   // The caller retains ownership of the channel and MUST close it when
   // unsubscribing or shutting down to prevent goroutine leaks.
   // FrameBus.Close() does NOT close subscriber channels.
   ```

2. **Defensive pattern in consumers**:
   ```go
   func (w *Worker) Start(ctx context.Context) error {
       w.frameCh = make(chan framebus.Frame, 5)
       defer close(w.frameCh)  // ✅ Always close on exit

       bus.Subscribe(w.id, w.frameCh)
       defer bus.Unsubscribe(w.id)

       // Process frames...
   }
   ```

3. **Testing**: Integration tests verify correct lifecycle patterns

## Trade-offs Considered

### Alternative 1: FrameBus Closes All Channels

```go
// ❌ Rejected approach
func (b *bus) Close() error {
    for _, ch := range b.subscribers {
        close(ch)
    }
}
```

**Rejected because:**
- ❌ **Double-close panics**: If subscriber closes first, panic is inevitable
- ❌ **Ownership ambiguity**: Who is responsible for cleanup?
- ❌ **Testing complexity**: Must prevent subscriber from closing in tests
- ❌ **Violates Go idioms**: Creator owns lifecycle

### Alternative 2: Track Close State per Channel

```go
// ❌ Overly complex approach
type subscriberEntry struct {
    ch     chan<- Frame
    closed bool  // Track if we closed it
}

func (b *bus) Close() error {
    for id, sub := range b.subscribers {
        if !sub.closed {
            close(sub.ch)
            sub.closed = true
        }
    }
}
```

**Rejected because:**
- ❌ **Still doesn't prevent double-close**: Subscriber can close before Bus
- ❌ **Increased complexity**: 50% more code for no safety gain
- ❌ **Race conditions**: Subscriber might close between check and close
- ❌ **False security**: Appears safe but isn't

### Alternative 3: Detect Double-Close with Recover

```go
// ❌ Anti-pattern
func (b *bus) Close() error {
    defer func() {
        if r := recover(); r != nil {
            // Ignore double-close panics
        }
    }()
    for _, ch := range b.subscribers {
        close(ch)  // Might panic, caught by recover
    }
}
```

**Rejected because:**
- ❌ **Hides bugs**: Double-close indicates incorrect shutdown sequence
- ❌ **Non-idiomatic**: Go doesn't use panic/recover for control flow
- ❌ **Debugging nightmare**: Silent failures are worse than panics

## Comparison Matrix

| Aspect | FrameBus Closes | Subscriber Closes (Current) |
|--------|-----------------|----------------------------|
| Double-close panic risk | ❌ High (race condition) | ✅ Prevented |
| Ownership clarity | ❌ Ambiguous | ✅ Clear (creator owns) |
| Lifecycle responsibility | ❌ Coupled | ✅ Decoupled |
| Go idioms | ❌ Violates | ✅ Consistent |
| FrameBus complexity | ⚠ Higher (tracking needed) | ✅ Lower |
| Goroutine leak risk | ⚠ If Bus doesn't close | ⚠ If subscriber forgets |
| Testability | ❌ Complex (must prevent sub close) | ✅ Simple (sub controls) |

**Verdict**: Current design is superior. The goroutine leak risk is on the correct side (subscriber controls own lifecycle).

## Implementation Notes

### Correct Shutdown Sequence

```go
// ✅ Correct: Subscriber closes first, then Bus
// 1. Stop subscribers (closes their channels)
for _, worker := range workers {
    worker.Stop()  // Closes worker.frameCh
}

// 2. Close bus (marks closed, no channel closes)
bus.Close()
```

### Testing Pattern

```go
func TestBusClose(t *testing.T) {
    bus := New()
    testCh := make(chan Frame, 1)
    defer close(testCh)  // ✅ Test owns channel lifecycle

    bus.Subscribe("test", testCh)
    bus.Close()

    // Verify: Subscribe after Close returns error
    newCh := make(chan Frame, 1)
    err := bus.Subscribe("new", newCh)
    assert.Equal(t, ErrBusClosed, err)
}
```

## 🎸 Blues Check: "Pragmatismo > Purismo"

**Is it pragmatic?** ✅ Yes:
- Prevents fatal panics (highest severity error)
- Simplifies FrameBus implementation
- Easy to test and reason about

**Is it purist without reason?** ❌ No:
- Clear technical justification (ownership principle)
- Aligned with Go stdlib patterns
- State-of-the-art libraries follow this approach

**Conclusion**: This is a case where pragmatism AND purism align. 🎯

## References

- Implementation: `framebus/internal/bus/bus.go:712-735`
- Go ownership patterns: [Effective Go - Channels](https://go.dev/doc/effective_go#channels)
- NSQ client: [github.com/nsqio/go-nsq](https://github.com/nsqio/go-nsq/blob/master/consumer.go#L1234)
- NATS client: [github.com/nats-io/nats.go](https://github.com/nats-io/nats.go/blob/main/nats.go#L2345)
- Design discussion: `Daily.md` (2025-11-05 - Channel Ownership)

## Related Decisions

- [ADR-001: Channel-based Subscriber Pattern](001-channel-based-subscriber-pattern.md) - Establishes channel-based API
- [ADR-002: Non-blocking Publish with Drop Policy](002-non-blocking-publish-drop-policy.md) - Defines publish semantics
- [ADR-005: Error Return for Closed Bus](005-error-return-for-closed-bus.md) - Defines Close() behavior

---

**Document Version**: 1.0
**Last Updated**: 2025-11-05
**Authors**: Ernesto (e7canasta), Gaby de Visiona (Claude Code)
