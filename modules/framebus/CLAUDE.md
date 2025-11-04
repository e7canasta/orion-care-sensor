# CLAUDE.md - FrameBus Module

## Bounded Context

**FrameBus** is responsible for **non-blocking frame distribution to multiple subscribers** using a fan-out pattern with intentional drop policy.

### Single Responsibility

**One reason to change:** "The way frames are distributed to multiple destinations in parallel with drop policy"

### Ubiquitous Language

- **Publisher**: The entity that calls `Publish()` to distribute frames
- **Subscriber**: An entity registered via `Subscribe()` that receives frames through a channel
- **Non-blocking send**: Frame distribution that never waits - drops frame if subscriber channel is full
- **Drop policy**: Intentional frame dropping to maintain real-time processing
- **Fan-out**: Pattern where one input (published frame) is sent to N outputs (subscribers)

### Core Philosophy

> "Drop frames, never queue. Latency > Completeness."

FrameBus prioritizes real-time processing over guaranteed delivery. If a subscriber cannot keep up with the publish rate, frames are dropped rather than queued. This prevents unbounded latency growth and memory usage.

## What FrameBus IS

✅ A **generic pub/sub mechanism** for frame distribution
✅ A **non-blocking fan-out** implementation with drop tracking
✅ A **stats provider** for observability (publish count, sent/dropped per subscriber)
✅ **Thread-safe** for concurrent publishers and dynamic subscriber registration
✅ **Reusable** across different use cases (inference workers, encoders, loggers, etc.)

## What FrameBus IS NOT (Anti-Responsibilities)

❌ **NOT a lifecycle manager** - Does not Start/Stop subscribers
❌ **NOT an observability system** - Does not log or alert automatically
❌ **NOT coupled to workers** - Does not know about "InferenceWorker" or any specific type
❌ **NOT a health checker** - Does not monitor subscriber health
❌ **NOT a frame processor** - Does not modify or inspect frame contents

### Separation of Concerns

| Concern | Responsible Module | FrameBus Role |
|---------|-------------------|---------------|
| Worker lifecycle | `worker-lifecycle` | None - consumers manage their own lifecycle |
| Health checking | `core` or `observability` | Provides stats via `Stats()`, consumer interprets |
| Logging/Alerting | `core` or `observability` | None - consumer reads `Stats()` and decides |
| Frame processing | `stream-capture` or `workers` | None - treats frames as opaque data |

## API Surface

### Core Interface

```go
// Bus distributes frames to multiple subscribers with drop policy
type Bus interface {
    // Subscribe registers a channel to receive frames
    // Returns error if id already exists
    Subscribe(id string, ch chan<- Frame) error

    // Unsubscribe removes a subscriber by id
    // Returns error if id not found
    Unsubscribe(id string) error

    // Publish sends frame to all subscribers (non-blocking)
    // Drops frame for subscribers whose channels are full
    Publish(frame Frame)

    // Stats returns current bus statistics
    Stats() BusStats

    // Close stops the bus and prevents further operations
    Close() error
}
```

### Data Structures

```go
// BusStats contains global and per-subscriber metrics
type BusStats struct {
    TotalPublished uint64                    // Number of Publish() calls
    TotalSent      uint64                    // Sum of frames sent to all subscribers
    TotalDropped   uint64                    // Sum of frames dropped across all subscribers
    Subscribers    map[string]SubscriberStats // Per-subscriber breakdown
}

// SubscriberStats tracks metrics for a single subscriber
type SubscriberStats struct {
    Sent    uint64  // Frames successfully sent to this subscriber
    Dropped uint64  // Frames dropped due to full channel
}

// Frame is the data type distributed by the bus
type Frame struct {
    Data      []byte            // Raw frame data (JPEG, PNG, etc.)
    Seq       uint64            // Sequence number
    Timestamp time.Time         // Capture timestamp
    Metadata  map[string]string // Optional metadata
}
```

## Usage Patterns

### Basic Usage

```go
// Create bus
bus := framebus.New()
defer bus.Close()

// Subscribe workers
worker1Ch := make(chan framebus.Frame, 5)
bus.Subscribe("worker-1", worker1Ch)

worker2Ch := make(chan framebus.Frame, 5)
bus.Subscribe("worker-2", worker2Ch)

// Publish frames (non-blocking)
for frame := range streamSource {
    bus.Publish(frame)
}

// Check stats
stats := bus.Stats()
fmt.Printf("Published: %d, Sent: %d, Dropped: %d\n",
    stats.TotalPublished, stats.TotalSent, stats.TotalDropped)
```

### Integration with Orion Core

```go
// Core orchestrates the connection between bus and workers
frameBus := framebus.New()

// Worker Lifecycle creates workers and subscribes to bus
for _, worker := range workers {
    frameCh := make(chan framebus.Frame, 5)
    frameBus.Subscribe(worker.ID(), frameCh)

    // Worker lifecycle manages worker goroutine
    go worker.ProcessFrames(frameCh)
}

// consumeFrames goroutine publishes to bus
for frame := range stream.Frames() {
    processedFrame := roiProcessor.Process(frame)
    frameBus.Publish(processedFrame)
}

// Observability reads stats periodically
ticker := time.NewTicker(10 * time.Second)
for range ticker.C {
    stats := frameBus.Stats()
    logBusHealth(stats)
}
```

### Dynamic Subscriber Management

```go
// Add subscriber at runtime
newWorkerCh := make(chan framebus.Frame, 5)
bus.Subscribe("worker-3", newWorkerCh)

// Remove subscriber
bus.Unsubscribe("worker-1")
```

## Thread Safety Model

### Concurrency Guarantees

- ✅ **Multiple publishers**: `Publish()` can be called concurrently from multiple goroutines
- ✅ **Dynamic subscribers**: `Subscribe()/Unsubscribe()` can be called while publishing
- ✅ **Stats reading**: `Stats()` can be called concurrently with all operations

### Internal Synchronization

```
┌─────────────────────────────────────────┐
│ Publish() goroutines (N concurrent)     │
│ - RLock for reading subscriber map      │
│ - Atomic increments for stats           │
└─────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────┐
│ Subscribe/Unsubscribe (exclusive)       │
│ - Lock for modifying subscriber map     │
└─────────────────────────────────────────┘
                  │
                  ↓
┌─────────────────────────────────────────┐
│ Stats() (concurrent reads)              │
│ - RLock for reading subscriber map      │
│ - Atomic loads for counters             │
└─────────────────────────────────────────┘
```

## Performance Characteristics

| Characteristic | Implementation | Benefit |
|---------------|----------------|---------|
| **Low Latency** | Non-blocking sends | Publisher never waits for slow subscribers |
| **Real-time Processing** | Drop policy | Always processing recent frames, not stale backlog |
| **Constant Memory** | Fixed channel buffers | No unbounded queue growth |
| **Subscriber Isolation** | Independent channels | One slow subscriber doesn't affect others |
| **Observable** | Per-subscriber stats | Easy to identify bottlenecks |

### Channel Buffer Sizing Recommendations

| Use Case | Recommended Buffer | Rationale |
|----------|-------------------|-----------|
| Fast inference workers | 2-5 frames | Small buffer allows brief processing variations |
| Slow processing (encoding) | 10-20 frames | Larger buffer smooths bursty processing |
| Logging/telemetry | 50-100 frames | Can tolerate more latency, prioritize completeness |

**Note**: Buffer size is controlled by the **subscriber** when creating the channel, not by FrameBus.

## Anti-Patterns & Alternatives

### ❌ Anti-Pattern: Using FrameBus to manage worker lifecycle

```go
// DON'T: FrameBus starting workers
bus.Start(ctx)  // ❌ This doesn't exist

// DO: Core orchestrates lifecycle
for _, worker := range workers {
    worker.Start(ctx)  // Worker lifecycle manages this
    frameCh := make(chan framebus.Frame, 5)
    bus.Subscribe(worker.ID(), frameCh)
}
```

### ❌ Anti-Pattern: Expecting guaranteed delivery

```go
// DON'T: Assume all frames are delivered
bus.Publish(criticalFrame)
// ❌ Frame might be dropped if subscriber channel is full

// DO: Use separate mechanism for critical frames
if isCritical(frame) {
    sendViaPersistentQueue(frame)  // Different system
} else {
    bus.Publish(frame)  // Best-effort delivery
}
```

### ❌ Anti-Pattern: Blocking operations in subscriber

```go
// DON'T: Slow processing without buffering
for frame := range subscriberCh {  // Buffer: 1
    slowInference(frame)  // 100ms processing
    // ❌ High drop rate because channel fills quickly
}

// DO: Use adequate buffer or pipeline pattern
subscriberCh := make(chan framebus.Frame, 10)  // Larger buffer
for frame := range subscriberCh {
    slowInference(frame)
}
```

### ❌ Anti-Pattern: Interpreting stats inside FrameBus

```go
// DON'T: FrameBus making decisions based on stats
func (b *bus) Publish(frame Frame) {
    stats := b.Stats()
    if stats.TotalDropped > 1000 {
        log.Warn("high drops!")  // ❌ Not FrameBus responsibility
    }
}

// DO: Consumer interprets stats
stats := bus.Stats()
if stats.TotalDropped > threshold {
    alerting.Warn("framebus high drops", stats)
}
```

## Testing Strategy

### Unit Tests

- ✅ Non-blocking behavior (Publish doesn't block on full channels)
- ✅ Stats accuracy (counts match actual sent/dropped)
- ✅ Thread safety (concurrent Publish + Subscribe/Unsubscribe)
- ✅ Subscribe/Unsubscribe edge cases (duplicate IDs, unknown IDs)

### Integration Tests

- ✅ Multiple subscribers receiving same frames
- ✅ Dynamic subscriber add/remove during publishing
- ✅ High load scenarios (1000+ frames/sec)

### Property-Based Tests

- ✅ `TotalSent + TotalDropped == TotalPublished * len(Subscribers)`
- ✅ Stats are monotonically increasing
- ✅ No frame is sent to unsubscribed channels

## Migration from Prototype

### Changes from `internal/framebus` (Prototype)

| Prototype | Orion 2.0 | Reason |
|-----------|-----------|--------|
| `Register(InferenceWorker)` | `Subscribe(id, chan)` | Decouple from worker types |
| `Start()/Stop()` methods | ❌ Removed | Not FrameBus responsibility |
| `StartStatsLogger()` | ❌ Removed | Consumer reads `Stats()` |
| `worker.SendFrame()` | Direct channel send | Non-blocking logic in Bus, not worker |
| Drops tracked by error return | Drops tracked internally | Cleaner separation |

### Migration Guide

```go
// OLD (Prototype)
frameBus := framebus.New()
frameBus.Register(worker)
frameBus.Start(ctx)
frameBus.StartStatsLogger(ctx, 10*time.Second)

// NEW (Orion 2.0)
frameBus := framebus.New()
workerCh := make(chan framebus.Frame, 5)
frameBus.Subscribe(worker.ID(), workerCh)

// Core manages worker lifecycle
worker.Start(ctx)

// Core manages observability
go logStatsLoop(frameBus, 10*time.Second)
```

## Architecture Decision Records

See `docs/adr/` for detailed rationale:

- [ADR-001: Channel-based Subscriber Pattern](docs/adr/001-channel-based-subscriber-pattern.md)
- [ADR-002: Non-blocking Publish with Drop Policy](docs/adr/002-non-blocking-publish-drop-policy.md)
- [ADR-003: Stats Tracking Design](docs/adr/003-stats-tracking-design.md)

## Module Structure (Updated 2025-11-04)

FrameBus follows Go's `internal/` package convention for clear API boundaries:

```
framebus/
├── framebus.go         # Public API (type aliases to internal types)
├── helpers.go          # Public utility functions
├── helpers_test.go     # Helper function tests
├── doc.go              # Package documentation
└── internal/
    └── bus/
        ├── bus.go      # Implementation (encapsulated)
        └── bus_test.go # Implementation tests
```

**Why internal/?**
- ✅ Compiler-enforced bounded context
- ✅ Prevents coupling to implementation details
- ✅ Enables evolution without breaking changes
- ✅ Clear separation: API contract vs implementation

**For consumers:** No changes needed. All examples in this doc work as-is.
**For contributors:** Implementation in `internal/bus/`, public contract in `framebus.go`.

## References

- Prototype implementation: `/References/orion-prototipe/internal/framebus/bus.go`
- Wiki documentation: `/VAULT/wiki/2.4-frame-distribution.md`
- Refactor summary: `INTERNAL_REFACTOR_SUMMARY.md`
- Quick wins summary: `QUICK_WINS_SUMMARY.md`

* [[modules/framebus/README]]

