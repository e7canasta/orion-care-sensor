# Testing Guide - Stream Capture Module

**Sprint**: 1.1 - Stream Capture Module
**Testing Approach**: Manual pair-programming (Ernesto executes, Gaby observes)

---

## Overview

This module includes three ways to test RTSP stream capture:

1. **`test-capture`** - Professional testing tool (recommended)
2. **`simple`** - Basic capture example
3. **`hot-reload`** - Interactive FPS adjustment example

---

## Quick Start

### 1. Build Everything

```bash
make all
```

### 2. Run Test Capture

```bash
# Replace with your actual RTSP URL
RTSP_URL=rtsp://192.168.1.100:8554/stream make run-test
```

---

## Test Scenarios (Sprint 1.1 Acceptance Criteria)

### ✅ Test 1: RTSP Connection

**Goal**: Verify RTSP stream se captura correctamente

**Steps**:
```bash
RTSP_URL=rtsp://your-camera/stream make run-test
```

**Expected**:
- Warm-up completes in ~5 seconds
- Frames logged to console with sequence numbers
- Real FPS matches target FPS (±10%)
- No errors in pipeline

**Success Criteria**:
- [ ] Pipeline reaches PLAYING state
- [ ] Frames received with correct resolution
- [ ] FPS mean within expected range

---

### ✅ Test 2: Reconnection

**Goal**: Reconexión automática en caso de fallo

**Steps**:
```bash
# 1. Start capture
RTSP_URL=rtsp://your-camera/stream make run-test

# 2. While running, disconnect camera or go2rtc
# 3. Wait and observe logs
# 4. Reconnect camera/go2rtc
```

**Expected**:
- Pipeline error detected
- Exponential backoff retry: 1s → 2s → 4s → 8s → 16s
- Max 5 reconnection attempts
- Stream resumes after reconnection
- Reconnect counter increments in stats

**Success Criteria**:
- [ ] Error logged: "pipeline error"
- [ ] Reconnection attempts logged with backoff times
- [ ] Stream resumes when source available
- [ ] Stats show reconnect count

---

### ✅ Test 3: Warm-up FPS Measurement

**Goal**: FPS se mide durante warm-up (5 segundos)

**Steps**:
```bash
RTSP_URL=rtsp://your-camera/stream DEBUG=1 make run-test
```

**Expected**:
- Log: "starting warm-up phase" (duration: 5s)
- Log: "RTSP stream started successfully" with FPS stats
  - warmup_frames: ~10 frames (for 2 FPS × 5s)
  - fps_mean: ~2.00
  - stable: true (if stddev < 15%)

**Success Criteria**:
- [ ] Warm-up completes in 5 seconds
- [ ] FPS mean calculated correctly
- [ ] Stability check logged (warning if unstable)

---

### ✅ Test 4: Hot-Reload FPS

**Goal**: Hot-reload de FPS sin reiniciar pipeline (~2s interrupción)

**Steps**:
```bash
# Use hot-reload example
RTSP_URL=rtsp://your-camera/stream make run-hotreload

# When interactive prompt appears:
> fps 0.5     # Change to 0.5 Hz (1 frame every 2 seconds)
> stats       # Check new FPS
> fps 5.0     # Change to 5 Hz
> quit
```

**Expected**:
- FPS update triggers capsfilter update
- Interruption: ~2 seconds (not full restart)
- New FPS reflected in stats
- Log: "updating target FPS" → "target FPS updated successfully"

**Success Criteria**:
- [ ] FPS change logged with old/new values
- [ ] Interruption < 3 seconds
- [ ] Real FPS matches new target FPS

---

### ✅ Test 5: Frame Quality Inspection

**Goal**: Verify frame data integrity (RGB format)

**Steps**:
```bash
# Capture 10 frames
RTSP_URL=rtsp://your-camera/stream \
OUTPUT_DIR=./frames \
FPS=1.0 \
MAX_FRAMES=10 \
make run-test

# Convert first frame to PNG
cd frames
convert -size 1280x720 -depth 8 rgb:frame_000001*.rgb test.png
open test.png  # macOS
# xdg-open test.png  # Linux
```

**Expected**:
- 10 `.rgb` files saved
- Each file size: width × height × 3 bytes
  - 720p: ~2.76 MB
  - 1080p: ~6.22 MB
- PNG conversion shows correct image

**Success Criteria**:
- [ ] Frames saved successfully (100% save rate)
- [ ] File sizes correct
- [ ] Image visible and recognizable

---

### ✅ Test 6: Long-Running Stability

**Goal**: Memory usage estable (no leaks en GStreamer buffers)

**Steps**:
```bash
# Run for 10 minutes (600 frames at 1 FPS)
RTSP_URL=rtsp://your-camera/stream \
FPS=1.0 \
MAX_FRAMES=600 \
STATS_INTERVAL=60 \
make run-test

# Monitor memory usage in another terminal
watch -n 5 'ps aux | grep test-capture'
```

**Expected**:
- Memory usage stable (no continuous growth)
- No goroutine leaks
- Frames processed continuously
- Stats report every 60 seconds

**Success Criteria**:
- [ ] Memory usage < 200 MB
- [ ] No memory growth over time
- [ ] All frames captured (600/600)

---

## Performance Benchmarks

### Latency Test

```bash
RTSP_URL=rtsp://your-camera/stream \
FPS=10.0 \
STATS_INTERVAL=5 \
make run-test
```

**Target**: Latency < 2 seconds

**Measurement**: Check "Latency" field in stats report

---

### Throughput Test

```bash
RTSP_URL=rtsp://your-camera/stream \
RES=1080p \
FPS=30.0 \
MAX_FRAMES=1000 \
make run-test
```

**Target**: Real FPS ≥ Target FPS × 0.9

**Measurement**: Compare "Real FPS" vs "Target FPS" in final stats

---

## Troubleshooting

### Problem: "GStreamer not available"

**Solution**:
```bash
# Ubuntu/Debian
sudo apt-get install gstreamer1.0-tools gstreamer1.0-plugins-base \
                     gstreamer1.0-plugins-good gstreamer1.0-plugins-bad \
                     gstreamer1.0-plugins-ugly gstreamer1.0-libav
```

### Problem: "rtph264depay element not found"

**Solution**:
```bash
# Install H.264 plugins
sudo apt-get install gstreamer1.0-plugins-ugly gstreamer1.0-libav
```

### Problem: High frame drop rate

**Causes**:
- Disk I/O bottleneck (when saving frames)
- CPU overload

**Solutions**:
- Use SSD for `--output` directory
- Lower FPS: `FPS=0.5 make run-test`
- Lower resolution: `RES=512p make run-test`
- Don't save frames (omit `OUTPUT_DIR`)

---

## Test Results Template

Use this template to document test results:

```markdown
## Test Results - [Date]

### Environment
- OS: Ubuntu 22.04 / macOS 14 / etc.
- GStreamer Version: 1.20.3
- Camera: [Model/Type]
- RTSP URL: rtsp://...

### Test 1: RTSP Connection
- [ ] Pass / [ ] Fail
- Notes: ...

### Test 2: Reconnection
- [ ] Pass / [ ] Fail
- Reconnection count: ...
- Notes: ...

### Test 3: Warm-up FPS
- [ ] Pass / [ ] Fail
- FPS mean: ...
- Stable: yes/no
- Notes: ...

### Test 4: Hot-Reload FPS
- [ ] Pass / [ ] Fail
- Interruption time: ...s
- Notes: ...

### Test 5: Frame Quality
- [ ] Pass / [ ] Fail
- Sample frame: [attach image]
- Notes: ...

### Test 6: Stability
- [ ] Pass / [ ] Fail
- Memory usage: ...MB
- Uptime: ...
- Notes: ...

### Issues Found
1. ...
2. ...

### Lessons Learned
- ...
```

---

## Next Steps (After Testing)

1. Document results in `BACKLOG.md` → Lecciones Aprendidas
2. Identify issues → Create GitHub issues
3. Update `docs/DESIGN.md` if architectural changes needed
4. Proceed to Sprint 1.2: Worker Lifecycle Module

---

**Testing Philosophy**: "Tests manuales con pair-programming"

**Pair-Programming Workflow**:
1. Ernesto ejecuta comandos
2. Gaby observa logs y métricas
3. Ambos discuten resultados
4. Documentar hallazgos en BACKLOG

---

**Status**: ✅ Ready for manual testing
**Last Updated**: 2025-11-03
