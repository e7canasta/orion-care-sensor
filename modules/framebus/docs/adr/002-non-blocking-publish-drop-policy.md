# ADR-002: Non-blocking Publish with Drop Policy

## Status

**ACCEPTED** - 2025-11-04

## Context

Real-time video processing systems face a fundamental trade-off:

**Completeness vs Latency**
- Process every frame → Latency grows unbounded when consumers are slow
- Drop frames → Maintain low latency but lose some data

In video surveillance for geriatric care, **processing the most recent frame** is more valuable than processing a backlog of stale frames. A 5-second-old frame showing a patient fall is less actionable than the current state.

### Queuing Approach (Alternative)

```
Stream (30 FPS) → Queue (unbounded) → Slow Worker (1 FPS)
```

**Problems:**
- Queue grows: 30 frames/sec in, 1 frame/sec out = +29 frames/sec
- Latency increases: After 1 minute, processing 60-second-old frames
- Memory grows: 1740 queued frames after 1 minute
- System falls further behind, never catches up

### Drop Approach (Orion Design)

```
Stream (30 FPS) → FrameBus (drop policy) → Slow Worker (1 FPS)
```

**Benefits:**
- Constant memory: Fixed channel buffers
- Constant latency: Always processing recent frames
- Predictable behavior: Worker processes at its own rate

## Decision

**FrameBus will use a non-blocking publish with intentional drop policy.**

### Implementation

```go
func (b *bus) Publish(frame Frame) {
    b.mu.RLock()
    defer b.mu.RUnlock()

    b.totalPublished.Add(1)

    for id, ch := range b.subscribers {
        select {
        case ch <- frame:
            // Frame sent successfully
            b.stats[id].sent.Add(1)
        default:
            // Channel full - drop frame
            b.stats[id].dropped.Add(1)
        }
    }
}
```

### Key Characteristics

1. **Non-blocking**: `Publish()` never waits for subscribers
2. **Immediate drop**: No retry, no queuing
3. **Per-subscriber decision**: One slow subscriber doesn't block others
4. **Observable**: Drops are tracked and exposed via `Stats()`

## Rationale

### Real-time Processing Philosophy

> "In real-time systems, no data is better than late data."

For Orion's use case (patient monitoring):
- **Recent frame (dropped 29 frames)**: Shows patient's current state
- **Complete history (queued 1740 frames)**: Processing 60 seconds in the past, useless for alerts

### Bounded Latency

**Design Goal:** < 2 second end-to-end latency (capture → inference → alert)

**Latency budget:**
- Stream capture: 33ms (30 FPS)
- FrameBus distribution: < 1ms (non-blocking)
- Inference: 50-100ms (YOLO640)
- MQTT publish: 10-20ms
- **Total:** ~100-150ms base + network

Queuing would violate this budget, drop policy maintains it.

### Constant Memory

**30 FPS stream, 10 subscribers, buffer=5:**
- Memory: 10 subscribers × 5 frames × ~100KB/frame = ~5MB
- Constant regardless of subscriber processing speed
- Predictable, no OOM risk

**With unbounded queues:**
- After 1 minute with 1 Hz inference: 1740 frames × 10 subscribers × 100KB = ~1.7GB
- Grows linearly with time
- OOM risk in long-running edge deployments

### Graceful Degradation

The drop policy enables graceful degradation under load:

| Scenario | Queuing Approach | Drop Approach (FrameBus) |
|----------|------------------|--------------------------|
| CPU spike | Queue grows, latency increases | Drop rate increases, latency constant |
| Network delay | Queue grows, memory increases | Drop rate increases, memory constant |
| Recovery | Must process backlog (slow) | Immediate (no backlog) |

## Consequences

### Positive

✅ **Predictable latency**: Always processing recent frames
✅ **Constant memory**: No unbounded growth
✅ **Fast recovery**: No backlog to clear after transient issues
✅ **Publisher never blocks**: Stream capture continues at camera FPS
✅ **Subscriber isolation**: Slow subscriber doesn't affect others
✅ **Observable**: Drop stats expose system health

### Negative

⚠ **Incomplete data**: Some frames never processed
⚠ **High drop rates**: Expected 70-97% for typical inference workloads
⚠ **Potential data loss**: Critical events might be in dropped frames

### Mitigations

1. **Stats monitoring**: Core can alert if drop rate exceeds expected range
2. **Buffer tuning**: Subscribers set buffer size based on processing characteristics
3. **Separate critical path**: Use different mechanism for must-process frames
4. **Temporal redundancy**: Most video events span multiple frames

## Expected Drop Rates

Drop rates are **expected and normal** for FrameBus. They indicate the system is working correctly (maintaining real-time).

### Example: 30 FPS Stream, 1 Hz Inference

```
Frames per second:  30
Inference rate:      1 Hz
Frames processed:    1/sec
Frames dropped:     29/sec
Drop rate:          96.7%
```

**This is correct behavior** - processing the most recent frame every second.

### Buffer Impact

Channel buffer size affects drop rate for bursty processing:

| Buffer Size | Processing Pattern | Drop Rate |
|-------------|-------------------|-----------|
| 1 | Uniform 1 Hz | 96.7% |
| 5 | Bursty (0.9s idle, 0.1s process 5 frames) | 83-96% |
| 10 | Very bursty | 67-96% |

**Recommendation:** Small buffers (2-5) for most use cases
- Allows brief processing variations
- Maintains real-time semantics
- Limits staleness (oldest frame in buffer is recent)

## Trade-offs Considered

### Alternative 1: Bounded Queue with Backpressure

```go
// Block publisher when queue full
ch <- frame  // Blocking send
```

**Rejected because:**
- Violates "Publisher never blocks" principle
- Stream capture would stall waiting for slow workers
- One slow worker affects all workers (head-of-line blocking)
- Doesn't solve the fundamental problem (completeness vs latency)

### Alternative 2: Timeout-based Send

```go
select {
case ch <- frame:
    // Success
case <-time.After(10 * time.Millisecond):
    // Drop after timeout
}
```

**Rejected because:**
- Adds complexity (timeout management)
- Publisher can still block for timeout duration
- Timeout value is arbitrary (10ms too much? too little?)
- Doesn't provide benefit over immediate drop

### Alternative 3: Priority Queue

```go
// Keep only N most recent frames, drop oldest
type PriorityQueue struct {
    frames [N]Frame
    // Drop oldest when full
}
```

**Rejected because:**
- Adds significant complexity
- Requires custom data structure
- Defeats channel semantics (select, range)
- Questionable value (if worker is slow, all N frames become stale)

### Alternative 4: Adaptive Buffering

```go
// Dynamically adjust buffer size based on drop rate
if dropRate > 0.95 {
    increaseBufferSize()
}
```

**Rejected because:**
- Can't resize Go channels dynamically
- Would require channel recreation (complex)
- Doesn't solve root cause (slow consumer)
- Better to fix slow consumer or adjust inference rate

## Design Philosophy Alignment

This decision aligns with the Orion Design Manifesto:

**"Complejidad por diseño, no por accidente"**
- The drop policy is **intentional complexity** to handle the inherent trade-off
- Not accidental complexity from poor implementation

**"Pragmatismo > Purismo"**
- Acknowledges that 100% frame processing is impossible and unnecessary
- Optimizes for the actual goal (real-time alerts) not theoretical ideal (completeness)

**"KISS ≠ Simplicidad Ingenua"**
- Simple implementation (`select/default`)
- Handles complex real-world scenario (slow consumers)
- Not simplistic (doesn't ignore the problem)

## Monitoring and Alerting

While FrameBus doesn't alert, it provides data for observability:

```go
// Core periodically checks stats
stats := frameBus.Stats()

for id, subscriber := range stats.Subscribers {
    dropRate := float64(subscriber.Dropped) /
                float64(subscriber.Sent + subscriber.Dropped)

    // Alert if drop rate is unexpectedly LOW
    // (indicates subscriber may be hung, not reading channel)
    if subscriber.Sent > 0 && dropRate < 0.50 {
        log.Warn("subscriber drop rate unexpectedly low",
            "id", id,
            "drop_rate", dropRate,
            "action", "check if subscriber is reading channel")
    }

    // Alert if drop rate is unexpectedly HIGH
    // (indicates inference rate config mismatch)
    if dropRate > 0.99 {
        log.Warn("subscriber drop rate very high",
            "id", id,
            "drop_rate", dropRate,
            "action", "check inference rate config")
    }
}
```

## References

- Orion Philosophy: `VAULT/D002 About Orion.md` ("Drop frames, never queue")
- CLAUDE.md: [CLAUDE.md:49-54](../CLAUDE.md)
- Prototype: `References/orion-prototipe/internal/framebus/bus.go` (lines 71-96)
- Wiki: `VAULT/wiki/2.4-frame-distribution.md` (lines 222-261)

## Related Decisions

- [ADR-001: Channel-based Subscriber Pattern](001-channel-based-subscriber-pattern.md)
- [ADR-003: Stats Tracking Design](003-stats-tracking-design.md)
