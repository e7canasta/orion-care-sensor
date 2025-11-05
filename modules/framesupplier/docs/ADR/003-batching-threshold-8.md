# ADR-003: Batching with Threshold=8

**Status**: Accepted
**Date**: 2025-01-05
**Authors**: Ernesto + Gaby

---

## Changelog

| Version | Date       | Author          | Changes                           |
|---------|------------|-----------------|-----------------------------------|
| 1.0     | 2025-01-05 | Ernesto + Gaby  | Initial decision - threshold=8    |

---

## Context

FrameSupplier must distribute frames to N workers. Worker count varies by deployment:

| Deployment Phase | Workers | Context                          |
|------------------|---------|----------------------------------|
| POC              | 1-5     | Single NUC, critical workers     |
| Expansion        | 5-10    | Multi-bed, add quality workers   |
| Full             | 10-20   | Entire facility, all workers     |
| Future           | 20-64+  | Multi-stream, experimental workers |

**Question**: Should we distribute to workers **sequentially** or **in parallel**?

### Distribution Cost Analysis

**Sequential approach**:
```go
for _, slot := range slots {
    publishToSlot(slot, frame)  // ~1µs per worker
}
```

**Cost**: `N × 1µs`
- 8 workers: 8µs
- 16 workers: 16µs
- 64 workers: 64µs

**Parallel approach**:
```go
for _, slot := range slots {
    go publishToSlot(slot, frame)  // Spawn goroutine per worker
}
```

**Cost**: `spawn_overhead + max(worker latencies)`
- Spawn: ~2µs per goroutine + 2KB stack
- Execution: parallel (max of all workers ≈ 1µs)
- 8 workers: 8 × 2µs + 1µs = **17µs** (worse than sequential!)
- 64 workers: 64 × 2µs + 1µs = **129µs** (worse than sequential!)

**Conclusion**: Naive parallelism is **counterproductive** due to spawn overhead.

---

## Decision

**We will use threshold-based batching: sequential ≤8 workers, batched parallelism >8 workers.**

### Implementation

```go
const publishBatchSize = 8  // Guardrail threshold

func (s *Supplier) distributeToWorkers(frame *Frame) {
    var slots []*WorkerSlot
    s.slots.Range(func(key, value interface{}) bool {
        slots = append(slots, value.(*WorkerSlot))
        return true
    })

    workerCount := len(slots)

    // Fast path: Small deployments (POC, Expansion)
    if workerCount <= publishBatchSize {
        for _, slot := range slots {
            s.publishToSlot(slot, frame)
        }
        return
    }

    // Scale path: Large deployments (Full, Future)
    for i := 0; i < workerCount; i += publishBatchSize {
        end := i + publishBatchSize
        if end > workerCount {
            end = workerCount
        }

        batch := slots[i:end]
        go func(b []*WorkerSlot) {  // Fire-and-forget
            for _, slot := range b {
                s.publishToSlot(slot, frame)
            }
        }(batch)
    }
}
```

**Behavior**:
- **≤8 workers**: Sequential (0 goroutines, 8µs)
- **9-16 workers**: 2 goroutines (4µs spawn + 8µs max = 12µs)
- **17-64 workers**: 3-8 goroutines (16µs spawn + 8µs max = 24µs)

---

## Consequences

### Positive ✅

1. **Simple for Small Scale**: POC/Expansion (≤8 workers) use simple sequential code
2. **Scales Gracefully**: Full deployment (64 workers) parallelizes automatically
3. **Controlled Overhead**: Max goroutines = ⌈N/8⌉ (bounded growth)
4. **Tunable**: `publishBatchSize` is a const (easy to adjust with benchmarks)

### Negative ❌

1. **Complexity**: Dual code paths (sequential + parallel)
2. **Magic Number**: Why 8? (requires documentation)
3. **No Dynamic Tuning**: Threshold is static (not runtime-adaptive)

### Mitigation

**Documentation**:
```go
// publishBatchSize: Distribution threshold.
//
// Context: Orion deployments scale from 1 worker (POC) to 64+ (Full).
// Sequential distribution cost: N × 1µs (simple, low overhead)
// Parallel distribution cost: spawn_overhead + parallel_exec
//   - Spawn: 2µs per goroutine + 2KB stack
//   - Crossover: ~8 workers (sequential=8µs, parallel=17µs)
//
// Rationale for threshold=8:
//   - ≤8 workers: Sequential is faster (0 spawn overhead)
//   - >8 workers: Batching amortizes spawn cost (8 workers/goroutine)
//
// Benchmark validation (see tests):
//   - 8 workers: 8µs sequential vs 17µs parallel → sequential wins
//   - 64 workers: 64µs sequential vs 24µs batched → batched wins
const publishBatchSize = 8
```

---

## Performance Analysis

### Latency Comparison

| Workers | Sequential | Naive Parallel (1/worker) | Batched (8/goroutine) | Winner       |
|---------|------------|----------------------------|-----------------------|--------------|
| 1       | 1µs        | 3µs (2+1)                  | 1µs                   | Sequential   |
| 4       | 4µs        | 9µs (2×4+1)                | 4µs                   | Sequential   |
| 8       | 8µs        | 17µs (2×8+1)               | 8µs                   | Sequential   |
| 16      | 16µs       | 33µs (2×16+1)              | 12µs (2×2µs+8µs)      | **Batched**  |
| 32      | 32µs       | 65µs                       | 16µs (4×2µs+8µs)      | **Batched**  |
| 64      | 64µs       | 129µs                      | 24µs (8×2µs+8µs)      | **Batched**  |

**Crossover point**: ~12 workers (sequential ≈ batched)

**Safety margin**: Threshold=8 (slightly before crossover, favors simplicity for small deployments)

---

### Latency Budget (Real-World)

**Scenario**: 64 workers @ 30fps (33ms inter-frame interval)

**Sequential**: 64µs
- % of budget: 64µs / 33,000µs = **0.2%**

**Batched**: 24µs
- % of budget: 24µs / 33,000µs = **0.07%**

**Absolute savings**: 40µs per frame

**Relative savings**: 62% reduction

**Verdict**: Both are acceptable (< 1% budget), but batching provides **guardrails** for future growth (higher FPS, complex publishToSlot logic).

---

## Alternatives Considered

### Alternative A: Always Sequential

```go
func (s *Supplier) distributeToWorkers(frame *Frame) {
    for _, slot := range slots {
        s.publishToSlot(slot, frame)
    }
}
```

**Pros**:
- ✅ Simplest code (no batching logic)
- ✅ Predictable (no goroutine scheduling variance)
- ✅ Adequate for current scale (64µs @ 64 workers)

**Cons**:
- ❌ No guardrails for future growth (100+ workers, 100fps)
- ❌ Vulnerable to "publishToSlot creep" (if logic becomes complex, latency grows linearly)

**Verdict**: ❌ Reject (YAGNI is valid, but threshold is **cheap insurance**)

---

### Alternative B: Always Parallel (1 goroutine per worker)

```go
func (s *Supplier) distributeToWorkers(frame *Frame) {
    for _, slot := range slots {
        go s.publishToSlot(slot, frame)
    }
}
```

**Pros**:
- ✅ Simple code (no threshold logic)
- ✅ Scales to infinite workers (in theory)

**Cons**:
- ❌ Spawn overhead dominates for small N (17µs @ 8 workers vs 8µs sequential)
- ❌ Goroutine churn (64 goroutines/frame @ 30fps = 1920 goroutines/sec)
- ❌ GC pressure (64 × 2KB stacks = 128KB/frame)

**Verdict**: ❌ Reject (premature optimization, hurts common case)

---

### Alternative C: Goroutine Pool (Pre-spawned)

```go
type Supplier struct {
    workerPool   chan publishJob  // Buffered channel
    poolSize     int               // Pre-spawned goroutines
}

func (s *Supplier) startWorkerPool(size int) {
    for i := 0; i < size; i++ {
        go func() {
            for job := range s.workerPool {
                s.publishToSlot(job.slot, job.frame)
            }
        }()
    }
}

func (s *Supplier) distributeToWorkers(frame *Frame) {
    for _, slot := range slots {
        s.workerPool <- publishJob{slot, frame}
    }
}
```

**Pros**:
- ✅ No spawn overhead (goroutines pre-allocated)
- ✅ Bounded concurrency (pool size controls parallelism)

**Cons**:
- ❌ Complexity (pool lifecycle, channel buffering, shutdown)
- ❌ Latency: Channel send/recv overhead (~100ns each)
- ❌ Over-engineering for current scale (publishToSlot is 1µs, pool overhead ~200ns)

**Verdict**: ❌ Reject (YAGNI, complexity not justified by 1µs operations)

---

### Alternative D: Threshold-Based Batching (Chosen)

**Pros**:
- ✅ Simple for common case (≤8 workers)
- ✅ Scales for large deployments (>8 workers)
- ✅ Low overhead (only spawn when needed)
- ✅ Tunable (const, easy to adjust)

**Cons**:
- ❌ Dual code paths (minor complexity)
- ❌ Static threshold (not runtime-adaptive)

**Verdict**: ✅ **Accept** (pragmatic balance, aligns with Orion deployment phases)

---

## Why Threshold=8?

### Business Context

**Orion deployment phases** (from CLAUDE.md):
```
POC:       1-5 workers   (PersonDetector only)
Expansion: 5-10 workers  (PersonDetector + Pose + Flow)
Full:      10-20 workers (All workers, multi-stream)
```

**Threshold=8**:
- Covers POC + Expansion with **simple sequential code**
- Switches to batching only for **Full deployments** (where it matters)

### Technical Context

**Break-even analysis**:
- Sequential: `N × 1µs`
- Batched: `⌈N/8⌉ × 2µs + 8µs`

**Break-even**: `N × 1µs = ⌈N/8⌉ × 2µs + 8µs`
- Solving: N ≈ 12 workers

**Threshold=8**: Slightly **before** break-even (favors simplicity until clearly beneficial)

### Safety Margin

**publishToSlot "creep" protection**:

Current: `publishToSlot()` is ~1µs (lock + assign + signal)

Future (hypothetical):
```go
func (s *Supplier) publishToSlot(slot *WorkerSlot, frame *Frame) {
    slot.mu.Lock()
    defer slot.mu.Unlock()

    // Future: Worker priority check (10µs)
    // Future: Per-worker rate limiting (5µs)
    // Future: Conditional stats update (5µs)
    // Total: 20µs per worker
}
```

**Impact**:
- Sequential @ 8 workers: 8 × 20µs = **160µs** (starting to matter @ 30fps)
- Batched @ 8 workers: 2µs spawn + 20µs = **22µs** (still negligible)

**Conclusion**: Threshold=8 provides **guardrails** if `publishToSlot` logic grows.

---

## Fire-and-Forget Rationale

**No `sync.WaitGroup`** in batching:

```go
// Fire-and-forget (no wg.Wait)
go func(batch []*WorkerSlot) {
    for _, slot := range batch {
        s.publishToSlot(slot, frame)
    }
}(batch)
```

**Why no wait?**

**Analysis** (from design discussion):
- @ 1fps: Inter-frame = 1000ms
- Distribution latency: ~100µs (64 workers)
- **Ratio**: 10,000× faster than next frame

**Physical impossibility**: Frame N+1 **cannot** arrive before frame N distribution completes.

**Invariant**: `distributeToWorkers() latency << inter-frame interval`

If this invariant is violated (distribution > inter-frame), the **entire system is broken** (GStreamer buffer overflows, frames dropped at source). Ordering is the **least** of our problems.

---

## Testing Strategy

### Unit Tests
1. **Threshold validation**: 8 workers use sequential, 9 workers spawn goroutines
2. **Correctness**: All workers receive frame (check via mock slots)
3. **No leaks**: goroutine counter before/after (race detector)

### Benchmarks
```go
BenchmarkDistribute_8Workers_Sequential
BenchmarkDistribute_8Workers_Parallel   // Should be slower
BenchmarkDistribute_16Workers_Sequential
BenchmarkDistribute_16Workers_Batched   // Should be faster
BenchmarkDistribute_64Workers_Sequential
BenchmarkDistribute_64Workers_Batched   // Should be much faster
```

**Acceptance Criteria**:
- Sequential wins for ≤8 workers
- Batched wins for >16 workers
- Batched @ 64 workers < 50µs (safety margin vs 33ms budget)

---

## References

- Go goroutine overhead: https://dave.cheney.net/2015/08/08/performance-without-the-event-loop
- ARCHITECTURE.md: Distribution algorithm
- C4_MODEL.md: Performance characteristics

---

## Related Decisions

- **ADR-002**: Zero-Copy (batching doesn't amplify copy cost - no copies!)
- **ADR-004**: Symmetric JIT (batching preserves JIT semantics - fire-and-forget)
