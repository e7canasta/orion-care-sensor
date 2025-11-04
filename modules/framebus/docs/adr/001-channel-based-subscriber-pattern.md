# ADR-001: Channel-based Subscriber Pattern

## Status

**ACCEPTED** - 2025-11-04

## Context

The FrameBus module needs to distribute frames to multiple consumers (inference workers, encoders, loggers, etc.). The prototype implementation (`References/orion-prototipe/internal/framebus/bus.go`) used an interface-based approach:

```go
// Prototype approach
type Bus struct {
    workers []types.InferenceWorker
}

func (b *Bus) Register(worker types.InferenceWorker) { ... }
```

This created tight coupling:
- FrameBus knew about `InferenceWorker` interface (8 methods: ID, Start, Stop, SendFrame, Results, Metrics, etc.)
- FrameBus was responsible for worker lifecycle (`Start()`, `Stop()`)
- Adding new consumer types required interface implementation
- Testing required mock implementations of the full interface

## Decision

**We will use a channel-based subscriber pattern instead of an interface-based approach.**

### API Design

```go
type Bus interface {
    Subscribe(id string, ch chan<- Frame) error
    Unsubscribe(id string) error
    Publish(frame Frame)
    Stats() BusStats
    Close() error
}
```

### Key Changes

1. **Subscribers are channels, not interfaces**
   - `Subscribe(id, chan)` instead of `Register(InferenceWorker)`
   - Consumers create their own channels with appropriate buffer sizes
   - FrameBus only knows how to send to a channel

2. **No lifecycle management**
   - Removed `Start()` and `Stop()` methods from Bus
   - Consumers manage their own lifecycle
   - FrameBus only manages subscription state

3. **Generic subscribers**
   - Any consumer can subscribe (workers, encoders, loggers)
   - No need to implement interfaces
   - Decoupled from domain concepts like "inference"

## Rationale

### Alignment with Go Idioms

Go's philosophy: _"Share memory by communicating, don't communicate by sharing memory"_

Channels are the idiomatic Go mechanism for:
- Decoupling producers from consumers
- Thread-safe communication without explicit locks
- Natural backpressure (buffered channels)

### Simplicity

**Interface-based (Prototype):**
```go
type InferenceWorker interface {
    ID() string
    Start(ctx context.Context) error
    SendFrame(frame Frame) error
    Results() <-chan Inference
    Stop() error
    Metrics() WorkerMetrics
}

// Every consumer must implement 6 methods
// FrameBus must handle lifecycle, metrics, results
```

**Channel-based (Orion 2.0):**
```go
// Consumer creates channel
ch := make(chan framebus.Frame, 5)

// Consumer subscribes
bus.Subscribe("my-id", ch)

// Consumer reads
for frame := range ch {
    process(frame)
}
```

The channel-based approach has:
- **1 method call** to subscribe vs implementing 6-method interface
- **Zero coupling** to FrameBus internals
- **Natural cleanup** via channel close

### Testability

**Interface-based testing:**
```go
// Must create mock with 6 methods
type mockWorker struct {
    id string
    frameCh chan Frame
    // ... implement all 6 interface methods
}

func (m *mockWorker) ID() string { return m.id }
func (m *mockWorker) Start(ctx) error { ... }
func (m *mockWorker) SendFrame(f Frame) error { ... }
// ... 3 more methods
```

**Channel-based testing:**
```go
// Just create a channel
testCh := make(chan framebus.Frame, 1)
bus.Subscribe("test", testCh)

// Verify
bus.Publish(frame)
received := <-testCh
assert.Equal(t, frame, received)
```

### Flexibility

The channel-based approach enables use cases that interfaces would restrict:

```go
// Use case 1: Multiple consumers with same channel
sharedCh := make(chan Frame, 10)
bus.Subscribe("worker-1", sharedCh)
bus.Subscribe("worker-2", sharedCh)  // ❌ Would fail with interface approach

// Use case 2: Dynamic buffer sizing
slowConsumer := make(chan Frame, 100)   // Large buffer
fastConsumer := make(chan Frame, 5)     // Small buffer

// Use case 3: Testing with select
select {
case frame := <-testCh:
    // Verify frame
case <-time.After(1 * time.Second):
    t.Fatal("timeout waiting for frame")
}
```

## Consequences

### Positive

✅ **Decoupling**: FrameBus has zero knowledge of consumer types
✅ **Simplicity**: No interface to implement, just create channel
✅ **Testability**: Trivial to test with real channels
✅ **Flexibility**: Consumers control buffer size, select patterns
✅ **Performance**: Direct channel send is faster than interface method call
✅ **Reusability**: Same Bus can serve workers, encoders, loggers, etc.

### Negative

⚠ **No compile-time enforcement**: Consumers must remember to read from channel
⚠ **Resource cleanup**: Consumers responsible for closing channels
⚠ **Discovery**: No way to query "what subscribers exist" beyond stats

### Mitigations

1. **Documentation**: `CLAUDE.md` includes anti-patterns and examples
2. **Testing**: Property-based tests ensure channel semantics work correctly
3. **Stats API**: `Stats()` exposes subscriber IDs and metrics for observability

## Trade-offs Considered

### Alternative 1: Keep Interface but Simplify

```go
type Subscriber interface {
    ID() string
    SendFrame(frame Frame) error
}
```

**Rejected because:**
- Still couples Bus to an interface concept
- Consumers must implement `SendFrame()` with non-blocking logic
- Less idiomatic than channels in Go

### Alternative 2: Callback Functions

```go
Subscribe(id string, callback func(Frame)) error
```

**Rejected because:**
- Goroutine management becomes unclear (who spawns?)
- No natural backpressure mechanism
- Error handling is awkward (panic in callback?)
- Channel semantics (select, timeout) lost

### Alternative 3: Hybrid (Interface + Channel)

```go
type Subscriber interface {
    ID() string
    FrameChannel() chan<- Frame
}
```

**Rejected because:**
- Unnecessary indirection - just pass the channel
- Forces consumers to expose channel as method
- Doesn't add value over direct channel passing

## Implementation Notes

### Non-blocking Send Pattern

FrameBus owns the non-blocking send logic:

```go
func (b *bus) Publish(frame Frame) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    for id, ch := range b.subscribers {
        select {
        case ch <- frame:
            b.stats[id].sent.Add(1)
        default:
            b.stats[id].dropped.Add(1)
        }
    }
}
```

This is **superior to the prototype** where workers implemented `SendFrame()`:
- Prototype: Each worker duplicates non-blocking logic
- Orion 2.0: Centralized in one place, consistent across all subscribers

## References

- Prototype implementation: `References/orion-prototipe/internal/framebus/bus.go` (lines 14, 30, 44)
- Prototype types: `References/orion-prototipe/internal/types/worker.go` (lines 18-31)
- Design session: `Daily.md` (Sesión de Café - FrameBus Design)
- Go philosophy: [Effective Go - Share by Communicating](https://go.dev/doc/effective_go#sharing)

## Related Decisions

- [ADR-002: Non-blocking Publish with Drop Policy](002-non-blocking-publish-drop-policy.md)
- [ADR-003: Stats Tracking Design](003-stats-tracking-design.md)
