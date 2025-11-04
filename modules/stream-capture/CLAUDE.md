# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Module Overview

**stream-capture** is a bounded context within Orion 2.0 responsible for video stream acquisition from RTSP sources. It provides a clean, production-ready Go interface (`StreamProvider`) wrapping GStreamer pipelines with hardware acceleration support (VAAPI), automatic reconnection, and hot-reload capabilities.

**Key Responsibilities:**
- RTSP stream acquisition with H.264 decode (hardware/software)
- Frame rate control with hot-reload (no restart required)
- Automatic reconnection with exponential backoff
- Hardware acceleration (Intel VAAPI) with fallback to software decode
- Frame distribution with non-blocking drop semantics
- Telemetry: FPS stability, decode latency (P95/mean/max), error categorization

**Design Philosophy:**
- **Fail-fast validation**: All configuration validated at construction time
- **Latency > Completeness**: Drop frames to maintain <2s latency (non-blocking channels)
- **Graceful degradation**: Single frame failures don't kill the pipeline
- **Lock-free telemetry**: Atomic operations for counters, atomic.Pointer for latency tracking
- **Separation of concerns**: Go for orchestration/concurrency, GStreamer for multimedia

## Quick Start (5 Minutes)

**Minimal production-ready example**:

```go
cfg := streamcapture.RTSPConfig{
    URL:          "rtsp://camera/stream",
    Resolution:   streamcapture.Res720p,
    TargetFPS:    1.0,
    Acceleration: streamcapture.AccelAuto,  // Try VAAPI, fallback to software
}

stream, err := streamcapture.NewRTSPStream(cfg)
if err != nil {
    log.Fatal("Failed to create stream:", err)  // Fail-fast validation
}

ctx := context.Background()
frameChan, err := stream.Start(ctx)
if err != nil {
    log.Fatal("Failed to start stream:", err)
}

// ⚠️ REQUIRED: Validate stability before production use
// Warmup enforces fail-fast - returns error if stream is unstable
stats, err := stream.Warmup(ctx, 5*time.Second)
if err != nil {
    log.Fatal("Stream unstable:", err)  // DO NOT proceed to production
}
log.Printf("✅ Stream stable - FPS: %.2f Hz, Jitter: %.3fs", stats.FPSMean, stats.JitterMean)

// ✅ Safe to process frames
for frame := range frameChan {
    // Process frame...
}
```

**Full example**: See `cmd/test-capture/main.go` for production-grade implementation with telemetry, error handling, and graceful shutdown.

**Note**: Examples in `examples/` directory may skip warmup for brevity, but **production code MUST call `Warmup()`** to validate stream stability.

## Development Commands

### Building

```bash
# Build module library
make build

# Build test-capture binary
make test-capture

# Build examples
make examples

# Build everything
make all
```

### Testing

```bash
# Run unit tests
make test

# Run unit tests with race detector
make test-verbose

# Run test-capture binary (requires RTSP_URL)
RTSP_URL=rtsp://camera/stream make run-test

# Run with custom parameters
RTSP_URL=rtsp://camera/stream OUTPUT_DIR=./frames FPS=1.0 RES=720p make run-test

# Run with VAAPI hardware acceleration
RTSP_URL=rtsp://camera/stream ACCEL=vaapi make run-test

# Run with debug logging
RTSP_URL=rtsp://camera/stream DEBUG=1 make run-test

# Capture limited frames for inspection
RTSP_URL=rtsp://camera/stream OUTPUT_DIR=./inspection FORMAT=jpeg MAX_FRAMES=3 make run-test
```

### Code Quality

```bash
# Format code
make fmt

# Run go vet
make vet

# Lint (fmt + vet)
make lint

# Development workflow (lint + build + test)
make dev
```

### Cleanup

```bash
# Remove build artifacts
make clean
```

## Architecture: The Big Picture

### Public API (`StreamProvider` interface)

The module exports a single interface defining the contract for video stream acquisition:

```go
type StreamProvider interface {
    Start(ctx context.Context) (<-chan Frame, error)  // Non-blocking, returns immediately
    Stop() error                                       // Graceful shutdown (idempotent)
    Stats() StreamStats                                // Thread-safe statistics
    SetTargetFPS(fps float64) error                    // Hot-reload FPS (~2s interruption)
    Warmup(ctx context.Context, duration time.Duration) (*WarmupStats, error)
}
```

**Key guarantees:**
- `Start()` is non-blocking (frames arrive asynchronously after ~3s GStreamer pipeline startup)
- `Stop()` is idempotent (safe to call multiple times)
- `Stats()` is thread-safe (atomic operations)
- `SetTargetFPS()` triggers hot-reload without restart (rollback on failure)
- `Warmup()` measures FPS stability (recommended after `Start()` before production use)

### Implementation: `RTSPStream`

**File: `rtsp.go`**

Implements `StreamProvider` using GStreamer for RTSP streaming with:
- Atomic counters for statistics (`frameCount`, `framesDropped`, `bytesRead`)
- `sync.RWMutex` for pipeline state protection
- `atomic.Pointer[LatencyWindow]` for lock-free decode latency tracking
- Reconnection state with exponential backoff

**Lifecycle:**
1. **Construction** (`NewRTSPStream`): Fail-fast validation (URL, FPS, resolution, GStreamer availability)
2. **Start** (`Start`): Create GStreamer pipeline, launch background goroutines (frame converter, pipeline monitor)
3. **Operation**: Asynchronous frame delivery via channel, automatic reconnection on errors
4. **Hot-reload** (`SetTargetFPS`): Update GStreamer capsfilter, rollback on failure
5. **Stop** (`Stop`): Cancel context, wait for goroutines (3s timeout), destroy pipeline, close channel (double-close protection)

### GStreamer Pipeline Architecture

**File: `internal/rtsp/pipeline.go`**

Two pipeline variants based on hardware acceleration:

**VAAPI Pipeline (Hardware Acceleration):**
```
rtspsrc → rtph264depay → vaapih264dec → vaapipostproc → videoconvert → capsRGB → videorate → capsfilter → appsink
          (TCP mode)     (GPU decode)   (GPU scale)      (NV12→RGB)    (format lock) (FPS ctrl)  (final caps)
```

**Software Pipeline (CPU Decode):**
```
rtspsrc → rtph264depay → avdec_h264 → videoconvert → videoscale → videorate → capsfilter → appsink
          (TCP mode)     (CPU decode)  (YUV→RGB)     (CPU scale)   (FPS ctrl)  (final caps)
```

**Key Optimizations:**
- VAAPI: GPU scaling in `vaapipostproc` (removes CPU `videoscale`)
- H.264-specific decoder (`vaapih264dec`) vs generic (`vaapidecodebin`)
- Low-latency mode (`low-latency=true`) for H.264 Main profile (no B-frames)
- Adaptive latency buffer: 50ms for low FPS (≤2fps), 200ms for high FPS
- QoS events: Upstream frame dropping before decode (when `appsink` drops frames)
- Multi-threaded conversion: `n-threads=0` (auto-detect cores)

**Optimization Levels** (internal code annotations in `pipeline.go`):

Code comments use "OPTIMIZATION Level N" to document the optimization hierarchy:

- **Level 1: Core Performance** - Fundamental optimizations that provide the biggest wins
  - H.264-specific decoder (vaapih264dec vs vaapidecodebin) - eliminates codec probing overhead
  - GPU scaling in vaapipostproc - removes CPU videoscale element entirely
  - RGB format lock (capsRGB) - prevents caps negotiation between pipeline stages

- **Level 2: FPS-Aware Tuning** - Optimizations that adapt to configured target FPS
  - Adaptive latency buffer (50ms for ≤2fps, 200ms otherwise) - reduces buffering for low FPS
  - Keyframe recovery (request-keyframe=true) - faster recovery from packet loss
  - Immediate drop mode for videorate (no averaging window) - deterministic frame selection

- **Level 3: Advanced Tuning** - Fine-grained optimizations with marginal gains
  - Multi-threaded YUV→RGB conversion (n-threads=0) - parallel processing on multi-core
  - QoS events (qos=true) - upstream frame dropping before decode
  - Skip corrupt frames (output-corrupt=false) - prevents garbage frames at low FPS

These levels guide future optimizations without breaking existing patterns. When modifying pipeline.go, maintain this hierarchy (Level 1 > Level 2 > Level 3) to preserve the optimization rationale.

### Reconnection Strategy

**File: `internal/rtsp/reconnect.go`**

Exponential backoff with configurable parameters:

**Default Configuration:**
- Max retries: 5 attempts
- Initial delay: 1 second
- Max delay: 30 seconds

**Backoff Schedule:**
```
Attempt 1: 1s
Attempt 2: 2s
Attempt 3: 4s
Attempt 4: 8s
Attempt 5: 16s
After 5 failures: Stop (max retries exceeded)
```

**Implementation:**
- `RunWithReconnect()`: Wraps connection function with retry logic
- `ReconnectState`: Tracks attempt count and total reconnections (atomic counter)
- Reset on success: Reconnect counter resets when pipeline reaches PLAYING state

### Frame Processing Flow

**File: `internal/rtsp/callbacks.go`**

GStreamer callbacks for frame delivery:

**OnNewSample (appsink callback):**
1. Pull sample from appsink
2. Map buffer to read pixel data (RGB format from GStreamer)
3. Capture decode latency telemetry (retrieve timestamp from buffer metadata)
4. Copy data (GStreamer will reuse buffer)
5. Update atomic counters (`frameCount`, `bytesRead`)
6. Send frame to channel (non-blocking - drop if full, track `framesDropped`)

**OnPadAdded (rtspsrc dynamic pads):**
- Links rtspsrc output pad to rtph264depay input pad when pad appears

**Decode Latency Tracking (VAAPI only):**
- GstPadProbe on decoder output captures timestamp when buffer exits GPU decoder
- Timestamp stored as ReferenceTimestampMeta on buffer
- OnNewSample retrieves timestamp and calculates: `latency = time.Now() - decodeExitTime`
- Measures post-processing pipeline latency (decode → RGB conversion → videoscale → videorate → appsink)
- Lock-free via `atomic.Pointer[LatencyWindow]` (ring buffer of 100 samples)

### Warmup & FPS Stability

**File: `warmup_stats.go`, `internal/warmup/warmup.go`**

**⚠️ CRITICAL**: `Warmup()` enforces fail-fast validation - it returns an **error** if the stream is unstable. This prevents production use of unreliable streams.

```go
stream, _ := streamcapture.NewRTSPStream(cfg)
frameChan, _ := stream.Start(ctx)

// REQUIRED for production: Warmup validates stream stability (fail-fast)
stats, err := stream.Warmup(ctx, 5*time.Second)
if err != nil {
    // Stream is unstable - DO NOT proceed to production
    // Error message includes FPS mean/stddev for diagnosis
    log.Fatal("warmup failed:", err)
}

// ✅ Stream validated - safe to process frames
log.Printf("Stream stable - FPS: %.2f ± %.2f Hz", stats.FPSMean, stats.FPSStdDev)
```

**When to skip Warmup:**
- Testing/development only (use `--skip-warmup` flag in `cmd/test-capture`)
- Mock streams with known stability
- **NEVER skip in production** - unstable streams cause unpredictable inference timing

**Stability Criteria (both must pass):**
1. **FPS stability**: stddev < 15% of mean
2. **Jitter stability**: mean jitter < 20% of expected interval
3. **Minimum frames**: At least 2 frames received during warmup

**Jitter Telemetry:**

**Jitter** measures inter-frame interval variance - critical for low-FPS inference timing.

- **Calculation**: `jitter = |actual_interval - expected_interval|`
- **Expected interval**: `1 / target_fps` (e.g., 1.0 Hz → 1.0 second expected)
- **Example**: At 1 Hz, jitter > 200ms indicates network congestion or camera issues

**Use case**: Low-FPS inference (≤2 Hz) is sensitive to timing variance. High jitter causes:
- Missed inference windows (frame arrives late)
- Bursty processing (multiple frames arrive together)
- Unpredictable system load

**Metrics:**
- **FPS**: mean, stddev, min, max (overall rate stability)
- **Jitter**: mean, stddev, max (timing consistency)
- **Duration**: actual warmup duration
- **IsStable**: boolean (`true` only if both FPS and jitter pass criteria)

## Key Types

### Frame

```go
type Frame struct {
    Seq          uint64        // Monotonic sequence number
    Timestamp    time.Time     // Capture/decode timestamp
    Width        int           // Frame width in pixels
    Height       int           // Frame height in pixels
    Data         []byte        // Raw RGB data (no alpha channel)
    SourceStream string        // Stream identifier (e.g., "LQ", "HQ")
    TraceID      string        // UUID for distributed tracing
}
```

### StreamStats

```go
type StreamStats struct {
    FrameCount          uint64  // Total frames captured
    FramesDropped       uint64  // Frames dropped (channel full)
    DropRate            float64 // Drop rate percentage (0-100)
    FPSTarget           float64 // Configured target FPS
    FPSReal             float64 // Measured real FPS
    LatencyMS           int64   // Time since last frame (ms)
    Reconnects          uint32  // Reconnection attempts
    BytesRead           uint64  // Total bytes read
    IsConnected         bool    // Connection status
    ErrorsNetwork       uint64  // Network errors (connection, timeout)
    ErrorsCodec         uint64  // Codec errors (decode failures)
    ErrorsAuth          uint64  // Auth errors
    ErrorsUnknown       uint64  // Unclassified errors
    DecodeLatencyMeanMS float64 // Mean decode latency (VAAPI only)
    DecodeLatencyP95MS  float64 // P95 decode latency (VAAPI only)
    DecodeLatencyMaxMS  float64 // Max decode latency (VAAPI only)
    UsingVAAPI          bool    // VAAPI hardware acceleration active
}
```

### RTSPConfig

```go
type RTSPConfig struct {
    URL                   string          // RTSP stream URL (required)
    Resolution            Resolution      // Video resolution (512p, 720p, 1080p)
    TargetFPS             float64         // Target FPS (0.1 - 30.0)
    SourceStream          string          // Stream identifier
    MaxReconnectAttempts  int             // Max reconnection attempts (default: 5)
    ReconnectInitialDelay time.Duration   // Initial retry delay (default: 1s)
    ReconnectMaxDelay     time.Duration   // Max retry delay (default: 30s)
    Acceleration          HardwareAccel   // Acceleration mode (auto/vaapi/software)
}
```

### HardwareAccel

```go
type HardwareAccel int

const (
    AccelAuto     HardwareAccel = iota  // Try VAAPI, fallback to software (recommended)
    AccelVAAPI                           // Force VAAPI, fail-fast if unavailable
    AccelSoftware                        // Force software decode (CPU)
)
```

## Error Telemetry & Diagnostics

**File: `internal/rtsp/errors.go`**

`StreamStats` includes categorized error counters for diagnostics. GStreamer errors are automatically classified into four categories:

### Error Categories with Actionable Remediation

| Error Category | Typical Causes | Detection Pattern | Remediation |
|---------------|----------------|-------------------|-------------|
| **ErrorsNetwork** | Connection timeout, DNS failure, firewall, NAT traversal | "Could not connect", "Timeout", "No route to host" | • Verify RTSP URL is reachable: `curl -v rtsp://camera/stream`<br>• Check network connectivity and firewall rules<br>• Test with TCP-only mode (already default)<br>• Increase reconnection timeout if intermittent |
| **ErrorsCodec** | Corrupt H.264 stream, unsupported codec, bandwidth saturation | "Failed to decode", "Invalid NAL unit", "Stream error" | • Check camera encoder settings (H.264 baseline/main profile)<br>• Verify network bandwidth (1080p@30fps ≈ 4 Mbps)<br>• Enable debug logging to see GStreamer codec details<br>• Try lower resolution (720p instead of 1080p) |
| **ErrorsAuth** | Invalid credentials, RTSP 401/403 | "Unauthorized", "Forbidden", "Authentication failed" | • Verify camera username/password in RTSP URL<br>• Check camera allows RTSP access (may be disabled)<br>• Test with rtsp-simple-server for local debugging |
| **ErrorsUnknown** | GStreamer internal errors, hardware issues | All other GStreamer errors | • Enable debug logging: `--debug` flag<br>• Check GStreamer version: `gst-inspect-1.0 --version`<br>• Review GStreamer logs for plugin issues<br>• Try software decode to rule out VAAPI issues |

### Monitoring Pattern (Alerting Thresholds)

```go
ticker := time.NewTicker(1 * time.Minute)
for range ticker.C {
    stats := stream.Stats()

    // Alert on high network error rate (>10/min indicates persistent connectivity issues)
    if stats.ErrorsNetwork > 10 {
        log.Warn("High network error rate - check connectivity",
            "errors", stats.ErrorsNetwork,
            "reconnects", stats.Reconnects)
        // Consider: Extend reconnection backoff, alert ops team
    }

    // Alert on codec errors (>5/min indicates stream quality issues)
    if stats.ErrorsCodec > 5 {
        log.Warn("High codec error rate - check camera encoder",
            "errors", stats.ErrorsCodec,
            "resolution", stats.Resolution)
        // Consider: Fallback to lower resolution, restart camera
    }

    // Auth errors should be rare (camera config issue)
    if stats.ErrorsAuth > 0 {
        log.Error("Authentication errors detected - check credentials",
            "errors", stats.ErrorsAuth)
        // Action: Stop stream, fix credentials, restart
    }
}
```

### Error Classification Logic

**Implementation**: `internal/rtsp/errors.go` uses regex patterns to classify GStreamer error messages:

- **Network**: `could not connect|timeout|not found|no route|unreachable`
- **Codec**: `decode|stream|format|codec|nal unit`
- **Auth**: `401|403|unauthorized|forbidden|authentication`
- **Unknown**: All other errors (fallback category)

**Thread-safety**: Error counters use `atomic.AddUint64()` for lock-free increments in GStreamer callback context.

## Configuration Patterns

### Production Deployment

```go
cfg := streamcapture.RTSPConfig{
    URL:          "rtsp://camera.local/stream",
    Resolution:   streamcapture.Res720p,
    TargetFPS:    1.0,
    SourceStream: "camera-01",
    Acceleration: streamcapture.AccelAuto,  // Try VAAPI, fallback to software
}
```

### Hardware Acceleration Required

```go
cfg := streamcapture.RTSPConfig{
    URL:          "rtsp://camera.local/stream",
    Resolution:   streamcapture.Res1080p,
    TargetFPS:    5.0,
    Acceleration: streamcapture.AccelVAAPI,  // Fail-fast if VAAPI unavailable
}
```

### Testing/Development (Software Only)

```go
cfg := streamcapture.RTSPConfig{
    URL:          "rtsp://127.0.0.1:8554/test",
    Resolution:   streamcapture.Res512p,
    TargetFPS:    2.0,
    Acceleration: streamcapture.AccelSoftware,  // Force CPU decode (max compatibility)
}
```

## Error Handling Patterns

### Fail-Fast at Construction

All configuration errors are caught at construction time (NewRTSPStream):

```go
stream, err := streamcapture.NewRTSPStream(cfg)
if err != nil {
    // Handle error (invalid config, GStreamer unavailable, VAAPI unavailable)
    log.Fatal("Failed to create stream:", err)
}
```

**Validated at construction:**
- RTSP URL not empty
- Target FPS in range (0.1 - 30.0)
- Resolution valid
- GStreamer availability
- VAAPI availability (if `AccelVAAPI` forced)

### Graceful Degradation at Runtime

Single frame failures don't kill the pipeline:

```go
// Inside OnNewSample callback
sample := sink.PullSample()
if sample == nil {
    slog.Warn("rtsp: failed to pull sample, skipping frame")
    return gst.FlowOK  // Continue processing
}
```

### Automatic Reconnection

Pipeline errors trigger automatic reconnection with exponential backoff:

```go
// Inside runPipeline goroutine
err := rtsp.RunWithReconnect(ctx, s.monitorPipeline, s.reconnectCfg, s.reconnectState)
if err != nil {
    slog.Error("pipeline stopped after reconnection failure", "error", err)
}
```

## Thread Safety

### Atomic Operations

Counters use `sync/atomic` for lock-free reads/writes:

```go
atomic.AddUint64(&s.frameCount, 1)
atomic.AddUint64(&s.framesDropped, 1)
frameCount := atomic.LoadUint64(&s.frameCount)
```

### Locks

`sync.RWMutex` protects pipeline state during Start/Stop/SetTargetFPS:

```go
s.mu.Lock()
defer s.mu.Unlock()
// ... modify pipeline state
```

### Lock-Free Telemetry

Decode latency tracking uses `atomic.Pointer[LatencyWindow]`:

```go
window := s.decodeLatencies.Load()  // Lock-free read
newWindow := *window
newWindow.AddSample(latencyMS)
s.decodeLatencies.Store(&newWindow)  // Lock-free write
```

## Troubleshooting Guide

### Issue: "VAAPI not available" error with `AccelVAAPI`

**Symptom**: `NewRTSPStream()` fails with:
```
stream-capture: VAAPI not available: vaapidecodebin not available (install gstreamer1.0-vaapi)
```

**Root Cause**: Missing GStreamer VAAPI plugins or incompatible hardware.

**Diagnosis**:
```bash
# Check if VAAPI driver is loaded
vainfo
# Expected output: iHD driver with supported profiles

# Check if GStreamer VAAPI plugins are installed
gst-inspect-1.0 vaapidecodebin
gst-inspect-1.0 vaapih264dec
# Expected: Plugin details (not "No such element")
```

**Solutions**:
1. **Install VAAPI support** (Ubuntu/Debian):
   ```bash
   sudo apt install gstreamer1.0-vaapi intel-media-va-driver
   ```

2. **Verify hardware compatibility**:
   - Intel: Requires Quick Sync (6th gen+ Core, or Atom C3000)
   - AMD: Not supported (use `AccelSoftware`)
   - VM: VAAPI may not work (use `AccelSoftware`)

3. **Use fallback mode** (recommended):
   ```go
   cfg.Acceleration = streamcapture.AccelAuto  // Try VAAPI, fallback to software
   ```

---

### Issue: Warmup fails with "stream FPS unstable"

**Symptom**: `Warmup()` returns error after 5 seconds:
```
warmup failed: stream-capture: warmup failed - stream FPS unstable (mean=1.85 Hz, stddev=0.45, threshold=15%)
```

**Root Cause**: High network jitter, camera frame rate mismatch, or insufficient warmup duration.

**Diagnosis**:
```bash
# Check actual camera FPS with ffprobe
ffprobe -rtsp_transport tcp rtsp://camera/stream
# Look for: "fps" in video stream info

# Monitor network with tcpdump (look for packet loss/retransmits)
sudo tcpdump -i eth0 host <camera-ip> -c 100
```

**Solutions**:
1. **Verify FPS match**: Camera actual FPS should match `TargetFPS` configuration
   ```go
   // If camera outputs 30 fps but config is 1 fps, videorate may cause jitter
   cfg.TargetFPS = 1.0  // Match camera's actual output rate for best stability
   ```

2. **Increase warmup duration** (for slow-stabilizing streams):
   ```go
   stats, err := stream.Warmup(ctx, 10*time.Second)  // Longer warmup
   ```

3. **Check network quality**:
   - Test with wired connection (avoid WiFi for production)
   - Verify camera is on same subnet (avoid routing hops)
   - Check switch/router QoS settings

4. **Testing only - skip warmup** (NOT for production):
   ```bash
   # test-capture binary supports --skip-warmup flag
   ./bin/test-capture --url rtsp://camera/stream --skip-warmup
   ```

---

### Issue: Pipeline timeout (5s) during `SetTargetFPS()`

**Symptom**: `SetTargetFPS()` returns error:
```
stream-capture: SetTargetFPS timeout after 5 seconds
```

**Root Cause**: GStreamer capsfilter negotiation stalled (rare, usually indicates pipeline state corruption).

**Solutions**:
1. **Restart stream** (safest approach):
   ```go
   stream.Stop()
   time.Sleep(1 * time.Second)
   stream.Start(ctx)
   stream.Warmup(ctx, 5*time.Second)
   ```

2. **Check GStreamer logs** (enable debug):
   ```bash
   GST_DEBUG=3 ./bin/test-capture --url rtsp://camera/stream --debug
   ```

3. **Avoid rapid FPS changes** (wait >5s between calls):
   ```go
   stream.SetTargetFPS(1.0)
   time.Sleep(6 * time.Second)  // Wait for caps negotiation + safety margin
   stream.SetTargetFPS(2.0)
   ```

---

### Issue: High frame drop rate (>10%)

**Symptom**: `StreamStats.DropRate` consistently above 10%:
```go
stats := stream.Stats()
fmt.Printf("Drop rate: %.1f%%\n", stats.DropRate)  // Output: 15.2%
```

**Root Cause**: Consumer processing frames too slowly, channel buffer (10 frames) fills up.

**Diagnosis**:
```go
// Add timing instrumentation to consumer
start := time.Now()
for frame := range frameChan {
    processFrame(frame)
    elapsed := time.Since(start)
    if elapsed > time.Second {
        log.Warn("Slow frame processing", "duration", elapsed)
    }
    start = time.Now()
}
```

**Solutions**:
1. **Reduce target FPS**:
   ```go
   cfg.TargetFPS = 0.5  // 1 frame every 2 seconds (more processing time)
   ```

2. **Optimize consumer** (process in separate goroutine):
   ```go
   // Producer: Fast frame consumption
   processChan := make(chan Frame, 100)
   go func() {
       for frame := range frameChan {
           select {
           case processChan <- frame:
           default:
               // Drop at processing stage instead of stream stage
           }
       }
   }()

   // Consumer: Slow processing (doesn't block stream)
   for frame := range processChan {
       doExpensiveInference(frame)  // OK to be slow here
   }
   ```

3. **Increase resolution** (if using 1080p, drops may be bandwidth-related):
   ```go
   cfg.Resolution = streamcapture.Res720p  // Lower bandwidth
   ```

---

### Issue: Double-close panic during shutdown

**Symptom**: Panic during concurrent `Stop()` calls:
```
panic: send on closed channel
```

**Root Cause**: Race condition between `Stop()` and context cancellation (should be impossible with current implementation, but historically occurred).

**Solutions**:
1. **Verify current code** (double-close protection added in rtsp.go:480):
   ```go
   // This should be present in rtsp.go Stop() method
   if s.framesClosed.CompareAndSwap(false, true) {
       close(s.frames)
   }
   ```

2. **Ensure single Stop() caller**:
   ```go
   // Bad: Multiple goroutines calling Stop()
   go stream.Stop()
   go stream.Stop()  // Race condition

   // Good: Single caller with sync
   var once sync.Once
   once.Do(func() { stream.Stop() })
   ```

---

### Issue: Memory leak during long-running streams

**Symptom**: Memory usage grows over time (hours/days).

**Diagnosis**:
```bash
# Monitor memory with pprof
go tool pprof http://localhost:6060/debug/pprof/heap

# Check for goroutine leaks
curl http://localhost:6060/debug/pprof/goroutine?debug=1
```

**Solutions**:
1. **Verify Stop() is called** (goroutines must exit):
   ```go
   defer stream.Stop()  // Always clean up
   ```

2. **Check consumer leaks** (frame channel must be drained):
   ```go
   // Bad: Consumer exits without draining channel
   for frame := range frameChan {
       if someCondition {
           return  // Leak: frameChan not drained
       }
   }

   // Good: Drain channel or call Stop() before exit
   stream.Stop()  // Closes frameChan, no leak
   ```

3. **Check LatencyWindow size** (bounded at 100 samples - should not grow):
   ```go
   // internal/rtsp/callbacks.go:32 - fixed size ring buffer
   Samples [100]float64  // Bounded memory
   ```

---

### Issue: RTSP stream works with VLC but not with stream-capture

**Symptom**: VLC can play stream, but `NewRTSPStream()` or `Start()` fails.

**Root Cause**: Codec mismatch (stream-capture only supports H.264), or RTSP authentication format.

**Diagnosis**:
```bash
# Check codec with ffprobe
ffprobe -rtsp_transport tcp rtsp://camera/stream
# Look for: "Video: h264" (OK) vs "Video: h265" (NOT SUPPORTED)

# Test with gst-launch (same pipeline as stream-capture)
gst-launch-1.0 rtspsrc location=rtsp://camera/stream protocols=4 latency=200 ! \
  rtph264depay ! avdec_h264 ! videoconvert ! autovideosink
```

**Solutions**:
1. **Verify H.264 codec**:
   - Stream must be H.264 (baseline or main profile)
   - H.265/HEVC not supported (requires different decoder)

2. **Check RTSP URL format** (authentication):
   ```go
   // Format: rtsp://username:password@host:port/path
   cfg.URL = "rtsp://admin:12345@192.168.1.100:554/stream1"
   ```

3. **Try UDP transport** (GStreamer limitation - stream-capture uses TCP only):
   - Current implementation: `protocols=4` (TCP only, line rtsp/pipeline.go:61)
   - If camera only supports UDP, modify pipeline.go or use rtsp-simple-server as proxy

## Common Patterns

### Basic Usage (Production)

```go
cfg := streamcapture.RTSPConfig{
    URL:          "rtsp://camera/stream",
    Resolution:   streamcapture.Res720p,
    TargetFPS:    2.0,
    Acceleration: streamcapture.AccelAuto,
}

stream, err := streamcapture.NewRTSPStream(cfg)
if err != nil {
    log.Fatal("Failed to create stream:", err)
}

ctx := context.Background()
frameChan, err := stream.Start(ctx)
if err != nil {
    log.Fatal("Failed to start stream:", err)
}

// ⚠️ REQUIRED for production: Warmup validates stream stability (fail-fast)
stats, err := stream.Warmup(ctx, 5*time.Second)
if err != nil {
    // Stream is unstable - DO NOT proceed to production
    log.Fatal("Warmup failed:", err)
}
log.Printf("✅ Stream validated - FPS: %.2f ± %.2f Hz, Jitter: %.3fs",
    stats.FPSMean, stats.FPSStdDev, stats.JitterMean)

// ✅ Safe to process frames
for frame := range frameChan {
    // Process frame...
    processInference(frame)
}
```

**Note**: Examples in `examples/` directory may skip warmup for simplicity, but **production code MUST call `Warmup()`** to validate stream stability before processing.

### Hot-Reload FPS

```go
// Change FPS without restarting stream (~2s interruption)
err := stream.SetTargetFPS(0.5)  // 1 frame every 2 seconds
if err != nil {
    log.Error("Failed to update FPS:", err)
    // Previous FPS restored on failure (automatic rollback)
}
```

### Statistics Monitoring

```go
ticker := time.NewTicker(10 * time.Second)
for range ticker.C {
    stats := stream.Stats()
    log.Printf("Frames: %d, FPS: %.2f, Drop rate: %.1f%%",
        stats.FrameCount, stats.FPSReal, stats.DropRate)

    if stats.UsingVAAPI {
        log.Printf("VAAPI decode latency - Mean: %.2fms, P95: %.2fms, Max: %.2fms",
            stats.DecodeLatencyMeanMS, stats.DecodeLatencyP95MS, stats.DecodeLatencyMaxMS)
    }
}
```

### Graceful Shutdown

```go
sigChan := make(chan os.Signal, 1)
signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

<-sigChan
log.Println("Shutting down...")

if err := stream.Stop(); err != nil {
    log.Error("Error stopping stream:", err)
}
```

## Dependencies

- **go-gst** (github.com/tinyzimmer/go-gst): GStreamer bindings for Go
- **uuid** (github.com/google/uuid): UUID generation for trace IDs

**System Requirements:**
- GStreamer 1.0+ (gstreamer1.0-tools, gstreamer1.0-plugins-good, gstreamer1.0-plugins-bad)
- VAAPI (optional): gstreamer1.0-vaapi, intel-media-va-driver (for Intel Quick Sync)

## Module Positioning

**This module IS:**
- A production-ready RTSP stream acquisition library
- Part of Orion 2.0 multi-module monorepo (bounded context: Stream Acquisition)
- A building block for video inference pipelines

**This module is NOT:**
- A standalone application (use `cmd/test-capture` for testing)
- A complete video processing system (no inference, no recording)
- A replacement for ffmpeg/opencv (focused on real-time RTSP streams only)

## Anti-Patterns & Alternatives

### ❌ When NOT to Use stream-capture

**Do NOT use this module for:**

1. **Video Recording/Storage**
   - **Why not:** stream-capture drops frames to maintain <2s latency (non-blocking channels). Recordings will have gaps.
   - **Use instead:**
     - `ffmpeg -i rtsp://camera/stream -c copy output.mp4` (no re-encoding, reliable recording)
     - GStreamer filesink with buffering enabled
     - go2rtc with recording mode

2. **Transcoding/Re-encoding**
   - **Why not:** stream-capture outputs raw RGB frames (uncompressed, 1280×720 = 2.7 MB/frame). Not suitable for format conversion.
   - **Use instead:**
     - `ffmpeg -i input.mp4 -c:v libx264 output.mp4` (codec chains)
     - GStreamer encodebin for H.264/H.265 encoding
     - Hardware transcoders (Intel QSV, NVIDIA NVENC)

3. **High-throughput processing** (>30 FPS sustained)
   - **Why not:** stream-capture is optimized for low-FPS inference (0.1-5 Hz) with intentional frame dropping. High-FPS requires different architecture.
   - **Use instead:**
     - GStreamer appsink directly (bypass Go abstraction overhead)
     - OpenCV VideoCapture (if RTSP support sufficient)
     - FFmpeg libraries (libavcodec/libavformat)

4. **Multi-stream aggregation**
   - **Why not:** stream-capture is 1:1 (1 stream → 1 RTSPStream instance). No built-in support for multiple cameras.
   - **Use instead:**
     - Orion 2.0 multi-stream architecture (see FASE_2_SCALE.md in parent repo)
     - GStreamer multiqueue + aggregation elements
     - Multiple RTSPStream instances with coordination layer

5. **Low-level video manipulation** (overlays, filters, effects)
   - **Why not:** stream-capture provides raw frames only. No GStreamer filter access.
   - **Use instead:**
     - GStreamer pipeline with videomixer, textoverlay, etc.
     - OpenCV drawing functions (cv2.rectangle, cv2.putText)
     - FFmpeg filters (drawtext, overlay, scale)

6. **Protocols beyond RTSP** (HTTP, WebRTC, local files)
   - **Why not:** stream-capture is RTSP-only (rtspsrc element). Other protocols require different GStreamer sources.
   - **Use instead:**
     - HTTP/HLS: GStreamer souphttpsrc
     - WebRTC: go2rtc with WebRTC API
     - Local files: GStreamer filesrc + decodebin

### ✅ Ideal Use Cases

stream-capture is **optimized for:**

1. **Low-FPS AI inference** (0.1-5 Hz)
   - Person detection in geriatric monitoring (1 frame/sec)
   - Object tracking with temporal hysteresis
   - Anomaly detection with low sampling rate

2. **Real-time responsiveness** (<2s latency requirement)
   - Fall detection alerts (must trigger within 2 seconds)
   - Intrusion detection (immediate notification)
   - Safety monitoring (real-time vs forensic analysis)

3. **Edge deployment** (resource-constrained devices)
   - Intel NUC with Quick Sync (VAAPI hardware acceleration)
   - Embedded Linux (aarch64) with software decode fallback
   - Fanless mini-PCs with thermal throttling considerations

4. **Hot-reload requirements** (operational flexibility)
   - Change FPS without stream restart (SetTargetFPS)
   - Adjust inference rate based on scene complexity
   - A/B testing of detection parameters in production

5. **Non-blocking frame distribution** (latency over completeness)
   - Drop frames instead of queuing (bounded memory)
   - Predictable latency for SLO compliance
   - Graceful degradation under load

### 🔄 Migration Patterns

**If you outgrow stream-capture limitations:**

- **Need recording?** Add separate recording worker consuming same RTSP stream (parallel path)
- **Need high FPS?** Migrate to GStreamer appsink + cgo (0.5ms overhead vs 2ms in stream-capture)
- **Need multi-stream?** Use Orion 2.0 framebus pattern (fan-out to multiple workers)
- **Need transcoding?** Add GStreamer encodebin after appsink (GPU encoding with VAAPI)

## Known Issues & Design Decisions

**No BACKLOG.md yet**: This module was extracted from Orion 1.0 monolith during Sprint 1.1. Future development tracked in parent repository backlog.

**Double-Close Protection**: `framesClosed atomic.Bool` prevents panic from double-close of frame channel during shutdown race conditions (rtsp.go:480). See [Troubleshooting: Double-close panic](#issue-double-close-panic-during-shutdown) for historical context.

**Decode Latency Tracking**: Only active when VAAPI is enabled (software decode doesn't benefit from GPU latency telemetry). Uses lock-free `atomic.Pointer[LatencyWindow]` with ring buffer (100 samples) for P95/mean/max calculation.

**RGB Format Lock**: `capsRGB` capsfilter forces RGB output before `videorate` to prevent caps negotiation issues between VAAPI pipeline and final capsfilter (pipeline.go:283-295). Without this, GStreamer attempts runtime negotiation which fails with `video/x-raw(memory:VASurface)` → RGB conversion.

**Frame Data Format**: Raw RGB bytes (no alpha channel, unlike RGBA). Consumers must add alpha=255 if converting to `image.RGBA`. See `cmd/test-capture/main.go:357-369` for conversion example.

**Test Philosophy**: Manual testing with pair-programming approach. The user runs tests manually while Claude observes and reviews. Always verify compilation first (`make build`), then integration testing (`make run-test`).

**Warmup Fail-Fast**: `Warmup()` returns error if stream is unstable - this is intentional design to prevent production use of unreliable streams. See [Warmup & FPS Stability](#warmup--fps-stability) for criteria. **Never skip warmup in production.**

**H.264 Only**: Stream-capture only supports H.264 codec (baseline/main profile). H.265/HEVC requires different decoder elements. See [Troubleshooting: RTSP stream works with VLC](#issue-rtsp-stream-works-with-vlc-but-not-with-stream-capture) for diagnosis.

**TCP-Only Transport**: Current implementation uses `protocols=4` (TCP only) for maximum compatibility and go2rtc integration. UDP transport requires pipeline modification. See pipeline.go:61.

## Development Workflow

1. **Read before coding**: Understand the big picture architecture (this file + code comments)
2. **Fail-fast principle**: Validate inputs at construction time, not runtime
3. **Atomic operations**: Use `sync/atomic` for counters, avoid locks in hot paths
4. **Graceful degradation**: Don't crash on single frame failures
5. **Lock-free telemetry**: Use `atomic.Pointer` for frequently-updated statistics
6. **Testing**: Always verify compilation first, then manual integration testing

## Commit Standards

- Co-authored by: `Gaby de Visiona <noreply@visiona.app>`
- Do NOT include "Generated with Claude Code" footer
- Focus on "why" rather than "what" in commit messages
- Follow existing commit style (see `git log` in parent repository)

---

## Documentation Version

**Last Updated**: 2025-11-04
**Revision**: 2.1 (Technical Consultation Quick Wins - Phase 1)

**Improvements from v2.0**:
- ✅ Added **Anti-Patterns & Alternatives** section (Quick Win #3)
  - 6 anti-patterns with clear "Why not" + "Use instead" guidance
  - 5 ideal use cases with concrete examples
  - Migration patterns for outgrowing limitations

**Improvements from v1.0**:
- ✅ Added **Quick Start** section with production-ready minimal example
- ✅ Enhanced **Warmup & FPS Stability** with fail-fast pattern and jitter telemetry explanation
- ✅ Added **Error Telemetry & Diagnostics** section with actionable remediation table
- ✅ Added comprehensive **Troubleshooting Guide** (8 common issues with diagnosis/solutions)
- ✅ Documented **Optimization Levels** hierarchy (Level 1/2/3) for pipeline.go
- ✅ Clarified **Common Patterns** with production emphasis (warmup is REQUIRED)
- ✅ Expanded **Known Issues & Design Decisions** with cross-references and rationale

**Validation**: All technical content cross-validated against source code (rtsp.go, pipeline.go, warmup_stats.go, callbacks.go).

**For future Claude instances**: This documentation follows Visiona's "Complejidad por diseño" philosophy - complexity is managed through architecture, not complicated code. Read the "Quick Start" and "Troubleshooting Guide" first for practical orientation.
