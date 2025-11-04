# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.


## Key Sections

1. Module Overview: Bounded context within Orion 2.0, design philosophy (fail-fast, non-blocking, hot-reload)
2. Building and Testing: All Makefile commands, manual testing approach with test-capture tool
3. Architecture:
	- StreamProvider interface contract
	- GStreamer pipeline structure
	- Component organization
	- Concurrency model (3 goroutines, synchronization primitives)
4. Key Architectural Decisions: 5 major decisions documented (fail-fast, non-blocking, hot-reload, reconnection, TCP-only)
5. Configuration & Data Types: RTSPConfig, Frame format, resolution dimensions
6. Usage Patterns: Code examples for basic capture, hot-reload, and statistics
7. Module Boundaries: Clear anti-responsibilities (what this BC does NOT do)
8. Known Issues: Warm-up not integrated, import cycle workarounds, dynamic pad linking
9. Testing Strategy: Manual testing with test-capture, acceptance criteria
10. Code Patterns: Thread safety, error handling, logging standards
11. Integration Context: Multi-module monorepo structure, future integration points


## Module Overview

**stream-capture** is a bounded context (BC) within Orion 2.0 responsible for RTSP stream acquisition and frame distribution. It implements the first phase of the video processing pipeline: capturing frames from RTSP cameras via GStreamer and distributing them to downstream workers.


### Design Philosophy

- **Fail-fast validation**: Validate at construction time (e.g., FPS range, RTSP URL), not runtime
- **Non-blocking channels**: Drop frames rather than queue to maintain low latency (<2s)
- **Hot-reload capabilities**: Change FPS without full stream restart (~2s interruption vs 5-10s)
- **Automatic reconnection**: Exponential backoff with configurable max retries
- **Thread-safe statistics**: Atomic counters for metrics tracking

### Technology Stack

- **Go 1.21+**: Core orchestration and concurrency
- **GStreamer 1.0**: RTSP stream decoding (via go-gst bindings)
- **Protocol**: TCP-only RTSP (compatibility with go2rtc)

## Building and Testing

### Build Commands

```bash
# Build module library
make build

# Build test-capture binary (manual testing tool)
make test-capture

# Build example binaries
make examples          # Build all examples
make examples/simple   # Build simple example
make examples/hot-reload  # Build hot-reload example

# Build everything
make all
```

### Running Tests

**Testing Philosophy**: Manual testing with pair-programming approach. The user runs tests manually for review.

```bash
# Unit tests (compile verification)
make test

# Manual testing with test-capture tool
RTSP_URL=rtsp://camera/stream make run-test

# Custom parameters
RTSP_URL=rtsp://camera/stream \
OUTPUT_DIR=./frames \
FPS=1.0 \
RES=720p \
FORMAT=jpeg \
make run-test

# Debug mode
RTSP_URL=rtsp://camera/stream DEBUG=1 make run-test
```

### Development Workflow

```bash
# Format, vet, and lint
make lint

# Full development check (lint + build + test)
make dev
```

## Architecture: The Big Picture

### Core Abstraction

The module exposes a single interface: `StreamProvider`

```go
type StreamProvider interface {
    Start(ctx context.Context) (<-chan Frame, error)
    Stop() error
    Stats() StreamStats
    SetTargetFPS(fps float64) error
}
```

**Key Guarantees**:
- `Start()` returns a channel that never closes until `Stop()`
- `Stop()` is idempotent (safe to call multiple times)
- `Stats()` is thread-safe (atomic operations)
- `SetTargetFPS()` hot-reloads without restart

### GStreamer Pipeline Structure

```
rtspsrc (TCP) → rtph264depay → avdec_h264 → videoconvert → videoscale
                                                                ↓
                                            videorate → capsfilter → appsink
```

**Key Elements**:
- `rtspsrc`: RTSP source with `protocols=4` (TCP only), `latency=200ms`
- `videorate`: FPS control with `drop-only=true` (never duplicate frames)
- `capsfilter`: Dynamic framerate caps for hot-reload
- `appsink`: Non-blocking sink with `max-buffers=1`, `drop=true`

### Component Structure

```
streamcapture/
├── provider.go          # StreamProvider interface
├── types.go             # Frame, StreamStats, RTSPConfig
├── rtsp.go              # RTSPStream implementation (main entry)
├── internal/
│   ├── rtsp/
│   │   ├── pipeline.go     # GStreamer pipeline creation/management
│   │   ├── callbacks.go    # appsink callbacks (frame extraction)
│   │   └── reconnect.go    # Exponential backoff reconnection
│   └── warmup/
│       ├── warmup.go       # FPS stability measurement
│       └── stats.go        # FPS statistics calculation
├── examples/
│   ├── simple/             # Basic capture example
│   └── hot-reload/         # FPS hot-reload example
└── cmd/
    └── test-capture/       # Manual testing tool
```

### Concurrency Model

**RTSPStream** manages 3 primary goroutines:

1. **runPipeline**: Monitors GStreamer bus for errors, triggers reconnection
2. **Frame converter**: Converts internal frames to public API frames
3. **Background goroutines** (implicit in GStreamer): appsink callbacks

**Synchronization**:
- `sync.RWMutex`: Protects stream lifecycle state
- `atomic.Uint64/Uint32`: Thread-safe counters (frames, bytes, reconnects)
- `context.Context`: Cancellation propagation for graceful shutdown

## Key Architectural Decisions

### AD-1: Fail-Fast Validation at Construction

**Why**: Catch configuration errors at load time, not runtime

```go
// ✅ Correct: NewRTSPStream validates FPS range
stream, err := NewRTSPStream(cfg)  // Returns error if FPS out of range

// ❌ Incorrect: Would fail at runtime when sending frame
```

**Validation Rules**:
- RTSP URL must not be empty
- FPS must be 0.1-30.0
- Resolution must be valid (512p, 720p, 1080p)
- GStreamer must be available

### AD-2: Non-Blocking Frame Distribution

**Why**: Latency > completeness. Drop frames rather than queue.

```go
// appsink configured with:
appsink.SetProperty("max-buffers", 1)    // Keep only latest frame
appsink.SetProperty("drop", true)        // Drop old frames
```

**Pattern Used**:
```go
select {
case frameChan <- frame:
    // Success
case <-ctx.Done():
    return
// No default case - blocking send is intentional here
}
```

### AD-3: Hot-Reload for FPS Changes

**Why**: 2s interruption vs 5-10s full restart

**Mechanism**: Update GStreamer capsfilter caps dynamically

```go
// Old approach: Stop() + reconfigure + Start() (~5-10s)
// New approach: SetTargetFPS() (~2s)
err := stream.SetTargetFPS(0.5)  // Changes FPS without restart
```

**Rollback on Failure**: If capsfilter update fails, previous FPS is preserved.

### AD-4: Automatic Reconnection with Exponential Backoff

**Why**: Resilience against transient network failures

**Strategy**:
- Initial retry: 1 second
- Exponential backoff: 2x each retry (1s → 2s → 4s → 8s → ...)
- Max interval: 30 seconds
- Max retries: 5

**Implementation**: `internal/rtsp/reconnect.go`

### AD-5: TCP-Only RTSP Protocol

**Why**: Compatibility with go2rtc and NAT/firewall traversal

```go
rtspsrc.SetProperty("protocols", 4)  // 4 = TCP only
```

**Trade-off**: Higher latency vs reliability/compatibility

## Configuration

### RTSPConfig

```go
type RTSPConfig struct {
    URL          string      // RTSP stream URL (required)
    Resolution   Resolution  // Res512p, Res720p, Res1080p
    TargetFPS    float64     // 0.1 - 30.0 Hz
    SourceStream string      // Identifier (e.g., "LQ", "HQ")
}
```

**Resolution Dimensions**:
- `Res512p`: 910x512
- `Res720p`: 1280x720 (default)
- `Res1080p`: 1920x1080

## Frame Format

```go
type Frame struct {
    Seq          uint64      // Monotonic sequence number
    Timestamp    time.Time   // Capture timestamp
    Width        int         // Frame width in pixels
    Height       int         // Frame height in pixels
    Data         []byte      // Raw RGB data (no compression)
    SourceStream string      // Stream identifier
    TraceID      string      // Distributed tracing ID (UUID)
}
```

**Data Format**: RGB (3 bytes per pixel)
- Size (720p): 1280 × 720 × 3 = 2,764,800 bytes (~2.6 MB)
- Size (1080p): 1920 × 1080 × 3 = 6,220,800 bytes (~5.9 MB)

## Usage Patterns

### Basic Stream Capture

```go
// 1. Create stream with fail-fast validation
cfg := streamcapture.RTSPConfig{
    URL:          "rtsp://camera/stream",
    Resolution:   streamcapture.Res720p,
    TargetFPS:    2.0,
    SourceStream: "LQ",
}
stream, err := streamcapture.NewRTSPStream(cfg)
if err != nil {
    // Handle validation error (RTSP URL, FPS, etc.)
}

// 2. Start stream (returns immediately, frames arrive asynchronously)
ctx := context.Background()
frameChan, err := stream.Start(ctx)
if err != nil {
    // Handle GStreamer error
}

// 3. Consume frames
for frame := range frameChan {
    // Process frame (frame.Data is raw RGB)
}

// 4. Stop stream (graceful shutdown)
stream.Stop()
```

### Hot-Reload FPS

```go
// Change FPS without restarting stream
err := stream.SetTargetFPS(0.5)  // 1 frame every 2 seconds
if err != nil {
    // Handle error (stream not running, FPS out of range)
}
```

### Statistics Tracking

```go
stats := stream.Stats()
// stats.FrameCount   - Total frames captured
// stats.FPSTarget    - Configured target FPS
// stats.FPSReal      - Measured real FPS
// stats.LatencyMS    - Time since last frame
// stats.Reconnects   - Reconnection count
// stats.BytesRead    - Total bytes read
// stats.IsConnected  - Connection status
```

## Module Boundaries (Anti-Responsibilities)

**What stream-capture DOES**:
- ✅ Acquire RTSP streams via GStreamer
- ✅ Decode H.264 to raw RGB frames
- ✅ Control FPS with hot-reload
- ✅ Provide thread-safe statistics
- ✅ Automatic reconnection with backoff

**What stream-capture DOES NOT DO**:
- ❌ Frame processing (inference, ROI, etc.) → worker-lifecycle BC
- ❌ Frame distribution to multiple workers → framebus BC
- ❌ MQTT control commands → control-plane BC
- ❌ Event emission → event-emitter BC

## Known Issues / Technical Debt

- **Warm-up function**: `WarmupStream()` exists in `internal/warmup/` but is not used in `Start()`. Production code should call it separately after `Start()` to measure FPS stability.
- **Import cycle avoidance**: `internal/rtsp/callbacks.go` uses `rtsp.Frame` instead of `streamcapture.Frame` to avoid import cycles. Conversion happens in `rtsp.go:171-198`.
- **Dynamic pad linking**: `pad-added` callback for rtspsrc is set up in `rtsp.go:218-224`. If `rtph264depay` element is not found, linking may fail silently.

## Dependencies

```go
require (
    github.com/google/uuid v1.6.0           // Trace ID generation
    github.com/tinyzimmer/go-gst v0.2.33    // GStreamer Go bindings
)
```

**System Dependencies**:
```bash
# Ubuntu/Debian
sudo apt-get install gstreamer1.0-tools \
                     gstreamer1.0-plugins-base \
                     gstreamer1.0-plugins-good \
                     gstreamer1.0-plugins-bad \
                     gstreamer1.0-plugins-ugly \
                     gstreamer1.0-libav

# macOS
brew install gstreamer \
             gst-plugins-base \
             gst-plugins-good \
             gst-plugins-bad \
             gst-plugins-ugly
```

## Testing Strategy

### Manual Testing with test-capture

The `test-capture` binary is the primary testing tool (see `cmd/test-capture/README.md`):

```bash
# Basic capture (no saving)
./bin/test-capture --url rtsp://camera/stream

# Save frames to disk (PNG format)
./bin/test-capture \
  --url rtsp://camera/stream \
  --output ./frames \
  --format png \
  --fps 1.0

# Save frames to disk (JPEG format - smaller)
./bin/test-capture \
  --url rtsp://camera/stream \
  --output ./frames \
  --format jpeg \
  --jpeg-quality 85 \
  --fps 1.0

# Performance benchmarking
./bin/test-capture \
  --url rtsp://camera/stream \
  --fps 10 \
  --max-frames 1000 \
  --stats-interval 5
```

### Acceptance Criteria (Sprint 1.1)

- [x] RTSP stream se captura correctamente
- [x] Reconexión automática en caso de fallo
- [x] FPS se mide durante warm-up (5 segundos)
- [x] Frames se distribuyen sin bloqueo

## Code Patterns

### Thread Safety

```go
// ✅ Atomic operations for counters
atomic.AddUint64(&s.frameCount, 1)
frameCount := atomic.LoadUint64(&s.frameCount)

// ✅ RWMutex for state access
s.mu.RLock()
defer s.mu.RUnlock()
// Read-only access

s.mu.Lock()
defer s.mu.Unlock()
// Write access
```

### Error Handling

```go
// ✅ Fail-fast at construction
stream, err := NewRTSPStream(cfg)
if err != nil {
    return fmt.Errorf("stream-capture: %w", err)
}

// ✅ Graceful degradation on runtime errors
if err := stream.SetTargetFPS(fps); err != nil {
    slog.Error("failed to update FPS", "error", err)
    // Continue with old FPS
}
```

### Logging Standards

```go
// Use structured logging with slog
slog.Info("stream-capture: event occurred",
    "url", rtspURL,
    "resolution", fmt.Sprintf("%dx%d", width, height),
    "target_fps", targetFPS,
)

// Prefix all logs with "stream-capture:" for BC identification
```

## Integration with Orion 2.0

### Multi-Module Monorepo Context

This module lives in:
```
OrionWork/
├── go.work                      # Workspace declaration
└── modules/
    └── stream-capture/          # This module
        ├── go.mod
        ├── CLAUDE.md            # This file
        └── ...
```

**Import Path**: `github.com/e7canasta/orion-care-sensor/modules/stream-capture`

### Future Integration Points

- **worker-lifecycle BC**: Will consume frames from `StreamProvider`
- **framebus BC**: Will distribute frames to multiple workers
- **control-plane BC**: Will send hot-reload commands (FPS changes)

## Commit Standards

- Co-authored by: `Gaby de Visiona <noreply@visiona.app>`
- Do NOT include "Generated with Claude Code" footer
- Focus on "why" rather than "what" in commit messages

## References

- **Test Tool**: `cmd/test-capture/README.md` - Comprehensive testing guide
- **Parent CLAUDE.md**: `/home/visiona/Work/OrionWork/CLAUDE.md` - Orion-wide guidance
- **Global CLAUDE.md**: `/home/visiona/.claude/CLAUDE.md` - Visiona team practices
