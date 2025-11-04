# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**stream-capture** is a bounded context module within Orion 2.0 (Sprint 1.1) that provides high-performance RTSP video stream acquisition via GStreamer. It operates as a self-contained Go library with a focus on reliability, low latency, and hot-reload capabilities.

### Core Philosophy

- **Fail Fast**: Validate configuration at construction time, not runtime
- **Non-blocking Channels**: Drop frames rather than queue to maintain <2s latency
- **Complexity by Design**: Attack complexity through architecture (reconnection, warmup), not complicated code
- **Thread Safety**: Extensive use of atomic operations and mutexes for concurrent access

### Technology Stack

- **Go 1.21**: Module implementation
- **GStreamer 1.0**: RTSP streaming, H.264 decoding, frame processing
- **go-gst bindings**: GStreamer integration via github.com/tinyzimmer/go-gst

## Development Commands

### Building

```bash
# Build module library (compile-only check)
make build

# Build test-capture binary (primary testing tool)
make test-capture

# Build example binaries
make examples

# Build everything
make all
```

### Testing

**Testing Philosophy**: Manual testing with pair-programming approach. Run tests manually, Claude observes and reviews.

```bash
# Run unit tests (always verify compilation)
make test

# Run test-capture (requires RTSP stream)
RTSP_URL=rtsp://camera/stream make run-test

# With custom parameters
RTSP_URL=rtsp://camera/stream OUTPUT_DIR=./frames FPS=1.0 RES=720p make run-test

# Debug mode
RTSP_URL=rtsp://camera/stream DEBUG=1 make run-test
```

### Linting & Code Quality

```bash
# Format code
make fmt

# Run go vet
make vet

# Run both
make lint

# Quick development check (format + build + test)
make dev
```

## Architecture: The Big Picture

### Core Design Pattern

**GStreamer Pipeline with Non-Blocking Distribution**

```
RTSP Camera → GStreamer Pipeline → Frame Callbacks → Internal Channel
                                                              ↓
                                                    Frame Converter Goroutine
                                                              ↓
                                                    Public Channel (non-blocking)
                                                              ↓
                                                    Consumer (e.g., FrameBus)
```

### Key Components

**Public API** (`provider.go`, `types.go`)
- `StreamProvider` interface: Contract for stream acquisition
- `RTSPConfig`, `Frame`, `StreamStats`, `WarmupStats`: Core types
- Resolution constants: `Res512p`, `Res720p`, `Res1080p`

**RTSP Stream Implementation** (`rtsp.go`)
- `RTSPStream`: Main implementation of `StreamProvider`
- Lifecycle: `NewRTSPStream()` → `Start()` → `Warmup()` → consume frames → `Stop()`
- Hot-reload: `SetTargetFPS()` updates GStreamer caps without restart (~2s interruption)
- Thread-safe statistics: Atomic counters for frame count, drops, bytes read

**GStreamer Pipeline** (`internal/rtsp/pipeline.go`)
- Pipeline structure: `rtspsrc → rtph264depay → avdec_h264 → videoconvert → videoscale → videorate → capsfilter → appsink`
- Framerate control: Capsfilter with fractional framerate support (0.1-30 Hz)
- Hot-reload support: `UpdateFramerateCaps()` for dynamic FPS changes

**Reconnection Logic** (`internal/rtsp/reconnect.go`)
- Exponential backoff: 1s → 2s → 4s → 8s → 16s (max 5 retries)
- Auto-recovery: Detects pipeline errors, attempts reconnection
- State tracking: Resets retry counter on successful connection

**Warmup System** (`internal/warmup/warmup.go`)
- FPS stability measurement over configurable duration (default: 5 seconds)
- Statistics: Mean, StdDev, Min, Max FPS
- Stability threshold: StdDev < 15% of mean

**GStreamer Callbacks** (`internal/rtsp/callbacks.go`)
- `OnNewSample`: Extracts frame data from GStreamer appsink
- `OnPadAdded`: Links rtspsrc dynamic pads to rtph264depay

### Lifecycle Flow

1. **Construction** (`NewRTSPStream`): Fail-fast validation (URL, FPS, resolution, GStreamer)
2. **Start** (`Start`): Create pipeline → Start in PLAYING state → Launch goroutines → Return frame channel immediately (non-blocking)
3. **Warmup** (`Warmup`): Consume frames for 5 seconds → Measure FPS stability → Return statistics
4. **Consume**: Read frames from channel → Process (external to module)
5. **Stop** (`Stop`): Cancel context → Wait for goroutines (3s timeout) → Destroy pipeline → Close channel (protected against double-close)

### Goroutine Architecture

**3 goroutines per stream**:

1. **Frame Converter** (`rtsp.go:174-211`): Converts internal `rtsp.Frame` to public `streamcapture.Frame`, sends to public channel (non-blocking with drop tracking)
2. **Pipeline Monitor** (`rtsp.go:254-263`): Monitors GStreamer bus for errors, triggers reconnection
3. **Reconnection Manager** (`rtsp.go:273-294`): Exponential backoff retry logic

### Non-Blocking Channel Strategy

**Drop Policy** (rtsp.go:196-209):
```go
select {
case s.frames <- publicFrame:
    // Success
case <-ctx.Done():
    return
default:
    // Channel full - drop frame and track metric
    atomic.AddUint64(&s.framesDropped, 1)
}
```

**Why**: Latency > completeness. Prefer dropping frames over queuing to maintain <2s latency.

## Configuration

### RTSPConfig

```go
cfg := streamcapture.RTSPConfig{
    URL:          "rtsp://192.168.1.100:8554/stream",
    Resolution:   streamcapture.Res720p,  // 512p, 720p, 1080p
    TargetFPS:    2.0,                     // 0.1 - 30.0 Hz
    SourceStream: "LQ",                    // Identifier (e.g., "LQ", "HQ")
}
```

### Supported Resolutions

- **512p**: 910x512 (custom Orion resolution)
- **720p**: 1280x720 (HD)
- **1080p**: 1920x1080 (Full HD)

### FPS Range

- **Minimum**: 0.1 Hz (1 frame every 10 seconds)
- **Maximum**: 30.0 Hz (30 FPS)
- **Fractional support**: 0.5 Hz → GStreamer framerate=1/2

## Code Patterns and Conventions

### Thread Safety

- **Atomic operations**: For counters (`frameCount`, `framesDropped`, `bytesRead`, `reconnects`)
- **sync.RWMutex**: For shared state (`mu` protects internal state during Start/Stop)
- **sync.WaitGroup**: For goroutine lifecycle management
- **context.Context**: For cancellation propagation

### Error Handling

- **Fail-fast validation**: At construction time (NewRTSPStream)
- **Graceful degradation**: Log errors but continue processing (frame drops, not crashes)
- **Idempotent operations**: Safe to call Stop() multiple times

### Logging

- **slog**: Structured logging with JSON handler support
- **Log levels**: DEBUG, INFO, WARN, ERROR
- **Context fields**: `url`, `resolution`, `target_fps`, `seq`, `trace_id`

### Naming Conventions

- **Public types**: `StreamProvider`, `RTSPStream`, `Frame`, `StreamStats`
- **Internal packages**: `internal/rtsp`, `internal/warmup`
- **Atomic flags**: Use `atomic.Bool` for shutdown protection (e.g., `framesClosed`)

## Key Architectural Decisions

### AD-1: Non-Blocking Channels with Drop Policy
**Why**: Latency > completeness. Prefer dropping frames over queuing to maintain <2s latency.
- Avoids head-of-line blocking
- Predictable, bounded latency
- Drop statistics tracked for observability

### AD-2: Fail-Fast Validation
**Why**: Catch configuration errors at load time, not runtime.
- Validate RTSP URL, FPS range, resolution at construction
- Check GStreamer availability before pipeline creation
- Clear error messages guide user to fix issues

### AD-3: Warmup Phase for FPS Stability
**Why**: GStreamer pipelines take ~3-5 seconds to stabilize.
- Measure real FPS vs target FPS
- Detect unstable streams (high variance)
- Provide visibility to consumers before processing

### AD-4: Exponential Backoff Reconnection
**Why**: Gracefully handle transient network failures.
- 5 retry attempts with exponential backoff (1s → 2s → 4s → 8s → 16s)
- Reset retry counter on successful connection
- Prevents infinite restart loops

### AD-5: Hot-Reload for FPS Changes
**Why**: Avoid full pipeline restart (5-10 seconds) for FPS changes.
- Update GStreamer capsfilter dynamically (~2s interruption)
- Rollback on failure (restore previous FPS)
- Critical for MQTT control plane integration (Orion 2.0)

### AD-6: Double-Close Protection with atomic.Bool
**Why**: Prevent "close of closed channel" panic during shutdown race conditions.
- Use `atomic.Bool` flag (`framesClosed`) with `CompareAndSwap`
- Ensures channel closed exactly once
- Safe for concurrent Stop() calls

## GStreamer Pipeline Details

### Pipeline Elements

```
rtspsrc (location=RTSP_URL, protocols=4 [TCP], latency=200ms)
    ↓
rtph264depay (extracts H.264 NAL units)
    ↓
avdec_h264 (decodes H.264 to raw video)
    ↓
videoconvert (converts to RGB)
    ↓
videoscale (scales to target resolution)
    ↓
videorate (drop-only mode, no frame duplication)
    ↓
capsfilter (caps=video/x-raw,format=RGB,width=W,height=H,framerate=N/D)
    ↓
appsink (sync=false, max-buffers=1, drop=true)
```

### Capsfilter Framerate Format

- **FPS >= 1.0**: `framerate=N/1` (e.g., 5.0 Hz → `5/1`)
- **FPS < 1.0**: `framerate=1/D` (e.g., 0.5 Hz → `1/2`)

### AppSink Configuration

- **sync=false**: No clock synchronization (real-time)
- **max-buffers=1**: Keep only latest frame (drop old frames)
- **drop=true**: Drop frames if consumer is slow

## Testing with test-capture

**Primary testing tool**: `cmd/test-capture/main.go`

### Basic Usage

```bash
# Minimal test (log frames, no saving)
./bin/test-capture --url rtsp://camera/stream

# Save frames to disk (PNG)
./bin/test-capture --url rtsp://camera/stream --output ./frames --format png

# Save frames (JPEG, smaller)
./bin/test-capture --url rtsp://camera/stream --output ./frames --format jpeg --jpeg-quality 85

# Custom FPS and resolution
./bin/test-capture --url rtsp://camera/stream --fps 1.0 --resolution 1080p

# Capture limited frames
./bin/test-capture --url rtsp://camera/stream --max-frames 100

# Debug mode
./bin/test-capture --url rtsp://camera/stream --debug
```

### Key Features

- **Warmup**: Runs 5-second warmup before processing, reports FPS stability
- **Live statistics**: Reports every 10 seconds (configurable with `--stats-interval`)
- **Frame saving**: PNG (lossless, ~960 KB) or JPEG (compressed, ~140 KB for q=90)
- **Graceful shutdown**: Ctrl+C triggers clean shutdown with final statistics

### Example Output

```
╔═══════════════════════════════════════════════════════════╗
║         Stream Capture Test - Orion 2.0 Module           ║
║                      Version v0.1.0                       ║
╚═══════════════════════════════════════════════════════════╝

Configuration:
  RTSP URL:      rtsp://192.168.1.100:8554/stream
  Resolution:    720p
  Target FPS:    2.00
  Source Stream: test
  Output Dir:    ./frames
  Max Frames:    unlimited

Running warmup (5 seconds) to measure stream stability...

╭─────────────────────────────────────────────────────────╮
│ Warmup Complete
├─────────────────────────────────────────────────────────┤
│ Frames Received:        10 frames
│ Duration:              5.0 seconds
│ FPS Mean:              2.01 fps
│ FPS StdDev:            0.12 fps
│ FPS Range:             1.9 - 2.1 fps
│ Stable:                true
╰─────────────────────────────────────────────────────────╯

Starting frame capture...
Press Ctrl+C to stop gracefully
═══════════════════════════════════════════════════════════

[12:34:56] Frame #1      | Seq: 1        | Size:  123.4 KB | Timestamp: 12:34:56.123
[12:34:57] Frame #2      | Seq: 2        | Size:  124.1 KB | Timestamp: 12:34:57.123
```

## Examples

### examples/simple

Basic frame capture with live statistics. No frame saving.

```bash
make run-simple
# or
RTSP_URL=rtsp://camera/stream ./bin/simple --url $RTSP_URL
```

### examples/hot-reload

Interactive example demonstrating hot-reload of FPS without restart.

```bash
make run-hotreload
# or
RTSP_URL=rtsp://camera/stream ./bin/hot-reload --url $RTSP_URL
```

## Integration with Orion 2.0

### Module Position in Orion 2.0 Architecture

- **Bounded Context**: Stream Acquisition
- **Sprint**: 1.1 (Foundation Phase)
- **Dependencies**: None (standalone module)
- **Dependents**: `framebus` (Sprint 1.2), `core` (Sprint 3)

### Future Integration Points

- **FrameBus**: Non-blocking fan-out to multiple workers
- **Control Plane**: MQTT hot-reload commands (change FPS, resolution)
- **Event Emitter**: Publish frame metadata to MQTT

### Multi-Module Monorepo Context

Located at: `OrionWork/modules/stream-capture/`

Workspace: `OrionWork/go.work` (Go 1.18+ workspace)

Related modules:
- `worker-lifecycle/` (Sprint 1.2)
- `framebus/` (Sprint 1.2)
- `control-plane/` (Sprint 2)
- `core/` (Sprint 3)

## Known Issues / Technical Debt

- **No README.md at module root**: Only in cmd/test-capture (acceptable for now)
- **Legacy binaries in root**: `simple_capture`, `hot-reload` (cleaned up via .gitignore)
- **Frame format**: Currently RGB (3 bytes/pixel), may change to JPEG for efficiency
- **Single-stream only**: No multi-stream support yet (planned for v2.0)

## Dependencies

### External

- `github.com/tinyzimmer/go-gst v0.2.33`: GStreamer bindings
- `github.com/google/uuid v1.6.0`: Trace ID generation

### System Requirements

- **GStreamer 1.0**: Core framework
- **gst-plugins-base**: Basic elements (videoconvert, videoscale)
- **gst-plugins-good**: RTSP client (rtspsrc)
- **gst-plugins-bad**: Video rate control (videorate)
- **gst-plugins-ugly**: H.264 decoder (avdec_h264)
- **gst-libav**: Additional codecs

### Installation

```bash
# Ubuntu/Debian
sudo apt-get install gstreamer1.0-tools gstreamer1.0-plugins-base \
                     gstreamer1.0-plugins-good gstreamer1.0-plugins-bad \
                     gstreamer1.0-plugins-ugly gstreamer1.0-libav

# macOS
brew install gstreamer gst-plugins-base gst-plugins-good \
             gst-plugins-bad gst-plugins-ugly
```

## Common Development Tasks

### Adding a New Resolution

1. Add constant to `types.go`: `Res4K Resolution = iota`
2. Update `Dimensions()` method: `case Res4K: return 3840, 2160`
3. Update `String()` method: `case Res4K: return "4K"`
4. Update test-capture flag validation in `cmd/test-capture/main.go`

### Modifying Pipeline Elements

1. Update `CreatePipeline()` in `internal/rtsp/pipeline.go`
2. Add new elements to `PipelineElements` struct if needed for hot-reload
3. Test with `make test-capture`

### Changing Drop Policy

1. Modify `select` statement in `rtsp.go:196-209`
2. Update `StreamStats` if new metrics added
3. Update test-capture statistics reporting

## Troubleshooting

### "GStreamer not available"

**Cause**: GStreamer not installed or not in PATH.

**Solution**: Install GStreamer (see Dependencies section).

### "Failed to create pipeline"

**Cause**: Missing GStreamer plugins (RTSP, H.264).

**Solution**: `sudo apt-get install gstreamer1.0-plugins-ugly gstreamer1.0-libav`

### High Frame Drop Rate

**Cause**: Consumer too slow (e.g., disk I/O bottleneck).

**Solutions**:
- Increase channel buffer size (currently 10)
- Reduce FPS
- Use faster storage (SSD)
- Don't save frames to disk

### Pipeline Errors in Logs

**Cause**: Incompatible stream format, missing codecs.

**Solution**: Check GStreamer pipeline manually:
```bash
gst-launch-1.0 rtspsrc location=rtsp://camera/stream protocols=4 latency=200 ! \
    rtph264depay ! avdec_h264 ! videoconvert ! autovideosink
```

## Commit Standards

- Co-authored by: `Gaby de Visiona <noreply@visiona.app>`
- Do NOT include "Generated with Claude Code" footer (implicit in co-author)
- Focus on "why" rather than "what" in commit messages
- Follow existing commit style (see `git log`)

## Development Workflow

### When Adding New Features

1. **Understand the Big Picture**: Review module architecture (this file)
2. **Complexity by Design**: Attack complexity through architecture, not code tricks
3. **Fail Fast**: Validate at load time, not runtime
4. **Cohesion > Location**: Modules defined by conceptual cohesion, not size
5. **One Reason to Change**: Each component has a single responsibility (SRP)

### Code Review Standards

- "Simple para leer, NO simple para escribir una vez"
- Clean design ≠ simplistic design
- Modularity reduces complexity when applied correctly
- Document architectural decisions (ADR style, add to this file)
