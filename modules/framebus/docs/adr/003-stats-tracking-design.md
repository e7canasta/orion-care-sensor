# ADR-003: Stats Tracking Design

## Status

**ACCEPTED** - 2025-11-04

## Context

FrameBus needs to expose metrics for observability, health checking, and debugging. The prototype implementation (`References/orion-prototipe/internal/framebus/bus.go`) had:

1. **Basic stats tracking** (frames distributed, dropped per worker)
2. **Automatic logging** via `StartStatsLogger()` method
3. **Built-in alerting** (warned if drop rate > 80%)

This mixed responsibilities:
- Stats **collection** (legitimate concern of FrameBus)
- Stats **interpretation** (should be consumer's responsibility)
- Automatic **logging/alerting** (observability concern, not distribution)

## Decision

**FrameBus will track comprehensive stats but will NOT log or alert automatically.**

### Stats API

```go
type BusStats struct {
    // Global metrics
    TotalPublished uint64  // Number of Publish() calls
    TotalSent      uint64  // Sum of frames sent to all subscribers
    TotalDropped   uint64  // Sum of frames dropped across all subscribers

    // Per-subscriber breakdown
    Subscribers map[string]SubscriberStats
}

type SubscriberStats struct {
    Sent    uint64  // Frames successfully sent
    Dropped uint64  // Frames dropped (channel full)
}

// Stats returns current statistics snapshot
func (b *Bus) Stats() BusStats
```

### Key Characteristics

1. **Comprehensive**: Global + per-subscriber metrics
2. **Read-only**: Returns snapshot, doesn't modify state
3. **Thread-safe**: Can be called concurrently with Publish
4. **No side effects**: No logging, no alerting, just data

## Rationale

### Separation of Concerns

| Concern | Responsible Module | FrameBus Role |
|---------|-------------------|---------------|
| **Stats collection** | FrameBus | ✅ Tracks TotalPublished, Sent, Dropped |
| **Stats interpretation** | Core/Observability | ❌ Consumers calculate drop rates, trends |
| **Logging** | Core/Observability | ❌ Consumers log when/how they want |
| **Alerting** | Core/Observability | ❌ Consumers define alert thresholds |

### Cohesion

**Single Responsibility Test:** "Does this code have one reason to change?"

❌ **Prototype (3 reasons to change):**
1. Distribution algorithm changes → Rewrite Distribute()
2. Logging format changes → Rewrite StartStatsLogger()
3. Alert thresholds change → Rewrite warning logic

✅ **Orion 2.0 (1 reason to change):**
1. Distribution algorithm changes → Update Stats() if needed

### Global Stats Value

The addition of `TotalPublished` enables consumers to:

1. **Detect no-subscriber scenario:**
   ```go
   if stats.TotalPublished > 0 && stats.TotalSent == 0 {
       log.Warn("framebus publishing but no subscribers")
   }
   ```

2. **Calculate global drop rate:**
   ```go
   totalAttempts := stats.TotalSent + stats.TotalDropped
   globalDropRate := float64(stats.TotalDropped) / float64(totalAttempts)
   ```

3. **Verify invariants:**
   ```go
   // Property: TotalSent + TotalDropped == TotalPublished × len(Subscribers)
   expected := stats.TotalPublished * uint64(len(stats.Subscribers))
   actual := stats.TotalSent + stats.TotalDropped
   assert.Equal(t, expected, actual)
   ```

4. **Monitor throughput:**
   ```go
   deltaPublished := currentStats.TotalPublished - prevStats.TotalPublished
   throughput := float64(deltaPublished) / intervalSeconds
   log.Info("framebus throughput", "fps", throughput)
   ```

## Consequences

### Positive

✅ **Cohesion**: Stats collection is natural FrameBus responsibility
✅ **Flexibility**: Consumers interpret stats as needed (log, alert, dashboard)
✅ **Testability**: Easy to verify stats accuracy without mocking logging
✅ **Reusability**: Same Bus works with different observability strategies
✅ **Observability**: Comprehensive metrics for debugging and monitoring

### Negative

⚠ **Consumers must implement observability**: No built-in logging
⚠ **Potential inconsistency**: Different consumers might log differently

### Mitigations

1. **Documentation**: Examples in `CLAUDE.md` show common observability patterns
2. **Core module responsibility**: Orion Core provides reference implementation
3. **Stats are simple**: Easy for any consumer to use correctly

## Implementation Details

### Thread Safety with Atomics

To avoid locking in the hot path (`Publish`), use atomic operations:

```go
type bus struct {
    mu          sync.RWMutex
    subscribers map[string]chan<- Frame

    // Global counter (atomic - no lock needed in Publish)
    totalPublished atomic.Uint64

    // Per-subscriber stats (protected by mu, but atomic increments)
    stats map[string]*subscriberStats
}

type subscriberStats struct {
    sent    atomic.Uint64
    dropped atomic.Uint64
}

func (b *bus) Publish(frame Frame) {
    b.totalPublished.Add(1)  // ✅ Atomic, no lock

    b.mu.RLock()
    defer b.mu.RUnlock()

    for id, ch := range b.subscribers {
        select {
        case ch <- frame:
            b.stats[id].sent.Add(1)     // ✅ Atomic, no lock
        default:
            b.stats[id].dropped.Add(1)  // ✅ Atomic, no lock
        }
    }
}

func (b *bus) Stats() BusStats {
    b.mu.RLock()  // ✅ Only RLock, Publish can run concurrently
    defer b.mu.RUnlock()

    result := BusStats{
        TotalPublished: b.totalPublished.Load(),
        Subscribers:    make(map[string]SubscriberStats),
    }

    var totalSent, totalDropped uint64
    for id, stats := range b.stats {
        sent := stats.sent.Load()
        dropped := stats.dropped.Load()

        totalSent += sent
        totalDropped += dropped

        result.Subscribers[id] = SubscriberStats{
            Sent:    sent,
            Dropped: dropped,
        }
    }

    result.TotalSent = totalSent
    result.TotalDropped = totalDropped

    return result
}
```

### Performance Characteristics

| Operation | Lock Type | Contention | Performance |
|-----------|-----------|------------|-------------|
| `Publish()` | RLock + Atomic | Low (read-only map access) | ~100ns per subscriber |
| `Subscribe()` | Lock | Rare (infrequent calls) | Microseconds |
| `Stats()` | RLock | Low (read-only) | ~1-10µs depending on subscriber count |

**Design goal**: `Publish()` should complete in microseconds even with 100+ subscribers.

### Invariants

The Stats API enforces these invariants:

1. **Conservation law:**
   ```
   TotalSent + TotalDropped == TotalPublished × len(Subscribers)
   ```

2. **Monotonicity:**
   ```
   For any two snapshots s1 (earlier), s2 (later):
   s2.TotalPublished >= s1.TotalPublished
   s2.TotalSent >= s1.TotalSent
   s2.TotalDropped >= s1.TotalDropped
   ```

3. **Per-subscriber totals:**
   ```
   TotalSent == Σ(subscriber.Sent)
   TotalDropped == Σ(subscriber.Dropped)
   ```

These invariants should be verified in property-based tests.

## Consumer Usage Examples

### Example 1: Periodic Logging (Core Module)

```go
func logStatsLoop(bus framebus.Bus, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    prevStats := bus.Stats()

    for range ticker.C {
        stats := bus.Stats()

        // Calculate deltas
        deltaPublished := stats.TotalPublished - prevStats.TotalPublished
        deltaSent := stats.TotalSent - prevStats.TotalSent
        deltaDropped := stats.TotalDropped - prevStats.TotalDropped

        // Log summary
        slog.Info("framebus stats",
            "published", stats.TotalPublished,
            "sent", stats.TotalSent,
            "dropped", stats.TotalDropped,
            "throughput_fps", float64(deltaPublished)/interval.Seconds(),
        )

        // Log per-subscriber details
        for id, sub := range stats.Subscribers {
            prevSub := prevStats.Subscribers[id]
            deltaSent := sub.Sent - prevSub.Sent
            deltaDropped := sub.Dropped - prevSub.Dropped

            if deltaSent+deltaDropped > 0 {
                dropRate := float64(deltaDropped) / float64(deltaSent+deltaDropped)
                slog.Debug("subscriber stats",
                    "id", id,
                    "sent", sub.Sent,
                    "dropped", sub.Dropped,
                    "drop_rate", dropRate,
                )
            }
        }

        prevStats = stats
    }
}
```

### Example 2: Health Endpoint (HTTP API)

```go
func healthHandler(bus framebus.Bus) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        stats := bus.Stats()

        health := struct {
            Status      string                          `json:"status"`
            Published   uint64                          `json:"total_published"`
            Sent        uint64                          `json:"total_sent"`
            Dropped     uint64                          `json:"total_dropped"`
            GlobalDrop  float64                         `json:"global_drop_rate"`
            Subscribers map[string]subscriberHealth    `json:"subscribers"`
        }{
            Status:    "ok",
            Published: stats.TotalPublished,
            Sent:      stats.TotalSent,
            Dropped:   stats.TotalDropped,
        }

        // Calculate global drop rate
        if total := stats.TotalSent + stats.TotalDropped; total > 0 {
            health.GlobalDrop = float64(stats.TotalDropped) / float64(total)
        }

        // Per-subscriber health
        health.Subscribers = make(map[string]subscriberHealth)
        for id, sub := range stats.Subscribers {
            sh := subscriberHealth{
                Sent:    sub.Sent,
                Dropped: sub.Dropped,
            }
            if total := sub.Sent + sub.Dropped; total > 0 {
                sh.DropRate = float64(sub.Dropped) / float64(total)
            }
            health.Subscribers[id] = sh
        }

        json.NewEncoder(w).Encode(health)
    }
}
```

### Example 3: Alerting (Observability Module)

```go
func checkBusHealth(bus framebus.Bus) []Alert {
    stats := bus.Stats()
    var alerts []Alert

    // Alert: Publishing but no subscribers
    if stats.TotalPublished > 100 && len(stats.Subscribers) == 0 {
        alerts = append(alerts, Alert{
            Severity: "warning",
            Message:  "FrameBus publishing but no subscribers registered",
        })
    }

    // Alert: Unusually low drop rate (might indicate hung subscriber)
    for id, sub := range stats.Subscribers {
        if sub.Sent > 0 {
            dropRate := float64(sub.Dropped) / float64(sub.Sent+sub.Dropped)
            if dropRate < 0.50 {  // Expected 70-97%
                alerts = append(alerts, Alert{
                    Severity: "warning",
                    Message:  fmt.Sprintf("Subscriber %s has low drop rate %.2f (expected 0.70-0.97)", id, dropRate),
                })
            }
        }
    }

    return alerts
}
```

## Trade-offs Considered

### Alternative 1: Keep Automatic Logging (Prototype)

```go
func (b *Bus) StartStatsLogger(ctx context.Context, interval time.Duration)
```

**Rejected because:**
- Violates single responsibility (stats collection + logging)
- Forces all consumers to use same logging strategy
- Makes testing harder (must verify log output)
- Reduces flexibility (what if consumer wants Prometheus metrics instead?)

### Alternative 2: Callback-based Stats

```go
type StatsCallback func(BusStats)
func (b *Bus) OnStats(interval time.Duration, callback StatsCallback)
```

**Rejected because:**
- Adds complexity (goroutine lifecycle, callback management)
- Consumer can easily implement this with `Stats()` + ticker
- Doesn't add value over pull-based approach

### Alternative 3: Separate Stats Object

```go
type StatsCollector interface {
    RecordPublish()
    RecordSent(id string)
    RecordDropped(id string)
}

func New(collector StatsCollector) *Bus
```

**Rejected because:**
- Over-engineering for the problem
- Stats are intrinsic to FrameBus behavior (not pluggable concern)
- Adds interface complexity without benefit

### Alternative 4: Minimal Stats (No Global Counters)

```go
type BusStats struct {
    Subscribers map[string]SubscriberStats  // Only per-subscriber
}
```

**Rejected because:**
- Consumers would have to sum subscribers to get totals (inefficient)
- Can't detect "no subscribers" scenario
- Loses throughput measurement (TotalPublished)

## Related Decisions

- [ADR-001: Channel-based Subscriber Pattern](001-channel-based-subscriber-pattern.md)
- [ADR-002: Non-blocking Publish with Drop Policy](002-non-blocking-publish-drop-policy.md)

## References

- Prototype: `References/orion-prototipe/internal/framebus/bus.go` (lines 100-212)
- Design discussion: `Daily.md` (Stats of publications suggestion)
- Orion Manifesto: "Complejidad por diseño" - Stats collection is intentional, logging is not FrameBus's concern
