# test-capture - Stream Capture Testing Tool

**Version**: v0.1.0
**Module**: stream-capture (Sprint 1.1)
**Purpose**: Manual testing and validation of RTSP stream capture

---

## Overview

`test-capture` is a command-line tool for testing the `stream-capture` module with real RTSP streams. It provides:

- ✅ Real-time frame capture with console logging
- ✅ Optional frame saving to disk (raw RGB format)
- ✅ Live statistics reporting (configurable interval)
- ✅ Graceful shutdown with final statistics
- ✅ Support for all module features (resolution, FPS, etc.)

---

## Quick Start

### Build

```bash
# Using Makefile
make test-capture

# Or directly with go build
go build -o bin/test-capture ./cmd/test-capture
```

### Basic Usage

```bash
# Minimal - just capture and log
./bin/test-capture --url rtsp://192.168.1.100/stream

# Save frames to disk
./bin/test-capture --url rtsp://camera/stream --output ./frames

# Custom FPS and resolution
./bin/test-capture \
  --url rtsp://camera/stream \
  --fps 1.0 \
  --resolution 1080p \
  --output ./frames
```

### Using Makefile

```bash
# Simple test (no frame saving)
RTSP_URL=rtsp://camera/stream make run-test

# Save frames with custom params
RTSP_URL=rtsp://camera/stream \
OUTPUT_DIR=./captured_frames \
FPS=0.5 \
RES=720p \
make run-test

# Debug mode
RTSP_URL=rtsp://camera/stream DEBUG=1 make run-test

# Capture only 100 frames
RTSP_URL=rtsp://camera/stream MAX_FRAMES=100 make run-test
```

---

## Command-Line Options

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--url` | string | **(required)** | RTSP stream URL |
| `--resolution` | string | `720p` | Resolution: `512p`, `720p`, `1080p` |
| `--fps` | float | `2.0` | Target FPS (0.1-30) |
| `--source` | string | `test` | Source stream identifier |
| `--output` | string | *(none)* | Directory to save frames (optional) |
| `--format` | string | `png` | Output format: `png`, `jpeg` |
| `--jpeg-quality` | int | `90` | JPEG quality (1-100, only for JPEG) |
| `--max-frames` | int | `0` | Max frames to capture (0 = unlimited) |
| `--stats-interval` | int | `10` | Seconds between stats reports |
| `--debug` | bool | `false` | Enable debug logging |
| `--version` | bool | `false` | Show version and exit |

---

## Examples

### Example 1: Basic Capture (No Saving)

```bash
./bin/test-capture --url rtsp://192.168.1.100:8554/stream
```

**Output**:
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
  Output Dir:    (none - frames not saved)
  Max Frames:    unlimited

Press Ctrl+C to stop gracefully
═══════════════════════════════════════════════════════════

[12:34:56] Frame #1      | Seq: 1        | Size:  123.4 KB | Timestamp: 12:34:56.123
[12:34:56] Frame #2      | Seq: 2        | Size:  124.1 KB | Timestamp: 12:34:56.623
...
```

### Example 2: Capture + Save Frames (PNG)

```bash
./bin/test-capture \
  --url rtsp://camera/stream \
  --output ./frames \
  --format png \
  --fps 1.0 \
  --max-frames 50
```

**Output**:
- Console logging (as above)
- PNG files saved to `./frames/`:
  - `frame_000001_20251103_123456.123.png` (~960 KB)
  - `frame_000002_20251103_123457.123.png` (~960 KB)
  - ...

### Example 2b: Capture + Save Frames (JPEG - smaller)

```bash
./bin/test-capture \
  --url rtsp://camera/stream \
  --output ./frames \
  --format jpeg \
  --jpeg-quality 85 \
  --fps 1.0 \
  --max-frames 50
```

**Output**:
- JPEG files saved to `./frames/`:
  - `frame_000001_20251103_123456.123.jpeg` (~140 KB)
  - `frame_000002_20251103_123457.123.jpeg` (~140 KB)
  - ...

### Example 3: Stats Reporting

Every 10 seconds (configurable with `--stats-interval`):

```
╭─────────────────────────────────────────────────────────╮
│ Stream Statistics (Uptime: 30s)
├─────────────────────────────────────────────────────────┤
│ Frames Captured:        60 frames
│ Frames Saved:           60 frames
│ Frames Dropped:          0 frames (0.0%)
│ Target FPS:           2.00 fps
│ Real FPS:             2.01 fps
│ Latency:               498 ms
│ Bytes Read:           7.32 MB
│ Reconnects:              0
│ Connected:            true
╰─────────────────────────────────────────────────────────╯
```

### Example 4: Final Statistics (on Ctrl+C)

```
═══════════════════════════════════════════════════════════
                     Final Statistics
═══════════════════════════════════════════════════════════
  Total Uptime:       2m30s
  Frames Captured:    300 frames
  Frames Saved:       298 frames
  Frames Dropped:     2 frames
  Save Success Rate:  99.3%
  Average FPS:        2.00 fps
  Bytes Read:         36.45 MB
  Reconnection Count: 0
═══════════════════════════════════════════════════════════
```

---

## Saved Frame Formats

Frames are saved as **PNG** (lossless) or **JPEG** (compressed) with the following naming convention:

```
frame_{sequence:06d}_{timestamp}.{format}
```

Example: `frame_000042_20251103_123456.789.png`

### Format Comparison

| Format | Size (720p) | Size (1080p) | Quality | Use Case |
|--------|-------------|--------------|---------|----------|
| **PNG** | ~960 KB | ~2.2 MB | Lossless | Quality inspection, analysis |
| **JPEG (q=90)** | ~140 KB | ~320 KB | High | General testing |
| **JPEG (q=85)** | ~140 KB | ~300 KB | High | Storage-efficient |
| **JPEG (q=70)** | ~100 KB | ~220 KB | Good | Long captures |

### Viewing Frames

Frames are saved in standard image formats and can be viewed with any image viewer:

```bash
# Linux
xdg-open frame_000001.png

# macOS
open frame_000001.png

# Command line (ImageMagick)
display frame_000001.png
```

---

## Troubleshooting

### "GStreamer not available"

**Cause**: GStreamer not installed or not in PATH.

**Solution**:
```bash
# Ubuntu/Debian
sudo apt-get install gstreamer1.0-tools gstreamer1.0-plugins-base \
                     gstreamer1.0-plugins-good gstreamer1.0-plugins-bad

# macOS
brew install gstreamer gst-plugins-base gst-plugins-good gst-plugins-bad
```

### "Failed to create pipeline"

**Cause**: Missing GStreamer plugins (rtsp, h264).

**Solution**:
```bash
# Install RTSP and H.264 plugins
sudo apt-get install gstreamer1.0-plugins-ugly gstreamer1.0-libav
```

### High Frame Drop Rate

**Cause**: Disk I/O bottleneck when saving frames.

**Solutions**:
- Use faster storage (SSD)
- Reduce FPS (`--fps 1.0`)
- Lower resolution (`--resolution 512p`)
- Don't save frames (omit `--output`)

---

## Use Cases

### 1. Module Validation (Sprint 1.1 Acceptance Criteria)

```bash
# Test RTSP connection
RTSP_URL=rtsp://camera/stream make run-test

# Test reconnection (manually disconnect go2rtc during test)
RTSP_URL=rtsp://camera/stream make run-test

# Test FPS stability (observe stats)
RTSP_URL=rtsp://camera/stream STATS_INTERVAL=5 make run-test
```

### 2. Performance Benchmarking

```bash
# Measure real FPS vs target
./bin/test-capture --url rtsp://camera/stream --fps 10 --max-frames 1000

# Measure latency
./bin/test-capture --url rtsp://camera/stream --stats-interval 1
```

### 3. Frame Quality Inspection

```bash
# Save frames for visual inspection
./bin/test-capture \
  --url rtsp://camera/stream \
  --fps 0.1 \
  --output ./inspection \
  --max-frames 10

# Convert and view
cd inspection
convert -size 1280x720 -depth 8 rgb:frame_000001*.rgb frame.png
open frame.png
```

### 4. Long-Running Stability Test

```bash
# Run for 1 hour, save every 60th frame
./bin/test-capture \
  --url rtsp://camera/stream \
  --fps 1.0 \
  --output ./stability_test \
  --stats-interval 60 \
  --max-frames 3600
```

---

## Integration with Sprint 1.1 Backlog

This tool addresses the following acceptance criteria:

- [x] **RTSP stream se captura correctamente** - Verified by frame logging
- [x] **Reconexión automática en caso de fallo** - Test by disconnecting stream
- [x] **FPS se mide durante warm-up (5 segundos)** - Logged at startup
- [x] **Frames se distribuyen sin bloqueo** - Verified by real-time logging

---

## Related Commands

- `make run-simple` - Run simple example (basic capture, no stats)
- `make run-hotreload` - Run hot-reload example (interactive FPS change)

---

**Maintained by**: Visiona Team
**Sprint**: 1.1 - Stream Capture Module
**Status**: ✅ Ready for manual testing
