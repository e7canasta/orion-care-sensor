# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Module Overview

**stream-capture** is a bounded context within Orion 2.0 that provides RTSP video stream acquisition using GStreamer. It is a standalone Go module designed for low-latency frame extraction with hot-reload capabilities.

**Positioning**: This is NOT a complete streaming solution - it's a specialized module for acquiring frames from RTSP cameras with precise FPS control and resilience.

### Core Responsibilities

- ✅ RTSP stream acquisition via GStreamer pipelines
- ✅ Frame rate control (0.1-30 FPS) with hot-reload support
- ✅ Resolution scaling (512p, 720p, 1080p)
- ✅ Automatic reconnection with exponential backoff
- ✅ FPS stability measurement (warmup phase)
- ✅ Non-blocking frame distribution with drop tracking

### Technology Stack

- **Go 1.21+**: Module implementation
- **GStreamer**: Video pipeline with hardware acceleration support
  - Software: `rtspsrc → rtph264depay → avdec_h264 → videoconvert → videoscale → videorate → capsfilter → appsink`
  - VAAPI (Intel): `rtspsrc → rtph264depay → vaapidecodebin → vaapipostproc → videoconvert → videoscale → videorate → capsfilter → appsink`
- **go-gst**: GStreamer Go bindings (`github.com/tinyzimmer/go-gst`)
- **VAAPI**: Intel Quick Sync hardware acceleration (optional, i5/i7 6th gen+)
  - GPU: H.264 decode + YUV processing
  - CPU: YUV→RGB conversion (minimal overhead)

## Development Commands

### Building

```bash
# Build module library
make build

# Build test binary
make test-capture

# Build examples
make examples

# Build everything
make all
```

### Testing

**Testing Philosophy**: Manual pair-programming approach (user runs tests, you observe). Compilation is primary validation.

```bash
# Run unit tests
make test

# Run with race detector
make test-verbose

# Development workflow (lint + build + test)
make dev
```

### Running Test Capture

```bash
# Basic test (no frame saving)
RTSP_URL=rtsp://camera/stream make run-test

# Save frames to disk
RTSP_URL=rtsp://camera/stream OUTPUT_DIR=./frames make run-test

# Custom FPS and resolution
RTSP_URL=rtsp://camera/stream FPS=1.0 RES=720p make run-test

# JPEG output with quality control
RTSP_URL=rtsp://camera/stream OUTPUT_DIR=./frames FORMAT=jpeg JPEG_QUALITY=85 make run-test

# Hardware acceleration (VAAPI on Intel i5/i7)
RTSP_URL=rtsp://camera/stream ACCEL=vaapi make run-test       # Force VAAPI, fail if unavailable
RTSP_URL=rtsp://camera/stream ACCEL=auto make run-test        # Try VAAPI, fallback to software (default)
RTSP_URL=rtsp://camera/stream ACCEL=software make run-test    # Force software decode

# Skip warmup (useful for low FPS testing)
RTSP_URL=rtsp://camera/stream SKIP_WARMUP=1 make run-test

# Debug logging
RTSP_URL=rtsp://camera/stream DEBUG=1 make run-test

# Limit frame capture
RTSP_URL=rtsp://camera/stream MAX_FRAMES=100 make run-test

# Combined example (low FPS with VAAPI, skip warmup)
RTSP_URL=rtsp://camera/stream ACCEL=vaapi FPS=0.5 MAX_FRAMES=10 SKIP_WARMUP=1 make run-test
```

### Running Examples

```bash
# Simple example (minimal code)
RTSP_URL=rtsp://camera/stream make run-simple

# Hot-reload example (demonstrates SetTargetFPS)
RTSP_URL=rtsp://camera/stream make run-hotreload
```

## Architecture: The Big Picture

### Design Philosophy

1. **"Complejidad por diseño, no por accidente"** - Manage complexity through architecture, not complicated code
2. **Fail-Fast Validation** - Validate configuration at construction time (load time > runtime)
3. **Non-Blocking Channels** - Drop frames to maintain low latency (<2s), never queue
4. **Hot-Reload Over Restart** - FPS changes via capsfilter update (~2s) vs full restart (5-10s)
5. **Resilience via Reconnection** - Exponential backoff with configurable max retries
6. **Thread Safety First** - `sync.RWMutex` for state, `atomic` for counters, context for cancellation

### Core Pattern: Provider Interface

The module exports a **StreamProvider** interface that guarantees:

- `Start()` returns immediately (non-blocking)
- `Start()` returns a channel that never closes until `Stop()`
- `Stop()` is idempotent (safe to call multiple times)
- `Stats()` is thread-safe (callable from any goroutine)
- `SetTargetFPS()` does NOT require restart (hot-reload)
- `Warmup()` measures FPS stability before production use

**Implementation**: `RTSPStream` (currently the only implementation)

### GStreamer Pipeline Structure

**Software Decode Pipeline** (CPU-based, maximum compatibility):
```
rtspsrc → rtph264depay → avdec_h264 → videoconvert → videoscale →
videorate → capsfilter → appsink
    ↓         (CPU)         (CPU)         (CPU)
protocols=4            drop-only=true  hot-reload
latency=200ms                         framerate caps
```

**VAAPI Hardware Decode Pipeline** (Intel Quick Sync, i5/i7 only):
```
rtspsrc → rtph264depay → vaapidecodebin → vaapipostproc → videoconvert →
    ↓                        (GPU H.264)      (GPU NV12)      (CPU YUV→RGB)
protocols=4                                                       ↓
latency=200ms                                                videoscale → videorate →
                                                                  capsfilter → appsink
```

**Key Properties**:
- `rtspsrc`: TCP-only (protocols=4) for go2rtc compatibility
- `videorate`: drop-only mode (never duplicates frames)
- `capsfilter`: hot-reload target for FPS changes
- `appsink`: delivers frames to Go via callbacks
- **VAAPI Pipeline**: H.264 decode in GPU, outputs NV12 (YUV), then CPU converts YUV→RGB
- **VAAPI Benefits**: ~70% less CPU usage, ~50% lower decode latency (10-15ms → 3-5ms)
- **Why videoconvert after VAAPI**: `vaapipostproc` outputs NV12 format, need CPU conversion to RGB for ONNX

### Lifecycle Management

**Start Sequence**:
1. `NewRTSPStream()` - fail-fast validation (URL, FPS, resolution, GStreamer)
2. `Start(ctx)` - create pipeline, set to PLAYING, return channel immediately
3. `Warmup(ctx, duration)` - measure real FPS and stability (RECOMMENDED before production)
4. Consume frames from channel

**Stop Sequence**:
1. Cancel context (signals shutdown)
2. Wait for goroutines (timeout 3s)
3. Destroy GStreamer pipeline
4. Close frame channel (double-close protected)
5. Reset state for restart

### Goroutine Structure

Each `RTSPStream` runs **3 goroutines**:

1. **Frame Converter**: `internalFrames → frames` (converts `rtsp.Frame` → `streamcapture.Frame`)
   - Non-blocking send with drop tracking
   - Updates `lastFrameAt` for latency metric

2. **Pipeline Monitor**: `runPipeline()` → `monitorPipeline()`
   - Polls GStreamer bus for messages (50ms intervals)
   - Triggers reconnection on error/EOS
   - Resets reconnect state when PLAYING

3. **Reconnection Logic**: `RunWithReconnect()` (inside `runPipeline`)
   - Exponential backoff: 1s → 2s → 4s → 8s → 16s (max 5 retries)
   - Resets on successful connection
   - Logs failures with context

### Hot-Reload Mechanism

**Target**: Change FPS without full pipeline restart

**Implementation**:
```go
// rtsp.go:493
func (s *RTSPStream) SetTargetFPS(fps float64) error {
    // 1. Validate range (0.1-30)
    // 2. Update capsfilter caps (hot-reload)
    // 3. Rollback on error
    // 4. Update internal state
}
```

**Interruption**: ~2 seconds (vs 5-10s for full restart)

**Mechanism**: Updates `capsfilter` caps via `rtsp.UpdateFramerateCaps()` using GStreamer's dynamic reconfiguration.

### Thread Safety

- **State Access**: `sync.RWMutex` for `frames`, `cancel`, `elements`, `lastFrameAt`
- **Counters**: `atomic.Uint64` for `frameCount`, `framesDropped`, `bytesRead`
- **Reconnects**: `atomic.Uint32` for `reconnectState.Reconnects`
- **Shutdown**: `atomic.Bool` for `framesClosed` (prevents double-close panic)
- **Cancellation**: `context.Context` for goroutine coordination

## Code Structure

### Public API (`streamcapture` package)

| File | Purpose |
|------|---------|
| `provider.go` | `StreamProvider` interface definition |
| `types.go` | `Frame`, `StreamStats`, `Resolution`, `RTSPConfig`, `WarmupStats` |
| `rtsp.go` | `RTSPStream` implementation |
| `warmup_stats.go` | `CalculateFPSStats()` - FPS stability calculation |

### Internal Packages

| Package | Purpose |
|---------|---------|
| `internal/rtsp/` | GStreamer pipeline management |
| `internal/rtsp/pipeline.go` | Pipeline creation, caps update, cleanup |
| `internal/rtsp/callbacks.go` | GStreamer callbacks (`OnNewSample`, `OnPadAdded`) |
| `internal/rtsp/reconnect.go` | Exponential backoff reconnection logic |
| `internal/warmup/` | Warmup statistics (minimal, delegates to `warmup_stats.go`) |

### Binaries

| Binary | Purpose |
|--------|---------|
| `cmd/test-capture/` | Full-featured test harness (frame saving, stats, CLI) |
| `examples/simple/` | Minimal example (20 lines) |
| `examples/hot-reload/` | Hot-reload demonstration |

## Key Architectural Decisions

### AD-1: Fail-Fast Validation (Load Time vs Runtime)

**Why**: Catch configuration errors immediately at construction, not during runtime.

**Implementation**:
- `NewRTSPStream()` validates URL, FPS range (0.1-30), resolution, GStreamer availability
- Returns error immediately if invalid
- **Trade-off**: Slightly more upfront complexity, but eliminates "runtime debugging hell"

**Location**: `rtsp.go:61-110`

### AD-2: Non-Blocking Channel with Drop Tracking

**Why**: Latency > completeness. Prefer dropping frames over queuing to maintain <2s latency.

**Implementation**:
```go
select {
case s.frames <- publicFrame:
    // Success
default:
    // Drop and track
    atomic.AddUint64(&s.framesDropped, 1)
}
```

**Metrics**: `StreamStats.FramesDropped`, `StreamStats.DropRate`

**Location**: `rtsp.go:196-209`

### AD-3: Hot-Reload via Capsfilter Update

**Why**: ~2s interruption vs 5-10s full restart. Critical for dynamic rate control.

**Implementation**: Update `capsfilter` caps string with new framerate using GStreamer's dynamic reconfiguration.

**Rollback**: On failure, previous FPS is preserved (no state change).

**Location**: `rtsp.go:493-542`, `internal/rtsp/pipeline.go`

### AD-4: Exponential Backoff Reconnection

**Why**: Transient network failures are common in RTSP. Backoff prevents thundering herd.

**Schedule**: 1s → 2s → 4s → 8s → 16s (max 5 retries, configurable)

**Reset**: On successful PLAYING state, reset retry counter.

**Location**: `internal/rtsp/reconnect.go:36-91`

### AD-5: Warmup Phase for FPS Stability

**Why**: GStreamer pipelines take time to stabilize. Production code needs to know real FPS.

**Implementation**: Consume frames for N seconds, calculate FPS mean/stddev, determine stability.

**Stability Threshold**: `stddev < 15% of mean`

**Location**: `rtsp.go:572-645`, `warmup_stats.go:8-82`

### AD-6: Double-Close Panic Protection

**Why**: Go panics on double-close. Idempotent `Stop()` requires protection.

**Implementation**: `atomic.Bool` flag (`framesClosed`) with `CompareAndSwap` for exactly-once close.

**Location**: `rtsp.go:414-419`

### AD-7: VAAPI Hardware Acceleration with Auto-Fallback

**Why**: Intel i5/i7 deployments benefit from ~70% CPU reduction via Quick Sync. Desacoplamiento de decodificación H.264 del CPU es crítico para multi-stream (Fase 2).

**Implementation**: Three modes via `HardwareAccel` enum:
- `AccelAuto` (default): Try VAAPI, fallback to software if unavailable
- `AccelVAAPI`: Force VAAPI, fail-fast if missing (production i5/i7)
- `AccelSoftware`: Force software decode (debugging, VMs)

**Pipeline Changes**:
- Software: `avdec_h264 → videoconvert → videoscale` (all CPU)
- VAAPI: `vaapidecodebin → vaapipostproc → videoconvert → videoscale`
  - GPU: H.264 decode (heavy) + YUV processing
  - CPU: YUV→RGB conversion (lightweight, ~2-3ms)

**Why videoconvert after vaapipostproc?**
- `vaapipostproc` outputs NV12 (YUV planar format)
- ONNX models expect RGB packed format
- CPU YUV→RGB conversion is fast (~2-3ms for 720p)
- Alternative (pure GPU) would require `vaapipostproc format=RGB` but causes negotiation issues with some cameras

**Performance Impact**:
- CPU usage: 25-40% → 5-10% (per stream)
- Decode latency: 10-15ms → 3-5ms
- Critical for >2 concurrent streams
- Tested with Intel iHD driver 25.2.6 on 6th gen+ CPUs

**Fail-Fast Validation**: `checkVAAPIAvailable()` validates `vaapidecodebin` and `vaapipostproc` at construction time when `AccelVAAPI` is set.

**Trade-off**: Adds GStreamer VAAPI dependency + small CPU YUV→RGB overhead, but massive win on H.264 decode (the bottleneck).

**Location**: `types.go:90-118`, `rtsp.go:89-94`, `internal/rtsp/pipeline.go:67-233`

## Common Patterns

### Starting a Stream

```go
// 1. Create stream with fail-fast validation
cfg := streamcapture.RTSPConfig{
    URL:          "rtsp://192.168.1.100/stream",
    Resolution:   streamcapture.Res720p,
    TargetFPS:    2.0,
    SourceStream: "camera-01",
    Acceleration: streamcapture.AccelAuto, // Try VAAPI, fallback to software (default)
}
stream, err := streamcapture.NewRTSPStream(cfg)
if err != nil {
    log.Fatal(err)
}

// 2. Start stream (non-blocking)
ctx, cancel := context.WithCancel(context.Background())
defer cancel()

frameChan, err := stream.Start(ctx)
if err != nil {
    log.Fatal(err)
}

// 3. RECOMMENDED: Warmup to measure stability
stats, err := stream.Warmup(ctx, 5*time.Second)
if err != nil {
    log.Fatal(err)
}
log.Printf("Stream stable: %v, FPS: %.2f", stats.IsStable, stats.FPSMean)

// 4. Consume frames
for frame := range frameChan {
    // Process frame...
}
```

### Hardware Acceleration (VAAPI)

```go
// Force VAAPI hardware acceleration (i5/i7 Intel only)
// Fails fast if VAAPI not available (gstreamer1.0-vaapi missing or no Intel GPU)
cfg := streamcapture.RTSPConfig{
    URL:          "rtsp://192.168.1.100/stream",
    Resolution:   streamcapture.Res1080p,  // Higher resolution = more VAAPI benefit
    TargetFPS:    5.0,                     // Higher FPS = more CPU saved
    SourceStream: "camera-sala-103",
    Acceleration: streamcapture.AccelVAAPI, // Fail-fast if unavailable
}
stream, err := streamcapture.NewRTSPStream(cfg)
if err != nil {
    // Error: "VAAPI not available: vaapidecodebin not found..."
    log.Fatal(err)
}

// Auto-fallback mode (recommended for production)
cfg.Acceleration = streamcapture.AccelAuto  // Try VAAPI, fallback to software
stream, err = streamcapture.NewRTSPStream(cfg)
// Never fails due to VAAPI unavailability

// Force software decode (debugging, compatibility)
cfg.Acceleration = streamcapture.AccelSoftware
```

### Hot-Reload FPS

```go
// Change FPS dynamically (~2s interruption)
err := stream.SetTargetFPS(0.5)  // 0.5 Hz = 1 frame every 2 seconds
if err != nil {
    log.Printf("Failed to update FPS: %v", err)
}
```

### Monitoring Statistics

```go
// Thread-safe stats retrieval
stats := stream.Stats()
log.Printf("Frames: %d, Drop Rate: %.1f%%, FPS: %.2f, Latency: %dms",
    stats.FrameCount,
    stats.DropRate,
    stats.FPSReal,
    stats.LatencyMS,
)
```

### Graceful Shutdown

```go
// Idempotent shutdown
if err := stream.Stop(); err != nil {
    log.Printf("Error stopping stream: %v", err)
}

// Safe to call multiple times
stream.Stop()  // No-op, returns nil
```

## Testing Strategy

**Philosophy**: Manual testing with pair-programming. No automated test files in early prototype.

**Primary Validation**: Compilation success (`make build`, `make test`)

**Integration Testing**:
1. Use `test-capture` binary with real RTSP streams
2. Observe stats output every 10 seconds
3. Verify warmup stability measurements
4. Test hot-reload via `examples/hot-reload`

**Review Approach**: User runs test, you observe and review results together.

## Dependencies

- **GStreamer** (system): Required runtime dependency
  - **Software decode** (minimum): `apt install gstreamer1.0-tools gstreamer1.0-plugins-base gstreamer1.0-plugins-good gstreamer1.0-plugins-bad gstreamer1.0-libav`
  - **VAAPI acceleration** (Intel i5/i7): `apt install gstreamer1.0-vaapi libva-drm2 intel-media-va-driver`
  - Elements needed (software): `rtspsrc`, `rtph264depay`, `avdec_h264`, `videoconvert`, `videoscale`, `videorate`, `capsfilter`, `appsink`
  - Elements needed (VAAPI): `rtspsrc`, `rtph264depay`, `vaapidecodebin`, `vaapipostproc`, `videorate`, `capsfilter`, `appsink`

- **Go Modules**:
  - `github.com/tinyzimmer/go-gst` - GStreamer Go bindings
  - `github.com/google/uuid` - UUID generation for TraceID

- **Hardware Requirements (VAAPI)**:
  - Intel CPU with Quick Sync (i5/i7 6th gen or newer)
  - Linux kernel with i915 driver
  - Permissions: User must be in `video` group (`sudo usermod -aG video $USER`)

## Commit Standards

- Co-authored by: `Gaby de Visiona <noreply@visiona.app>`
- Do NOT include "Generated with Claude Code" footer (implicit in co-author)
- Focus on "why" rather than "what" in commit messages

## Known Issues / Future Work

- **Multi-Stream Support**: Current design assumes single stream per instance. Multi-stream requires `stream_id` metadata (BACKLOG Fase 2).
- **Frame Format**: Currently outputs RGB (3 bytes/pixel). Intentionally simple - Python ONNX expects RGB.
- **ROI Processing**: Not implemented in this module (handled by consumers).
- **Memory Pool**: Frame allocations not pooled. Add `sync.Pool` if GC becomes bottleneck (measure first).
- **Metrics Export**: Stats are in-memory only. Future: Prometheus exporter.

## Module Positioning (Orion 2.0 Context)

This module is **Bounded Context #1** in Orion 2.0's multi-module monorepo:

```
OrionWork/
├── modules/
│   ├── stream-capture/     ← YOU ARE HERE (Sprint 1.1)
│   ├── worker-lifecycle/   (Sprint 1.2)
│   ├── framebus/           (Sprint 2)
│   ├── control-plane/      (Sprint 2)
│   └── core/               (Sprint 3)
```

**Isolation**: This module has NO dependencies on other Orion modules. It can be used standalone.

**Consumers**: `framebus` and `core` will depend on this module in future sprints.
