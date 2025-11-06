# Orion Pipeline Example - Module Composition Demo

**Version**: v0.1.0
**Purpose**: Demonstrate composability between `stream-capture` and `framesupplier` modules
**Philosophy**: "Testing composability = Lego freedom" 🎸

---

## Overview

This example demonstrates **real module composition** in the Orion 2.0 monorepo:

```
stream-capture (Producer) → FrameSupplier (Distributor) → Mock Workers (Consumers)
```

### What This Validates

✅ **Composability**: Two independent modules work together seamlessly
✅ **Non-blocking Distribution**: Slow workers don't block the pipeline
✅ **Drop Policy**: Visualize frame drops under different worker loads
✅ **Zero-copy**: Same frame pointer shared across workers (logged)
✅ **Batching**: Sequential distribution (<8 workers) vs batched (≥8 workers)
✅ **Idle Detection**: Identify workers falling behind
✅ **Realistic Usage**: JPEG decode + processing simulation

---

## Quick Start

### Build

```bash
# From monorepo root
cd examples/orion-pipeline
go build -o bin/pipeline .
```

### Run with Real RTSP Stream

**Note**: Currently requires real RTSP stream (mock stream not yet implemented in stream-capture module).

```bash
./bin/pipeline --url rtsp://192.168.1.100:8554/stream --fps 1.0
```

### Save Frames to Disk (Optional)

```bash
# Save as PNG (lossless, ~960KB @ 720p)
./bin/pipeline --url rtsp://127.0.0.1:8554/stream --output ./frames

# Save as JPEG (compressed, ~140KB @ 720p)
./bin/pipeline --url rtsp://127.0.0.1:8554/stream --output ./frames --format jpeg --jpeg-quality 85
```

**Saved frames**: `frame_{seq:06d}_{timestamp}.{ext}`
Example: `frame_000042_20251105_234517.123.png`

---

## Command-Line Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--url` | string | *(required)* | RTSP stream URL |
| `--resolution` | string | `720p` | Stream resolution (512p, 720p, 1080p) |
| `--fps` | float | `1.0` | Target FPS |
| `--output` | string | *(none)* | Output directory to save frames (optional) |
| `--format` | string | `png` | Output format: png or jpeg |
| `--jpeg-quality` | int | `90` | JPEG quality (1-100, only for JPEG) |
| `--stats-interval` | int | `5` | Statistics reporting interval (seconds) |
| `--debug` | bool | `false` | Enable debug logging |

---

## Mock Workers

The example spawns **3 workers** with different processing latencies:

| Worker | Latency | SLA | Expected Behavior |
|--------|---------|-----|-------------------|
| **Worker-Fast** | 10ms | Critical | ✅ Processes all frames (no drops) |
| **Worker-Medium** | 50ms | Normal | ⚠ Occasional drops @ high FPS |
| **Worker-Slow** | 200ms | BestEffort | ❌ Frequent drops @ >5 FPS |

Each worker:
- **Validates RGB data** (dimensions, size - `Width × Height × 3 bytes`)
- **Simulates inference** (configurable latency, like ONNX forward pass)
- **Collects statistics** (processed, drops, avg latency)

**Note**: Workers consume RGB raw data (like real ONNX workers), not JPEG.

---

## Example Output

### Live Statistics (Every 5 seconds)

```
╭─────────────────────────────────────────────────────────────────╮
│ Pipeline Statistics (Uptime: 30s)
├─────────────────────────────────────────────────────────────────┤
│ Stream Capture:
│   Frames Captured:        60 frames
│   Frames Dropped:          0 frames (0.0%)
│   Target FPS:           1.00 fps
│   Real FPS:             1.01 fps
│   Latency:               498 ms
│   Reconnects:              0
│   Connected:            true
│
│ FrameSupplier:
│   Inbox Drops:             0 (0.0%)
│   Published:              60 frames
│   Active Workers:          3
│   Idle Workers:       Worker-Slow (3.2s)
│   Distribution:       Sequential (<8 workers)
│
│ Workers:
│   Worker-Fast    :   60 processed,   0 drops (0.0%), avg= 10ms
│   Worker-Medium  :   59 processed,   1 drop (1.7%), avg= 50ms
│   Worker-Slow    :   42 processed,  18 drops (30.0%), avg=200ms
╰─────────────────────────────────────────────────────────────────╯
```

### Final Statistics (on Ctrl+C)

```
═══════════════════════════════════════════════════════════════
                     Final Statistics
═══════════════════════════════════════════════════════════════
  Frames Captured:       150 frames
  Stream Drops:            0 frames (0.0%)
  Average FPS:           1.00 fps
  Reconnection Count:      0

  Inbox Drops:             0 (0.0%)
  Frames Published:      150

  Worker Summary:
    Worker-Fast    : 150 processed, 0 drops (0.0%)
    Worker-Medium  : 148 processed, 2 drops (1.3%)
    Worker-Slow    : 105 processed, 45 drops (30.0%)
═══════════════════════════════════════════════════════════════
```

---

## What to Observe

### 1. Non-blocking Distribution

**Expected**: Slow workers drop frames, but **don't block** fast workers or the stream.

- Worker-Slow processes ~40-50% of frames @ 1 FPS (200ms latency >> 1000ms interval)
- Worker-Fast processes 100% of frames
- Stream never blocks (inbox drops = 0)

**Validates**:
- ✅ FrameSupplier's non-blocking mailbox with drop policy
- ✅ RGB data integrity (correct dimensions, size)

---

### 2. Idle Detection

**Expected**: Worker-Slow appears in "Idle Workers" list when falling behind.

- Idle threshold: 2 seconds without consuming a frame
- Logged idle time shows how far behind the worker is

**Validates**: FrameSupplier's idle detection mechanism (ADR-XXX).

---

### 3. Zero-Copy (Debug Mode)

```bash
./bin/pipeline --url rtsp://camera/stream --debug
```

**Expected**: Same RGB data pointer logged across multiple workers (debug logs).

```
DEBUG Frame published to supplier stream_seq=42 size_kb=2765  # RGB @ 1280x720 = 2.7MB
DEBUG [Worker-Fast] Processing frame seq=42 ptr=0x7f8a3c001200
DEBUG [Worker-Medium] Processing frame seq=42 ptr=0x7f8a3c001200
DEBUG [Worker-Slow] Processing frame seq=42 ptr=0x7f8a3c001200
```

**Validates**: Zero-copy frame sharing (ADR-002).

**Note**: RGB frames are larger than JPEG (~2.7MB @ 720p vs ~120KB JPEG).

---

### 4. Distribution Method

| Workers | Expected Behavior |
|---------|-------------------|
| 3 workers | Sequential distribution (threshold=8 not reached) |
| 10 workers | Batched distribution (threshold exceeded) |

**To test batching**: Modify `config.WorkerProfiles` in `main.go` to spawn 10+ workers.

**Validates**: Batching optimization (ADR-003).

---

## Use Cases

### 1. Module Validation (Sprint 1.1 + 1.2 Acceptance Criteria)

```bash
# Test RTSP stream capture + frame distribution
./bin/pipeline --url rtsp://camera/stream --fps 1.0

# Test reconnection (disconnect go2rtc during test)
./bin/pipeline --url rtsp://camera/stream --stats-interval 1
```

**Validates**:
- ✅ RTSP stream captured correctly
- ✅ Frames distributed to multiple workers
- ✅ Non-blocking semantics work
- ✅ Reconnection handled gracefully

---

### 2. Performance Benchmarking

```bash
# High FPS stress test
./bin/pipeline --url rtsp://camera/stream --fps 10.0

# Observe drop rates at different FPS
./bin/pipeline --url rtsp://camera/stream --fps 1.0   # Worker-Slow: ~0% drops
./bin/pipeline --url rtsp://camera/stream --fps 5.0   # Worker-Slow: ~50% drops
./bin/pipeline --url rtsp://camera/stream --fps 10.0  # Worker-Slow: ~90% drops
```

**Validates**: Drop policy under load.

---

### 3. Long-Running Stability Test

```bash
# Run for 10 minutes
timeout 600s ./bin/pipeline --url rtsp://camera/stream --stats-interval 30
```

**Validates**:
- ✅ No memory leaks (frame pooling)
- ✅ Stable FPS over time
- ✅ Reconnection resilience

---

## Architecture Decisions Demonstrated

This example validates the following ADRs:

- **[ADR-001](../../modules/framesupplier/docs/ADR/001-sync-cond-for-mailbox-semantics.md)**: sync.Cond for blocking semantics
- **[ADR-002](../../modules/framesupplier/docs/ADR/002-zero-copy-frame-sharing.md)**: Zero-copy frame sharing
- **[ADR-003](../../modules/framesupplier/docs/ADR/003-batching-threshold-8.md)**: Batching with threshold=8
- **[ADR-004](../../modules/framesupplier/docs/ADR/004-symmetric-jit-architecture.md)**: Symmetric JIT (inbox + worker mailboxes)

**Related**: [C4 Model](../../modules/framesupplier/docs/C4_MODEL.md), [ARCHITECTURE.md](../../modules/framesupplier/docs/ARCHITECTURE.md)

---

## Troubleshooting

### "Failed to create stream provider"

**Cause**: GStreamer not installed or RTSP URL unreachable.

**Solution**:
```bash
# Test with mock stream first
./bin/pipeline --mock

# Install GStreamer if needed
sudo apt-get install gstreamer1.0-tools gstreamer1.0-plugins-base \
                     gstreamer1.0-plugins-good gstreamer1.0-plugins-bad
```

---

### "Worker-Fast also dropping frames"

**Cause**: CPU overload or FPS too high for system.

**Solution**:
- Reduce FPS: `--fps 0.5`
- Lower resolution: `--resolution 512p`
- Check CPU usage: `top -H -p $(pgrep pipeline)`

---

### "No idle detection triggered"

**Cause**: FPS too low (all workers keep up).

**Solution**: Increase FPS to stress workers:
```bash
./bin/pipeline --url rtsp://camera/stream --fps 10.0
```

At 10 FPS, Worker-Slow (200ms latency) **cannot keep up** (200ms >> 100ms interval).

---

## Module Dependencies

This example demonstrates dependencies between:

```
examples/orion-pipeline/
├── github.com/e7canasta/orion-care-sensor/modules/stream-capture (v0.1.0)
└── github.com/e7canasta/orion-care-sensor/modules/framesupplier (v0.1.0)
```

**Portability**: This example includes `go.mod` with `replace` directives, making it **portable** (can be copied outside monorepo).

---

## Future Extensions

### Add More Workers (Test Batching)

Edit `main.go`:

```go
config.WorkerProfiles = []WorkerProfile{
    {ID: "Worker-1", Latency: 10 * time.Millisecond, SLA: "Critical"},
    {ID: "Worker-2", Latency: 20 * time.Millisecond, SLA: "High"},
    // ... add 8+ workers total
}
```

**Expected**: Stats show "Distribution: Batched (threshold=8)".

---

### Integrate with control-plane Module (Future Sprint 2)

When `control-plane` module exists:

```go
// Add MQTT control handler
controlHandler := controlplane.NewHandler(supplier, ...)
go controlHandler.Run(ctx)
```

**Validates**: Hot-reload commands (pause/resume, FPS change).

---

### Profile Memory Usage

```bash
# Run with memory profiling
go run -gcflags="-m" . --mock --fps 10.0 2>&1 | grep escape

# Or use pprof
go run . --mock --pprof :6060 &
go tool pprof http://localhost:6060/debug/pprof/heap
```

**Validates**: Zero-copy eliminates frame allocations.

---

## Related Documentation

- [Global CLAUDE.md](../../CLAUDE.md) - Project overview
- [FrameSupplier Module](../../modules/framesupplier/)
- [Stream-Capture Module](../../modules/stream-capture/)
- [Pair Discovery Protocol](../../PAIR_DISCOVERY_PROTOCOL.md)

---

**Maintained by**: Visiona Team
**Sprint**: 1.2 - FrameSupplier Module
**Status**: ✅ Ready for testing
**Philosophy**: "Complejidad por diseño, no por accidente" 🎸
