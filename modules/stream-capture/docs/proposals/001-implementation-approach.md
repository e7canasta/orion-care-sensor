# Proposal 001: Implementation Approach - Sprint 1.1

**Date**: 2025-11-03
**Status**: ✅ Approved
**Authors**: Ernesto + Gaby

---

## 🎯 Goal

Implement Stream Capture module (Sprint 1.1) with RTSP capture, reconnection, and FPS adaptation.

---

## 📋 Implementation Strategy

### Phase 1: Setup & Public API (Day 1-2)

**Files to create**:
1. `provider.go` - StreamProvider interface definition
2. `types.go` - Frame, StreamConfig types
3. `capture.go` - RTSPStream implementation (skeleton)

**Public API**:
```go
// provider.go
type StreamProvider interface {
    Start(ctx context.Context) (<-chan Frame, error)
    Stop() error
    SetTargetFPS(fps float64) error
}

// types.go
type Frame struct {
    Data      []byte
    Timestamp time.Time
    Width     int
    Height    int
    Sequence  uint64
}

type StreamConfig struct {
    URL       string
    TargetFPS float64
}
```

**Decision**: Start with interface-first approach
- ✅ Defines contract clearly
- ✅ Allows mock implementation for testing
- ✅ FrameBus can depend on interface immediately

---

### Phase 2: GStreamer Integration (Day 3-5)

**Files to create**:
1. `internal/rtsp/pipeline.go` - GStreamer pipeline management
2. `internal/rtsp/callbacks.go` - GStreamer callbacks (appsink)
3. `capture.go` - RTSPStream full implementation

**GStreamer Pipeline**:
```
rtspsrc location=<url> ! videorate ! video/x-raw,framerate=<fps>/1 ! jpegenc ! appsink
```

**Reconnection Logic**:
- Monitor pipeline state
- On ERROR or EOS, trigger reconnection
- Max reconnect attempts: configurable (default: infinite)
- Backoff: 1s, 2s, 4s, 8s (capped at 30s)

**Decision**: Use appsink + channel
- ✅ Non-blocking (aligns with Orion philosophy)
- ✅ Go-friendly (no need for callbacks)
- ⚠️ Requires CGo (acceptable trade-off)

---

### Phase 3: FPS Warm-up (Day 6-7)

**Files to create**:
1. `internal/warmup/measure.go` - FPS measurement during warm-up

**Warm-up Logic**:
```go
func MeasureFPS(frameChan <-chan Frame, duration time.Duration) float64 {
    start := time.Now()
    count := 0

    for {
        select {
        case <-frameChan:
            count++
        case <-time.After(duration):
            elapsed := time.Since(start).Seconds()
            return float64(count) / elapsed
        }
    }
}
```

**Integration**:
- First 5 seconds of stream = warm-up period
- Measured FPS logged for observability
- SetTargetFPS() uses measured FPS to validate

**Decision**: Passive measurement (don't block stream)
- ✅ Simple implementation
- ✅ No impact on latency
- ⚠️ Assumes stable FPS after warm-up

---

### Phase 4: Testing (Day 8-10)

**Test Files**:
1. `capture_test.go` - Unit tests with mock
2. `integration_test.go` - Integration tests with real RTSP (tag: integration)

**Test Coverage**:
- Unit tests: 80% coverage target
  - Mock StreamProvider
  - Frame struct validation
  - Error handling
- Integration tests:
  - Real RTSP camera (if available)
  - Reconnection on stream failure (simulate by killing stream)
  - FPS adaptation (change target FPS mid-stream)

---

## 🔧 External Dependencies

**Add to go.mod**:
```go
require (
    github.com/tinyzimmer/go-gst/gst v0.3.2
    github.com/tinyzimmer/go-glib/glib v0.0.3
)
```

**CGo Requirements**:
- GStreamer 1.0 development libraries
- pkg-config

---

## 🚧 Constraints

1. **No FrameBus dependency yet**
   - Stream Capture is independent (leaf module)
   - FrameBus will depend on Stream Capture, not vice versa

2. **CGo Requirement**
   - GStreamer requires CGo
   - Acceptable trade-off for video processing

3. **RTSP-only (v0.1.0)**
   - No file input, HTTP, or other protocols yet
   - Future: Add MockStream for testing

---

## 📊 Success Criteria

- ✅ StreamProvider interface defined
- ✅ RTSPStream captures frames correctly
- ✅ Reconnection works on stream failure
- ✅ FPS measured during warm-up
- ✅ 80% unit test coverage
- ✅ Integration tests pass with real RTSP

---

## 🔗 References

- [BACKLOG.md](../../BACKLOG.md) - Sprint 1.1 tasks
- [C4 Model - Stream Capture](../../../../docs/DESIGN/C4_MODEL.md#c3---component-diagram)
- [GStreamer Go Bindings](https://github.com/tinyzimmer/go-gst)

---

**Status**: ✅ Approved
**Next**: Start Phase 1 (Setup & Public API)
