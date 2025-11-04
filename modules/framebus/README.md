# FrameBus

> Non-blocking frame distribution with intentional drop policy for real-time video processing

## Overview

FrameBus is a generic pub/sub mechanism for distributing video frames to multiple subscribers using a non-blocking fan-out pattern. It prioritizes **low latency over completeness**, intentionally dropping frames when subscribers cannot keep up.

**Core Philosophy:** _"Drop frames, never queue. Latency > Completeness."_

## Features

- ✅ **Non-blocking fan-out** - Publisher never waits for slow subscribers
- ✅ **Intentional drop policy** - Maintains real-time processing by dropping stale frames
- ✅ **Thread-safe** - Concurrent publishers and dynamic subscriber management
- ✅ **Observable** - Detailed stats (published, sent, dropped) per subscriber
- ✅ **Generic** - Channel-based API decoupled from specific worker types
- ✅ **Zero dependencies** - Pure Go standard library

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/visiona/orion/modules/framebus"
)

func main() {
    // Create bus
    bus := framebus.New()
    defer bus.Close()

    // Create subscriber channels
    worker1 := make(chan framebus.Frame, 5)
    worker2 := make(chan framebus.Frame, 5)

    // Subscribe
    bus.Subscribe("worker-1", worker1)
    bus.Subscribe("worker-2", worker2)

    // Publish frames (non-blocking)
    for i := 0; i < 100; i++ {
        frame := framebus.Frame{
            Seq:  uint64(i),
            Data: []byte(fmt.Sprintf("frame-%d", i)),
        }
        bus.Publish(frame)
    }

    // Check stats
    stats := bus.Stats()
    fmt.Printf("Published: %d, Sent: %d, Dropped: %d\n",
        stats.TotalPublished,
        stats.TotalSent,
        stats.TotalDropped,
    )
}
```

## Use Cases

### Orion Video Pipeline

FrameBus distributes frames from the stream capture to multiple inference workers:

```
Stream Capture → FrameBus ─┬→ Person Detector Worker 1
                           ├→ Person Detector Worker 2
                           └→ Pose Estimation Worker
```

### Multi-Stage Processing

```
Video Source → FrameBus ─┬→ Inference Pipeline
                         ├→ Recording Pipeline
                         └→ Monitoring/Telemetry
```

## API

### Core Interface

```go
type Bus interface {
    Subscribe(id string, ch chan<- Frame) error
    Unsubscribe(id string) error
    Publish(frame Frame)
    Stats() BusStats
    Close() error
}
```

### Stats

```go
type BusStats struct {
    TotalPublished uint64                    // Publish() calls
    TotalSent      uint64                    // Frames sent to all subscribers
    TotalDropped   uint64                    // Frames dropped (channel full)
    Subscribers    map[string]SubscriberStats
}

type SubscriberStats struct {
    Sent    uint64
    Dropped uint64
}
```

## Design Principles

### Single Responsibility

**One reason to change:** "The way frames are distributed to multiple destinations in parallel with drop policy"

### What FrameBus Does NOT Do

❌ Worker lifecycle management (Start/Stop)
❌ Automatic logging or alerting
❌ Health checking
❌ Frame processing or modification

These are responsibilities of other modules (`worker-lifecycle`, `observability`, `core`).

## Performance

- **Latency**: `Publish()` completes in microseconds (no blocking waits)
- **Memory**: Constant (fixed channel buffers, no unbounded queues)
- **Throughput**: Tested at 1000+ frames/sec with 10+ subscribers
- **Drop Rate**: Expected 70-97% for inference workloads (by design)

### Why High Drop Rates Are Normal

In a 30 FPS stream with 1 Hz inference rate:
- 30 frames published per second
- 1 frame processed per second
- **29 frames dropped per second (97% drop rate)**

This is intentional - processing the most recent frame is more valuable than queuing stale frames.

## Testing

```bash
# Run unit tests
go test ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem
```

## Documentation

- [CLAUDE.md](CLAUDE.md) - Detailed bounded context, API, anti-patterns
- [docs/adr/](docs/adr/) - Architecture Decision Records
- [examples/](examples/) - Usage examples

## Architecture Decision Records

- [ADR-001: Channel-based Subscriber Pattern](docs/adr/001-channel-based-subscriber-pattern.md)
- [ADR-002: Non-blocking Publish with Drop Policy](docs/adr/002-non-blocking-publish-drop-policy.md)
- [ADR-003: Stats Tracking Design](docs/adr/003-stats-tracking-design.md)

## License

MIT License - Visiona App

## Contributing

This module follows the [Orion Design Manifesto](../../docs/DESIGN/MANIFESTO_DISENO.md):

- **Complexity by Design** - Attack complexity through architecture, not complicated code
- **Cohesion > Location** - Modules defined by conceptual cohesion, not size
- **Pragmatism > Purism** - Deliver value, avoid over-engineering
- **KISS ≠ Simplistic** - Clean design handles inherent complexity elegantly
