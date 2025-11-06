# FileSim - Filesystem Frame Stream Simulator

Simulates frame streaming from disk files. Useful for testing FrameSupplier without cameras or GStreamer.

## Use Case

- **Input**: Directory with image files (PNG/JPEG) sorted by name
- **Output**: Multiple workers copying frames to separate output directories
- **Control**: Configurable FPS, loop count, file pattern

## Usage

### Basic (infinite loop @ 30fps)
```bash
go run main.go -input data/frames -output data/outputs
```

### Real-time Stats Monitoring ⭐
```bash
go run main.go -fps 30 -stats
# Output: [STATS] FPS: 29.8/30.0 | Drops: worker-1=0.0% worker-2=3.2% worker-3=12.0% | Inbox: 0
```

### Variable Worker Speeds (Simulate YOLO320/640)
```bash
go run main.go -fps 30 -worker-delays "20ms,50ms,100ms" -stats
# worker-1: Fast (PersonDetector - YOLO320)
# worker-2: Medium (PoseDetector - YOLO640)
# worker-3: Slow (VLM/Analytics)
```

### MQTT Emitter Worker
```bash
go run main.go -mqtt-worker -mqtt-broker tcp://localhost:1883
```

### Run Tracking (Ultralytics-style) ⭐⭐
```bash
# Auto-increment runs (exp1, exp2, exp3, ...)
go run main.go -fps 30 -n 1  # → runs/detect/exp1
go run main.go -fps 30 -n 1  # → runs/detect/exp2

# Custom run name
go run main.go -name benchmark-v1 -fps 30 -n 2

# Overwrite existing run
go run main.go -name benchmark-v1 --exist-ok

# Check saved results
cat runs/detect/latest/config.yaml
cat runs/detect/latest/stats.json | jq '.workers'
```

### Custom FPS
```bash
go run main.go -fps 5.0
```

### Fixed iterations
```bash
go run main.go -n 3  # Loop 3 times then stop
```

### All options combined
```bash
go run main.go \
  -input data/frames \
  -output data/outputs \
  -fps 30.0 \
  -n 2 \
  -pattern "*.jpg" \
  -stats \
  -worker-delays "20ms,50ms,100ms" \
  -mqtt-worker \
  -mqtt-broker tcp://localhost:1883
```

## Flags

| Flag            | Default              | Description                                      |
|-----------------|----------------------|--------------------------------------------------|
| -input          | data/frames          | Input directory with frame images                |
| -output         | data/outputs         | Base output directory                            |
| -fps            | 30.0                 | Frames per second to simulate                    |
| -n              | 0                    | Number of loops (0 = infinite)                   |
| -pattern        | *.png                | File pattern to match                            |
| -stats          | false                | Enable real-time stats monitoring ⭐             |
| -worker-delays  | 20ms,50ms,100ms      | Inference delays per worker (comma-separated) ⭐ |
| -mqtt-worker    | false                | Add MQTT emitter worker ⭐                       |
| -mqtt-broker    | tcp://localhost:1883 | MQTT broker address                              |
| **-project**    | **runs/detect**      | **Project directory for runs** ⭐⭐              |
| **-name**       | **exp**              | **Run name (auto-increment if exists)** ⭐⭐     |
| **-exist-ok**   | **false**            | **Overwrite existing run** ⭐⭐                  |
| **-save-stats** | **true**             | **Save stats.json at end of run** ⭐⭐           |

## Example Setup

1. **Prepare input frames:**
```bash
mkdir -p data/frames
# Copy your frames as: frame_001.png, frame_002.png, etc.
```

2. **Run simulator:**
```bash
go run main.go -fps 5 -n 1
```

3. **Check outputs:**
```bash
ls -l data/outputs/worker-1/
ls -l data/outputs/worker-2/
ls -l data/outputs/worker-3/
```

## What It Demonstrates

- **Producer pattern**: Reading files → Publish() to FrameSupplier
- **Consumer pattern**: Subscribe() → blocking read → process (copy to disk)
- **Multi-worker fanout**: 3 workers, all receive same frames
- **Frame drops**: If worker slow (disk I/O), frames dropped (check logs)
- **Graceful shutdown**: CTRL+C or natural completion (-n flag)
- **Real-time metrics**: FPS accuracy, drop rates per worker, inbox health ⭐
- **Variable inference times**: Simulates different worker SLAs (critical vs best-effort) ⭐
- **MQTT integration**: Optional MQTT emitter worker for end-to-end testing ⭐
- **Run tracking**: Ultralytics-style runs system (config.yaml + stats.json) ⭐⭐

## Architecture

```
Disk Files → Producer → FrameSupplier → Worker 1 → data/outputs/worker-1/
                             ↓
                          Worker 2 → data/outputs/worker-2/
                             ↓
                          Worker 3 → data/outputs/worker-3/
```

## Expected Behavior

- All workers receive **same frames** (fanout)
- Frame sequence numbers increment monotonically
- Slow workers may drop frames (logged as WARNING)
- If `-n` specified, exits after N loops
- If `-n 0` (default), runs until CTRL+C

## Performance Notes

- **Disk I/O bottleneck**: Workers writing simultaneously may slow down
- **FPS accuracy**: Actual FPS may vary based on disk read speed
- **Memory**: Entire frame loaded into memory (100KB-1MB per frame)

## Troubleshooting

**No frames found:**
```
ERROR: No frames found in data/frames matching *.png
```
→ Check input directory and pattern

**Permission denied:**
```
ERROR: Failed to create output dir
```
→ Check write permissions on output directory

**Frame drops logged:**
```
WARNING: [worker-2] dropped frame (mailbox full)
```
→ Normal if FPS > worker processing rate
