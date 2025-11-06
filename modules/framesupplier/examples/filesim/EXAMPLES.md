# FileSim Usage Examples

## 1. Basic Testing (Quick Start)

```bash
# Simple test: 1 loop @ 5fps
./filesim -fps 5 -n 1
```

**Expected output:**
```
Loaded 10 frames from data/frames
Published frame 1/10 from frame_001.png
...
Completed 1 loops, stopping producer
```

---

## 2. Real-time Stats Monitoring ⭐

```bash
# Monitor FPS accuracy and drop rates
./filesim -fps 30 -stats
```

**Expected output:**
```
[STATS] FPS: 29.8/30.0 | Drops: worker-1=0.0% worker-2=0.0% worker-3=0.0% | Inbox: 0
```

**Metrics explained:**
- `FPS: 29.8/30.0` - Actual vs target FPS
- `worker-X=Y%` - Drop rate per worker
- `Inbox: N` - Dropped frames at distribution (should be 0)

---

## 3. Variable Worker Speeds (YOLO Simulation)

```bash
# Simulate PersonDetector (20ms), PoseDetector (50ms), VLM (100ms)
./filesim -fps 30 -stats -worker-delays "20ms,50ms,100ms"
```

**What to observe:**
- Worker-1 (20ms): ~0% drops (fast enough for 30fps)
- Worker-2 (50ms): 30-40% drops (1/30s = 33ms < 50ms)
- Worker-3 (100ms): 60-70% drops (slow worker)

**Validates**: Non-blocking drop policy works as designed

---

## 4. Stress Test (Trigger Drops)

```bash
# High FPS + slow workers = guaranteed drops
./filesim -fps 50 -stats -worker-delays "10ms,80ms,200ms"
```

**Expected:**
- Worker-3 will drop 75%+ frames
- Worker-2 will drop 50%+ frames
- Inbox drops should remain 0 (distribution loop is fast)

---

## 5. MQTT Integration Testing

```bash
# Add MQTT emitter worker
./filesim -fps 10 -mqtt-worker -mqtt-broker tcp://localhost:1883 -stats
```

**Note:** Currently STUB (logs only, doesn't actually publish)

---

## 6. Continuous Loop (Manual Stop)

```bash
# Run forever, CTRL+C to stop
./filesim -fps 30 -stats

# Press CTRL+C when done
^C
Producer finished, shutting down...
```

---

## 7. Custom Input Frames

```bash
# Use your own frame directory
./filesim -input /path/to/my/frames -pattern "*.jpg" -fps 15 -n 2
```

---

## 8. Run Tracking (Ultralytics-style) ⭐⭐

### Auto-increment Experiments
```bash
# First run
./filesim -fps 30 -n 1
# → runs/detect/exp1/

# Second run (auto-increment)
./filesim -fps 30 -n 1
# → runs/detect/exp2/

# Third run
./filesim -fps 30 -n 1
# → runs/detect/exp3/
```

### Custom Run Names
```bash
# Named benchmark
./filesim -name benchmark-baseline -fps 30 -n 2
# → runs/detect/benchmark-baseline/

# Try same name (auto-increment with suffix)
./filesim -name benchmark-baseline -fps 60 -n 2
# → runs/detect/benchmark-baseline1/

# Overwrite existing
./filesim -name benchmark-baseline --exist-ok -fps 30 -n 2
# → runs/detect/benchmark-baseline/ (overwritten)
```

### Inspect Results
```bash
# Always points to latest run
ls runs/detect/latest/

# View config
cat runs/detect/latest/config.yaml

# View stats (requires jq)
cat runs/detect/latest/stats.json | jq '.'

# Compare runs
diff <(cat runs/detect/exp1/stats.json) <(cat runs/detect/exp2/stats.json)
```

### Run Directory Structure
```
runs/detect/
├── exp1/
│   ├── config.yaml      # Run configuration
│   └── stats.json       # Final statistics
├── exp2/
│   ├── config.yaml
│   └── stats.json
├── benchmark-baseline/
│   ├── config.yaml
│   └── stats.json
└── latest -> exp2       # Symlink to most recent run
```

### Example stats.json
```json
{
  "duration_seconds": 1.5,
  "total_frames_published": 30,
  "total_loops": 3,
  "workers": {
    "worker-1": {
      "drop_rate_percent": 0,
      "total_drops": 0,
      "total_consumed": 30
    },
    "worker-2": {
      "drop_rate_percent": 33.3,
      "total_drops": 10,
      "total_consumed": 30
    },
    "worker-3": {
      "drop_rate_percent": 70.0,
      "total_drops": 21,
      "total_consumed": 30
    }
  },
  "inbox_drops": 0
}
```

### Use Cases
- **Reproducibility**: Re-run with exact same config
- **Benchmarking**: Compare performance across runs
- **CI/CD**: Archive test results with metadata
- **Debugging**: Historical runs for issue investigation

---

## Performance Benchmarks

### Disk I/O Bottleneck Test
```bash
# 3 workers writing simultaneously @ 30fps
./filesim -fps 30 -worker-delays "5ms,5ms,5ms" -stats
```

**What to measure:**
- If drops occur, disk write is bottleneck (not FrameSupplier)
- Compare with RAM disk (`-output /tmp/outputs`)

### Memory Usage Test
```bash
# Monitor memory during high FPS
./filesim -fps 60 -n 10 &
PID=$!
while kill -0 $PID 2>/dev/null; do 
  ps -p $PID -o rss,vsz | tail -1
  sleep 1
done
```

---

## Troubleshooting

### High Inbox Drops
```
[STATS] ... | Inbox: 152
```
→ **Problem:** Distribution loop slow (should NEVER happen)
→ **Check:** CPU starvation, deadlock, or design bug

### All Workers Dropping Frames
```
[STATS] ... | Drops: w1=50% w2=50% w3=50%
```
→ **Problem:** FPS too high OR disk I/O bottleneck
→ **Fix:** Lower `-fps` or use RAM disk for output

### Stats Not Updating
→ **Check:** `-stats` flag enabled?
→ **Check:** Producer actually running? (verify with logs)

---

## Advanced: Profiling

```bash
# CPU profiling
go run main.go -fps 60 -cpuprofile cpu.prof
go tool pprof cpu.prof

# Memory profiling
go run main.go -fps 60 -memprofile mem.prof
go tool pprof mem.prof
```

*(Note: Profiling flags not yet implemented - TODO)*
