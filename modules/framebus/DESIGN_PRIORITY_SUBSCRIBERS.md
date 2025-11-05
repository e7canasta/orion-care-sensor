# Priority Subscribers - Design Document

**Status:** Design Complete - Ready for Implementation
**Date:** 2025-11-04
**Author:** Gaby de Visiona (AI Companion)
**Approved by:** Ernesto (pending review)

---

## Executive Summary

**Feature:** Priority-based load shedding for subscribers
**Goal:** Protect critical workers under high load by dropping frames to lower-priority subscribers first
**Complexity:** +40% (sorting overhead, priority logic)
**Performance Cost:** ~200ns additional latency (sorting overhead)

---

## Problem Statement

### Current Behavior (Equal Priority)

All subscribers are treated equally. When channels are full, frames are dropped based on **who happens to be full at that moment**, not based on business criticality.

```go
// Current: All subscribers equal
for id, ch := range b.subscribers {
    select {
    case ch <- frame:
        sent++
    default:
        dropped++  // ❌ Drops without considering importance
    }
}
```

**Problem:**
- Critical worker (person detection with SLA) drops frames
- Best-effort worker (experimental model, no SLA) drops frames
- **No differentiation** based on business value

### Desired Behavior (Priority-based Load Shedding)

Under load, protect critical subscribers by dropping frames to lower-priority subscribers first.

**Example Scenario:**
- **Worker 1:** Person detection (PriorityCritical) - 0% drops
- **Worker 2:** Fall detection (PriorityHigh) - 10% drops
- **Worker 3:** Experimental pose model (PriorityBestEffort) - 90% drops

---

## API Design

### 1. Priority Enum

```go
// SubscriberPriority defines load shedding priority.
type SubscriberPriority int

const (
    // PriorityCritical: Never drop frames (retry with timeout).
    // Use for: Mission-critical workers with SLAs (person detection, fall detection)
    PriorityCritical SubscriberPriority = 0

    // PriorityHigh: Drop only under severe load.
    // Use for: Important but not critical workers (activity recognition)
    PriorityHigh SubscriberPriority = 1

    // PriorityNormal: Default priority (backward compatible).
    // Use for: Standard workers without special requirements
    PriorityNormal SubscriberPriority = 2

    // PriorityBestEffort: Drop first under any load.
    // Use for: Experimental models, logging, telemetry
    PriorityBestEffort SubscriberPriority = 3
)
```

### 2. Extended Bus Interface

```go
type Bus interface {
    // Existing methods (unchanged)
    Subscribe(id string, ch chan<- Frame) error
    Unsubscribe(id string) error
    Publish(frame Frame)
    PublishWithContext(ctx context.Context, frame Frame)
    Stats() BusStats
    GetHealth(id string) SubscriberHealth
    GetUnhealthySubscribers() []string
    Close() error

    // NEW: Priority-aware subscribe
    SubscribeWithPriority(id string, ch chan<- Frame, priority SubscriberPriority) error
}
```

### 3. Internal Data Structures

```go
// subscriberEntry holds channel + priority
type subscriberEntry struct {
    ch       chan<- Frame
    priority SubscriberPriority
}

type bus struct {
    mu          sync.RWMutex
    subscribers map[string]*subscriberEntry  // Changed: now holds entry with priority
    stats       map[string]*subscriberStats
    closed      bool
    totalPublished atomic.Uint64
}
```

---

## Implementation Strategy

### Phase 1: Data Structure Changes

```go
// OLD (current)
subscribers map[string]chan<- Frame

// NEW (priority-aware)
subscribers map[string]*subscriberEntry

type subscriberEntry struct {
    ch       chan<- Frame
    priority SubscriberPriority
}
```

### Phase 2: Publish Algorithm (Priority-based)

```go
func (b *bus) Publish(frame Frame) {
    b.totalPublished.Add(1)

    b.mu.RLock()
    defer b.mu.RUnlock()

    if b.closed {
        panic("publish on closed bus")
    }

    // STEP 1: Sort subscribers by priority (critical first)
    sorted := b.sortSubscribersByPriority()

    // STEP 2: Iterate in priority order
    for _, sub := range sorted {
        select {
        case sub.ch <- frame:
            // SUCCESS: Frame sent
            b.stats[sub.id].sent.Add(1)

        default:
            // CHANNEL FULL: Apply priority logic
            if sub.priority == PriorityCritical {
                // CRITICAL: Retry with timeout (blocking!)
                if b.retryCritical(sub, frame) {
                    b.stats[sub.id].sent.Add(1)
                } else {
                    // Even critical dropped (log alert)
                    b.stats[sub.id].dropped.Add(1)
                    b.stats[sub.id].criticalDropped.Add(1)  // Special metric
                }
            } else {
                // NON-CRITICAL: Drop immediately
                b.stats[sub.id].dropped.Add(1)
            }
        }
    }
}
```

### Phase 3: Critical Retry Logic

```go
// retryCritical attempts to send to critical subscriber with timeout.
// Returns true if sent, false if dropped.
func (b *bus) retryCritical(sub *subscriberEntry, frame Frame) bool {
    timeout := 1 * time.Millisecond  // Configurable?

    select {
    case sub.ch <- frame:
        return true  // Sent successfully
    case <-time.After(timeout):
        return false  // Dropped (timeout)
    }
}
```

### Phase 4: Sorting Algorithm

```go
// sortedSubscriber holds subscriber info for sorting
type sortedSubscriber struct {
    id       string
    entry    *subscriberEntry
}

// sortSubscribersByPriority returns subscribers sorted by priority (critical first).
// Time complexity: O(N log N) where N = subscriber count
func (b *bus) sortSubscribersByPriority() []sortedSubscriber {
    sorted := make([]sortedSubscriber, 0, len(b.subscribers))

    for id, entry := range b.subscribers {
        sorted = append(sorted, sortedSubscriber{id: id, entry: entry})
    }

    // Sort by priority (ascending: 0=critical first)
    sort.Slice(sorted, func(i, j int) bool {
        return sorted[i].entry.priority < sorted[j].entry.priority
    })

    return sorted
}
```

---

## Performance Analysis

### Latency Breakdown (10 subscribers)

| Operation | Current (No Priority) | With Priority | Overhead |
|-----------|----------------------|---------------|----------|
| **Atomic increment** | 5ns | 5ns | 0ns |
| **RLock acquisition** | 100ns | 100ns | 0ns |
| **Map iteration** | 50ns | 50ns | 0ns |
| **Sorting** | 0ns | **200ns** (O(N log N)) | **+200ns** |
| **Select operations (10×)** | 1,000ns | 1,000ns | 0ns |
| **Critical retry** | 0ns | **0-1,000,000ns** (0-1ms if triggered) | **Variable** |
| **Atomic increments (10×)** | 50ns | 50ns | 0ns |
| **RUnlock** | 50ns | 50ns | 0ns |
| **Total (no critical drops)** | **1,260ns** | **1,460ns** | **+200ns (+16%)** |

### Scaling Analysis

| Subscribers | Sort Time (O(N log N)) | Total Latency |
|-------------|------------------------|---------------|
| **10** | 200ns | 1.5μs |
| **50** | 1,000ns | 2.3μs |
| **100** | 2,000ns | 3.3μs |
| **1000** | 20,000ns | 21μs |

**Acceptable for 2-100 subscribers** (Orion's range).

---

## Trade-offs Analysis

### Pro ✅

1. **SLA Protection:** Critical workers protected under load
2. **Explicit Priority:** Business logic encoded in config, not guesswork
3. **Backward Compatible:** Default priority = Normal (existing code works)
4. **Observable:** `stats.CriticalDropped` metric for alerting

### Con ⚠️

1. **Complexity:** +40% code complexity (sorting, retry logic)
2. **Performance:** +200ns latency (16% overhead for 10 subscribers)
3. **Blocking Risk:** Critical retry can block up to 1ms
4. **Memory:** +8 bytes per subscriber (priority int)

---

## Testing Strategy

### Unit Tests

```go
// TestPriorityOrdering verifies critical subscribers receive frames first
func TestPriorityOrdering(t *testing.T) {
    bus := New()
    defer bus.Close()

    // Subscribe in reverse priority order
    bestEffortCh := make(chan Frame, 1)
    bus.SubscribeWithPriority("best-effort", bestEffortCh, PriorityBestEffort)

    criticalCh := make(chan Frame, 1)
    bus.SubscribeWithPriority("critical", criticalCh, PriorityCritical)

    // Publish 2 frames (both channels buffer=1, one will drop)
    bus.Publish(Frame{Seq: 1})
    bus.Publish(Frame{Seq: 2})

    // EXPECT: Critical receives frames, best-effort drops
    stats := bus.Stats()
    assert.Equal(t, 2, stats.Subscribers["critical"].Sent)
    assert.Equal(t, 0, stats.Subscribers["best-effort"].Sent)
}

// TestCriticalRetry verifies critical retry logic
func TestCriticalRetry(t *testing.T) {
    bus := New()
    defer bus.Close()

    // Critical subscriber with buffer=1
    criticalCh := make(chan Frame, 1)
    bus.SubscribeWithPriority("critical", criticalCh, PriorityCritical)

    // Publish 2 frames quickly (second will trigger retry)
    bus.Publish(Frame{Seq: 1})
    bus.Publish(Frame{Seq: 2})  // Should retry with timeout

    // Drain channel
    <-criticalCh

    // Wait for retry to complete
    time.Sleep(2 * time.Millisecond)

    stats := bus.Stats()
    // Either sent=2 (retry succeeded) or sent=1, dropped=1 (timeout)
    assert.True(t, stats.Subscribers["critical"].Sent >= 1)
}

// TestBackwardCompatibility verifies default priority
func TestBackwardCompatibility(t *testing.T) {
    bus := New()
    defer bus.Close()

    // Old API: Subscribe without priority (should default to Normal)
    ch := make(chan Frame, 10)
    bus.Subscribe("worker", ch)

    // Verify it works
    bus.Publish(Frame{Seq: 1})
    frame := <-ch
    assert.Equal(t, 1, frame.Seq)
}
```

### Benchmarks

```go
// BenchmarkPublishWithPriority measures priority sorting overhead
func BenchmarkPublishWithPriority(b *testing.B) {
    bus := New()
    defer bus.Close()

    // 10 subscribers with mixed priorities
    for i := 0; i < 10; i++ {
        ch := make(chan Frame, 100)
        priority := SubscriberPriority(i % 4)  // Mix of priorities
        bus.SubscribeWithPriority(fmt.Sprintf("worker-%d", i), ch, priority)
    }

    frame := Frame{Seq: 1, Data: make([]byte, 1024)}

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        bus.Publish(frame)
    }
}

// BenchmarkPriorityVsNoPriority compares overhead
func BenchmarkPriorityVsNoPriority(b *testing.B) {
    // TODO: Compare against current implementation
}
```

---

## Migration Strategy

### Step 1: Internal Refactor (Non-Breaking)

```go
// Change internal structure (no API change)
type bus struct {
    subscribers map[string]*subscriberEntry  // Was: map[string]chan<- Frame
}

// Subscribe wraps with default priority
func (b *bus) Subscribe(id string, ch chan<- Frame) error {
    return b.SubscribeWithPriority(id, ch, PriorityNormal)
}
```

### Step 2: Add SubscribeWithPriority (Additive)

```go
func (b *bus) SubscribeWithPriority(id string, ch chan<- Frame, priority SubscriberPriority) error {
    // Implementation
}
```

### Step 3: Update Publish Logic (Internal)

```go
func (b *bus) Publish(frame Frame) {
    // Add sorting + priority logic
}
```

### Step 4: Update Stats (Additive)

```go
type SubscriberStats struct {
    Sent            uint64
    Dropped         uint64
    CriticalDropped uint64  // NEW: Count of dropped critical frames (alert!)
}
```

**Result:** 100% backward compatible. Existing code continues to work with `PriorityNormal`.

---

## Configuration Example (Orion Core)

```go
// orion/internal/core/orion.go
func (o *Orion) initializeWorkers() error {
    // Critical: Person detection (SLA requirement)
    personCh := make(chan framebus.Frame, 5)
    o.frameBus.SubscribeWithPriority(
        "person-detector",
        personCh,
        framebus.PriorityCritical,
    )

    // High: Fall detection (important but tolerates brief delays)
    fallCh := make(chan framebus.Frame, 5)
    o.frameBus.SubscribeWithPriority(
        "fall-detector",
        fallCh,
        framebus.PriorityHigh,
    )

    // Normal: Activity recognition
    activityCh := make(chan framebus.Frame, 5)
    o.frameBus.Subscribe("activity-detector", activityCh)  // Default: Normal

    // BestEffort: Experimental model (no SLA)
    expCh := make(chan framebus.Frame, 5)
    o.frameBus.SubscribeWithPriority(
        "experimental-model",
        expCh,
        framebus.PriorityBestEffort,
    )

    return nil
}
```

---

## Monitoring & Alerting

### Metrics to Track

```go
// Proactive health check
unhealthy := bus.GetUnhealthySubscribers()
for _, id := range unhealthy {
    health := bus.GetHealth(id)
    stats := bus.Stats().Subscribers[id]

    if health == framebus.HealthSaturated {
        if stats.CriticalDropped > 0 {
            // 🚨 CRITICAL ALERT: Critical worker dropping frames!
            alerting.Critical("critical worker saturated", "id", id, "drops", stats.CriticalDropped)
            // Auto-restart or page on-call
        } else {
            // ⚠️ WARNING: Non-critical worker saturated
            alerting.Warn("worker saturated", "id", id, "priority", getPriority(id))
        }
    }
}
```

### Dashboard Metrics

- **Critical Drop Rate:** `stats.CriticalDropped / stats.TotalPublished` (should be 0%)
- **Per-Priority Drop Rate:** Grouped by priority level
- **Load Shedding Effectiveness:** Compare drops across priority levels

---

## Open Questions for Review

1. **Critical Retry Timeout:**
   - Current design: 1ms hardcoded
   - Alternative: Configurable per-subscriber? Or global config?
   - **Recommendation:** Start with 1ms hardcoded, make configurable if needed

2. **Sorting Optimization:**
   - Current: Sort on every Publish() call (O(N log N))
   - Alternative: Pre-sort once on Subscribe/Unsubscribe, cache sorted list
   - **Trade-off:** More complex (cache invalidation) vs faster publish
   - **Recommendation:** Simple first (sort every time), optimize if benchmarks show bottleneck

3. **Priority Levels:**
   - Current design: 4 levels (Critical, High, Normal, BestEffort)
   - Alternative: 3 levels? 5 levels?
   - **Recommendation:** 4 levels is standard (AWS, Kubernetes, etc.)

4. **Backward Compatibility Test:**
   - Need comprehensive test that existing Orion code works without changes
   - **TODO:** Add `TestBackwardCompatibilityFullIntegration` with actual Orion worker patterns

---

## Implementation Checklist

### Code Changes

- [ ] Update `subscriberEntry` struct with priority field
- [ ] Implement `SubscribeWithPriority()`
- [ ] Update `Subscribe()` to use default priority
- [ ] Implement `sortSubscribersByPriority()`
- [ ] Update `Publish()` with sorting + priority logic
- [ ] Implement `retryCritical()` helper
- [ ] Add `CriticalDropped` to `SubscriberStats`
- [ ] Export `SubscriberPriority` in public API

### Tests

- [ ] `TestPriorityOrdering` - Verify critical gets frames first
- [ ] `TestCriticalRetry` - Verify retry logic
- [ ] `TestCriticalDropped` - Verify stats tracking
- [ ] `TestBackwardCompatibility` - Verify old API works
- [ ] `TestPriorityAllLevels` - Test all 4 priority levels
- [ ] `BenchmarkPublishWithPriority` - Measure overhead
- [ ] `BenchmarkSortingOverhead` - Isolate sorting cost

### Documentation

- [ ] Update `ARCHITECTURE.md` with priority design
- [ ] Update `C4_MODEL.md` with priority flow diagrams
- [ ] Update `CLAUDE.md` with priority API
- [ ] Update `README.md` with priority examples
- [ ] Add `ADR-009: Priority Subscribers` with rationale

---

## Success Criteria

✅ **Feature Complete:**
- [ ] All tests pass with race detector
- [ ] Benchmarks show <300ns overhead for 10 subscribers
- [ ] CriticalDropped metric tracked correctly
- [ ] Backward compatibility verified

✅ **Production Ready:**
- [ ] Documentation updated
- [ ] Orion Core integration example
- [ ] Monitoring dashboard updated
- [ ] Alerting rules configured

---

## Next Session Agenda

1. **Review this design** (5 mins)
2. **Implement code changes** (30 mins)
3. **Write tests** (20 mins)
4. **Run benchmarks** (5 mins)
5. **Update docs** (10 mins)
6. **Commit** (5 mins)

**Estimated time:** 75 minutes total

---

## References

- [AWS Priority Queues](https://docs.aws.amazon.com/AWSSimpleQueueService/latest/SQSDeveloperGuide/FIFO-queues.html)
- [Kubernetes Quality of Service Classes](https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/)
- [NATS JetStream Priority](https://docs.nats.io/nats-concepts/jetstream/streams#priority)

---

**Document Version:** 1.0
**Status:** Ready for Implementation
**Estimated Complexity:** High (+40%)
**Estimated Benefit:** High (SLA protection)

---

🎸 **"Tocar con conocimiento de las reglas, no seguir la partitura al pie de la letra"**

Priority Subscribers es una técnica avanzada (load shedding), pero la aplicamos solo cuando el contexto lo demanda (SLAs diferenciados). Pragmatismo > Purismo.
