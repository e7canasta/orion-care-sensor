# ADR-002: Zero-Copy Frame Sharing

**Status**: Accepted
**Date**: 2025-01-05
**Authors**: Ernesto + Gaby

---

## Changelog

| Version | Date       | Author          | Changes                           |
|---------|------------|-----------------|-----------------------------------|
| 1.0     | 2025-01-05 | Ernesto + Gaby  | Initial decision - zero-copy      |

---

## Context

FrameSupplier distributes video frames to N workers (1-64 workers typical, up to 100+ in future).

**Frame size**: ~50-100KB (JPEG-encoded, 640×480 or 1920×1080)

**Distribution rate**: 1-30fps

**Question**: Should we **copy** frames when distributing, or **share** pointers?

### Performance Context

**Orion competes with**:
- **GStreamer**: All-in-RAM, zero-copy pipelines
- **NVIDIA DeepStream**: Zero-copy GPU memory sharing
- **Intel DL Streamer**: Shared memory buffers

**Why Go?**: Intelligence/orchestration flexibility, not raw performance. But we can't be **orders of magnitude** slower.

### Memory Bandwidth Cost

**Scenario**: 64 workers, 30fps, 100KB frames

**With copying**:
```
Copy per worker: 100KB
Total copies: 64 × 100KB = 6.4MB per frame
Bandwidth: 6.4MB × 30fps = 192 MB/s
```

**Memory bandwidth**: Typical DDR4 ~20 GB/s

**Utilization**: 192 MB/s / 20 GB/s = **1%** (seems acceptable?)

**But**: This is **just FrameSupplier**. GStreamer, inference, result serialization all use bandwidth.

**With zero-copy**:
```
Copy per worker: 0
Total copies: 0
Bandwidth: 0
```

**Savings**: 192 MB/s freed for actual compute.

---

## Decision

**We will use zero-copy (shared pointers) with immutability contract.**

### Implementation

**Frame Type** (immutable by contract):
```go
// Frame is immutable after Publish().
// Publisher MUST NOT modify frame.Data after calling Publish().
// Workers MUST NOT modify frame.Data (read-only access only).
type Frame struct {
    Data      []byte    // JPEG bytes (shared pointer)
    Width     int
    Height    int
    Timestamp time.Time
    Seq       uint64
}
```

**Publishing**:
```go
// stream-capture creates frame
frame := &Frame{
    Data: jpegBytes,  // From GStreamer C.GoBytes (1 copy here)
    // ...
}

// Publish shares pointer (0 copies)
supplier.Publish(frame)

// ❌ ILLEGAL after Publish:
frame.Data[0] = 0xFF  // Undefined behavior!
```

**Distribution** (internal):
```go
// Inbox mailbox
s.inboxFrame = frame  // Share pointer (0 copies)

// Worker slots
slot.frame = frame  // Share pointer (0 copies)
```

**Worker Consumption**:
```go
// Worker receives pointer (0 copies)
frame := readFunc()

// ✅ OK: Read frame.Data
jpegData := frame.Data

// ❌ ILLEGAL: Modify frame.Data
frame.Data[0] = 0xFF  // Violates contract!
```

---

## Consequences

### Positive ✅

1. **Zero Memory Bandwidth**: No frame copies within Go
2. **Competitive Performance**: Matches GStreamer/DeepStream zero-copy philosophy
3. **Low Latency**: Pointer assignment ~1ns (vs memcpy ~50µs for 100KB)
4. **Scalable**: Cost is O(1) regardless of worker count

### Negative ❌

1. **Immutability Burden**: Publisher and workers must not mutate frame.Data
2. **No Compile-Time Enforcement**: Go has no `const` or Rust-style borrow checker
3. **Shared Lifetime**: Frame GC-eligible only when last worker finishes (delayed)
4. **Race Condition Risk**: If contract violated, silent data corruption

### Mitigation Strategies

#### 1. Documentation (Primary)
```go
// Frame is immutable by contract.
//
// SAFETY: After calling Publish(frame), the caller MUST NOT modify
// frame.Data. Workers receiving frames MUST NOT modify frame.Data.
// Violation results in undefined behavior (data races).
type Frame struct { ... }
```

#### 2. Defensive Copying at Boundaries (Secondary)

If a worker **needs** to mutate (rare), **it** copies:
```go
// Worker that needs to modify (e.g., image preprocessing)
frame := readFunc()
mutableCopy := make([]byte, len(frame.Data))
copy(mutableCopy, frame.Data)
// Now safe to mutate mutableCopy
```

#### 3. Runtime Checks (Optional, Dev Only)

```go
// +build debug

type Frame struct {
    Data      []byte
    published atomic.Bool  // Set on Publish()
}

func (f *Frame) checkMutable() {
    if f.published.Load() {
        panic("frame mutated after publish")
    }
}

// Instrumented setters (dev builds only)
func (f *Frame) setData(data []byte) {
    f.checkMutable()
    f.Data = data
}
```

**Decision**: Not in scope for v1 (trust + documentation sufficient).

---

## Alternatives Considered

### Alternative A: Copy-on-Publish

```go
func (s *Supplier) Publish(frame *Frame) {
    // Copy frame.Data for each worker
    for _, slot := range s.slots {
        frameCopy := &Frame{
            Data: make([]byte, len(frame.Data)),
            // ...
        }
        copy(frameCopy.Data, frame.Data)
        slot.frame = frameCopy
    }
}
```

**Pros**:
- ✅ Safe (no shared state)
- ✅ Workers can mutate freely

**Cons**:
- ❌ 192 MB/s bandwidth @ 64 workers, 30fps
- ❌ CPU cost: 64 × 50µs = 3.2ms per frame (10% of 33ms @ 30fps)
- ❌ GC pressure: 64 × 100KB = 6.4MB allocations per frame

**Verdict**: ❌ Reject (performance unacceptable)

---

### Alternative B: Hybrid (Copy-on-Write)

```go
type Frame struct {
    data       []byte
    refCount   atomic.Int32
    copyOnMod  bool
}

func (f *Frame) Data() []byte {
    if f.refCount.Load() == 1 {
        return f.data  // Exclusive access, no copy
    }
    // Shared access, copy on first modification
    // (Complex logic...)
}
```

**Pros**:
- ✅ Zero-copy when possible
- ✅ Safe mutations (COW)

**Cons**:
- ❌ Complexity (reference counting, COW logic)
- ❌ Runtime overhead (atomic ops on every access)
- ❌ Unclear GC interactions

**Verdict**: ❌ Reject (over-engineering, YAGNI)

---

### Alternative C: Zero-Copy with Immutability (Chosen)

**Pros**:
- ✅ Zero overhead (pointer assignment)
- ✅ Simple implementation
- ✅ Matches GStreamer/DeepStream patterns

**Cons**:
- ❌ Requires contract discipline (no enforcement)

**Verdict**: ✅ **Accept** (pragmatic, aligns with Orion philosophy)

---

## Copy Budget Analysis

**Total copies in end-to-end pipeline**:

```
GStreamer appsink (C memory)
    ↓ C.GoBytes()           ← COPY #1 (inevitable, CGo boundary)
Go *Frame (stream-capture)
    ↓ Supplier.Publish()    ← 0 copies (pointer share)
Inbox mailbox
    ↓ distributeToWorkers() ← 0 copies (pointer share)
Worker slots (N)
    ↓ readFunc()            ← 0 copies (pointer share)
Worker Go wrapper
    ↓ MsgPack.Encode()      ← SERIALIZE #2 (inevitable, subprocess boundary)
Python stdin
    ↓ msgpack.decode()      ← DESERIALIZE (Python heap allocation)
NumPy array
```

**Copies we control**: 0 (within FrameSupplier)
**Copies we don't control**: 2 (CGo + MsgPack, inevitable)

**Conclusion**: We've eliminated all **avoidable** copies.

---

## Immutability Contract Enforcement

**Contract Definition**:
```
1. Publisher MUST NOT modify frame.Data after Publish(frame)
2. Workers MUST NOT modify frame.Data received from readFunc()
3. Frame.Data slice is read-only view (backing array shared)
```

**Enforcement Strategy**: **Documentation + Code Review**

**Rationale**:
- Go lacks `const` (unlike C++)
- Go lacks borrow checker (unlike Rust)
- Runtime checks add overhead (unacceptable for hot path)
- Trust between bounded contexts (Orion 2.0 modular architecture)

**Acceptable Risk**: Orion is **not** a general-purpose library. All components are **under our control** (same team, same codebase).

---

## References

- Go memory model: https://go.dev/ref/mem
- GStreamer zero-copy: https://gstreamer.freedesktop.org/documentation/additional/design/memory.html
- ARCHITECTURE.md: Memory Model section
- C4_MODEL.md: Technology Stack

---

## Related Decisions

- **ADR-001**: sync.Cond for Mailbox Semantics (impacts critical section duration)
- **ADR-003**: Batching (benefits from zero-copy - no amplification of copy cost)
- **ADR-004**: Symmetric JIT (zero-copy at all levels)
