# FrameBus

> Non-blocking frame distribution with intentional drop policy for real-time video processing

## Overview

FrameBus is a generic pub/sub mechanism for distributing video frames to multiple subscribers using a non-blocking fan-out pattern. It prioritizes **low latency over completeness**, intentionally dropping frames when subscribers cannot keep up.

**Core Philosophy:** _"Drop frames, never queue. Latency > Completeness."_

## Features

- ✅ **Non-blocking fan-out** - Publisher never waits for slow subscribers
- ✅ **Priority-based load shedding** - Protect critical subscribers under load
- ✅ **Intentional drop policy** - Maintains real-time processing by dropping stale frames
- ✅ **Thread-safe** - Concurrent publishers and dynamic subscriber management
- ✅ **Observable** - Detailed stats (published, sent, dropped) per subscriber with priority
- ✅ **Generic** - Channel-based API decoupled from specific worker types
- ✅ **Zero dependencies** - Pure Go standard library

## Quick Start

### Basic Usage (Default Priority)

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

    // Subscribe (default priority: Normal)
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

### Priority Subscribers (SLA Protection)

```go
package main

import (
    "fmt"
    "github.com/visiona/orion/modules/framebus"
)

func main() {
    bus := framebus.New()
    defer bus.Close()

    // Critical: Person detection (foundation for fall detection downstream in Sala)
    personDetectorCh := make(chan framebus.Frame, 10)
    bus.SubscribeWithPriority("person-detector-worker", personDetectorCh, framebus.PriorityCritical)

    // High: Pose estimation (important for edge-of-bed analysis in Sala)
    poseWorkerCh := make(chan framebus.Frame, 10)
    bus.SubscribeWithPriority("pose-worker", poseWorkerCh, framebus.PriorityHigh)

    // Normal: Optical flow (micro-awakening detection, quality-of-life)
    flowWorkerCh := make(chan framebus.Frame, 5)
    bus.Subscribe("flow-worker", flowWorkerCh) // Default: Normal

    // BestEffort: Experimental VLM (research, no production SLA)
    vlmWorkerCh := make(chan framebus.Frame, 5)
    bus.SubscribeWithPriority("vlm-experimental-worker", vlmWorkerCh, framebus.PriorityBestEffort)

    // Publish frames
    for i := 0; i < 100; i++ {
        bus.Publish(framebus.Frame{Seq: uint64(i)})
    }

    // Check priority-aware stats
    stats := bus.Stats()
    for id, sub := range stats.Subscribers {
        dropRate := float64(sub.Dropped) / float64(sub.Sent + sub.Dropped) * 100
        fmt.Printf("%s (Priority %d): %.1f%% drop rate\n", id, sub.Priority, dropRate)
    }
    
    // Expected output under load:
    // person-detector-worker (Priority 0):     0.0% drop rate  ← Protected (Critical)
    // pose-worker (Priority 1):                5.0% drop rate  ← Minimal drops (High)
    // flow-worker (Priority 2):               40.0% drop rate  ← Acceptable (Normal)
    // vlm-experimental-worker (Priority 3):   90.0% drop rate  ← Shed first (BestEffort)
    
    // Downstream Impact (Sala Experts via MQTT):
    // - EdgeExpert gets reliable person detection + pose data → Fall risk analysis works
    // - SleepExpert gets degraded flow data → Sleep quality insights reduced (tolerable)
    // - Research pipeline gets minimal VLM data → Expected, no production impact
}
```

## Architecture

### Fan-out Pattern

```
                ┌──────────────┐
                │  Publisher   │
                │ (30 FPS cam) │
                └──────┬───────┘
                       │ Publish(frame)
                       ↓
                ┌──────────────┐
                │  FrameBus    │ (non-blocking fan-out)
                │              │
                └──┬───┬───┬───┘
                   │   │   │
        ┌──────────┘   │   └──────────┐
        ↓              ↓              ↓
  ┌──────────┐  ┌──────────┐  ┌──────────┐
  │ Worker 1 │  │ Worker 2 │  │ Worker 3 │
  │  (fast)  │  │  (slow)  │  │  (fast)  │
  │  5ms/fr  │  │ 50ms/fr  │  │  5ms/fr  │
  └──────────┘  └──────────┘  └──────────┘
   Drops: 0%     Drops: 97%    Drops: 0%
```

## Use Cases

### Orion Video Pipeline (with Priority SLAs)

FrameBus distributes frames from stream capture to **Orion Workers** with differentiated SLAs:

```
Stream-Capture → FrameBus ─┬→ PersonDetectorWorker (Critical) → MQTT → EdgeExpert (Sala)
                           ├→ PoseWorker (High)              → MQTT → EdgeExpert, SleepExpert
                           ├→ FlowWorker (Normal)            → MQTT → SleepExpert
                           └→ VLMExperimentalWorker (BestEffort) → Research Pipeline
```

**Under Load** (FrameBus saturated):
- PersonDetectorWorker: 0% drops (protected) → EdgeExpert gets reliable data
- PoseWorker: <10% drops (minimal degradation) → EdgeExpert can still analyze fall risk
- FlowWorker: <50% drops (acceptable) → SleepExpert insights reduced (tolerable)
- VLMExperimentalWorker: 90%+ drops (expected, no SLA) → Research continues best-effort

**Key Insight**: Priority in FrameBus protects the **entire inference chain**:
```
PersonDetectorWorker (0% drops, FrameBus protected)
  → MQTT inference
    → EdgeExpert (Sala, reliable fall detection)
      → Care Staff Alert (life saved)
```

See [FRAMEBUS_CUSTOMERS.md](docs/FRAMEBUS_CUSTOMERS.md) for detailed customer context.

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
    // Subscribe with default priority (Normal)
    Subscribe(id string, ch chan<- Frame) error
    
    // Subscribe with custom priority
    SubscribeWithPriority(id string, ch chan<- Frame, priority SubscriberPriority) error
    
    Unsubscribe(id string) error
    Publish(frame Frame)
    PublishWithContext(ctx context.Context, frame Frame)
    Stats() BusStats
    GetHealth(id string) SubscriberHealth
    GetUnhealthySubscribers() []string
    Close() error
}

// Priority levels (lower = higher priority)
const (
    PriorityCritical   SubscriberPriority = 0  // Mission-critical (fall detection)
    PriorityHigh       SubscriberPriority = 1  // Important (sleep monitoring)
    PriorityNormal     SubscriberPriority = 2  // Standard (default)
    PriorityBestEffort SubscriberPriority = 3  // Experimental (no SLA)
)
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
    Sent     uint64
    Dropped  uint64
    Priority SubscriberPriority  // Priority level of subscriber
}

// Helper functions
func CalculateDropRate(stats BusStats) float64
func CalculateSubscriberDropRate(stats BusStats, subscriberID string) float64
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

**Helper functions for drop rate calculation:**
```go
stats := bus.Stats()

// Global drop rate (0.0 to 1.0)
globalRate := framebus.CalculateDropRate(stats)
fmt.Printf("Drop rate: %.2f%%\n", globalRate*100)

// Per-subscriber drop rate
workerRate := framebus.CalculateSubscriberDropRate(stats, "worker-1")
fmt.Printf("Worker drop rate: %.2f%%\n", workerRate*100)
```

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
- [ADR-009: Priority-Based Load Shedding](docs/adr/ADR-009-Priority-Subscribers.md) ⭐ **NEW**

## Customer Context

**Who uses FrameBus?** [Orion Workers](docs/FRAMEBUS_CUSTOMERS.md) - AI inference workers (PersonDetector, Pose, Flow, VLM) internal to Orion.

**Why Priority matters?** Protects mission-critical workers (PersonDetectorWorker → fall detection chain) while enabling experimental features (VLM research) on shared hardware.

**Downstream consumers**: Sala Experts (EdgeExpert, SleepExpert) consume worker inferences via MQTT, outside Orion boundary.

## License

MIT License - Visiona App

## Contributing

This module follows the [Orion Design Manifesto](../../docs/DESIGN/MANIFESTO_DISENO.md):

- **Complexity by Design** - Attack complexity through architecture, not complicated code
- **Cohesion > Location** - Modules defined by conceptual cohesion, not size
- **Pragmatism > Purism** - Deliver value, avoid over-engineering
- **KISS ≠ Simplistic** - Clean design handles inherent complexity elegantly
