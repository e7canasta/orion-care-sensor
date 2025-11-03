

---
## Project Overview ##


**Orion** is a real-time AI inference system for video streams, designed for surveillance and monitoring applications. It uses a hybrid Go-Python architecture:

- **Go** handles the streaming pipeline, frame distribution, MQTT control plane, and system orchestration
- **Python** workers run ONNX inference (YOLO person detection) via subprocess communication
- **GStreamer** manages video capture and frame processing
- **MQTT** provides the control plane for runtime configuration and inference result distribution


> Key architectural principle: **Pragmatic performance** - the system intentionally drops frames to maintain real-time processing rather than queuing and falling behind.


## Architecture Overview ##


### Core Pipeline Flow ###

```
RTSP Stream → GStreamer → Frame Channel → ROI Processor → FrameBus → Workers (Python)
                                                                          ↓
MQTT ← Emitter ← Inference Results ← Results Channel ← Python ONNX Inference

```


### Key Components

**1. `internal/core/orion.go`** - Main service orchestrator
- Coordinates all components (stream, workers, MQTT, control plane)
- Manages lifecycle (startup, shutdown, auto-recovery)
- Implements worker health watchdog with adaptive timeouts

**2. `internal/stream/rtsp.go`** - GStreamer video capture
- Wraps GStreamer pipeline for RTSP stream ingestion
- Converts raw frames to JPEG for Python workers
- Handles stream warmup to measure real FPS

**3. `internal/framebus/bus.go`** - Frame distribution hub
- Distributes frames to multiple workers (fan-out pattern)
- Non-blocking sends with drop policy (no backpressure)
- Tracks per-worker drop rates

**5. `internal/worker/person_detector_python.go`** - Python subprocess bridge
- Spawns Python worker via `models/run_worker.sh`
- Uses MsgPack with length-prefix framing for efficient communication
- Implements backpressure handling and timeout protection
- Supports hot-reload for model size changes

**6. `internal/control/handler.go`** - MQTT control plane
- Subscribes to `care/control/{instance_id}` for runtime commands
- Supports: pause/resume, rate adjustment, model hot-reload, ROI commands
- Implements graceful shutdown via MQTT

**7. `internal/emitter/mqtt.go`** - Inference result publisher
- Publishes to `care/inferences/{instance_id}` with JSON payloads
- Includes timing metrics and ROI metadata for downstream consumers

> el Namespace del sistema deberia ser care_orion o algo asi care/orion



---

### Python Worker Communication Protocol ###


**Go → Python (stdin)**: MsgPack with 4-byte length prefix
```python
{
  "frame_data": bytes,  # Raw JPEG bytes (NO base64!)
  "width": int,
  "height": int,
  "meta": {
    "instance_id": str,
    "room_id": str,
    "seq": int,
    "timestamp": str
  }
}
```



**Python → Go (stdout)**: MsgPack with 4-byte length prefix
```python
{
  "data": {
    "detections": [...],
    "person_count": int,
    "confidence_threshold": float
  },
  "timing": {
    "total_ms": float,
    "inference_ms": float
  }
}
```


---

### Auto-Recovery System

The worker health watchdog monitors workers every 30 seconds:
- Calculates **adaptive timeout** = `max(30s, 3 × inference_period)`
- If worker silent beyond timeout → restart once (KISS approach)
- Failed restarts require manual intervention (logged as error)

### Hot-Reload Support

Runtime configuration changes without restart:
- **Inference rate**: Adjusts `process_interval` dynamically
- **Model size**: Sends JSON command to Python worker to reload model
- **Auto-focus strategies**: Switch between simple/smoothing/velocity at runtime



## Configuration HotReload

Primary config: `config/orion.yaml`

Key sections:
- `instance_id`: Unique identifier for this Orion instance
- `mqtt.broker`: MQTT broker address


---

## Important Patterns & Conventions

### Frame Processing Philosophy
- **Drop frames, never queue**: Maintains real-time processing (< 2 second latency)
- **Non-blocking sends**: All channel operations use `select` with `default` case
- **GStreamer controls rate**: No rate limiting in Go workers (upstream videorate handles it)

---

### Worker Lifecycle
1. `NewWorker(config)` - Validate config, create channels
2. `Start(ctx)` - Spawn Python process, launch goroutines
3. `SendFrame(frame)` - Non-blocking frame distribution
4. `Stop()` - Cancel context, close stdin, wait with timeout, force kill if needed

---

### Error Handling
- Log errors but continue processing (don't crash on single frame failure)
- Use structured logging (`slog`) with context fields
- Worker failures trigger watchdog auto-recovery


---

### Thread Safety
- Use `sync.RWMutex` for shared state (e.g., ROI processor)
- Use `atomic` operations for counters (e.g., frame counts)
- Context cancellation for graceful shutdown

### MsgPack vs JSON
- **Go ↔ Python communication**: MsgPack (5x faster, no base64 overhead)
- **MQTT payloads**: JSON (human-readable, standard format)
- **Hot-reload commands**: JSON (rare operations, clarity over performance)


---


## Python Worker Development

Python workers are located in `models/`:
- `person_detector.py` - YOLO person detection worker
- `run_worker.sh` - Shell wrapper to activate venv and run worker

Worker requirements:
- Read MsgPack frames from stdin (4-byte length prefix + msgpack data)
- Write MsgPack results to stdout (4-byte length prefix + msgpack data)
- Log to stderr (Go parses log levels: [ERROR], [WARNING], [INFO], [DEBUG])
- Handle SIGTERM gracefully (close stdin signals shutdown)
- Support control commands via stdin (e.g., `{"type": "command", "command": "set_model_size"}`)




---


## MQTT Control Commands

Send to topic: `care/control/{instance_id}`

Examples:
```json
{"command": "get_status"}
{"command": "pause"}
{"command": "resume"}
{"command": "set_inference_rate", "rate_hz": 2.0}
{"command": "set_model_size", "size": "m"}
{"command": "set_attention_rois", "rois": [{"x": 0.2, "y": 0.3, "width": 0.4, "height": 0.5}]}
{"command": "clear_attention_rois"}
{"command": "enable_auto_focus"}
{"command": "disable_auto_focus"}
{"command": "set_auto_focus_strategy", "strategy": "smoothing", "params": {"alpha": 0.3}}
{"command": "shutdown"}
```

Key sections:
- `instance_id`: Unique identifier for this Orion instance
- `camera.rtsp_url`: RTSP stream URL 
- `stream.resolution`: Frame resolution (512p, 720p, 1080p)
- `stream.fps`: Target FPS (GStreamer videorate)
- `models.person_detector.max_inference_rate_hz`: Maximum inference rate (e.g., 1.0 = 1 Hz)

> The orion service instance al levantar informa que esta levantado y esperando asignacion de trabajo.

## Important Patterns & Conventions

### Frame Processing Philosophy
- **Drop frames, never queue**: Maintains real-time processing (< 2 second latency)
- **Non-blocking sends**: All channel operations use `select` with `default` case
- **GStreamer controls rate**: No rate limiting in Go workers (upstream videorate handles it)


-- 

## GStreamer Notes

- Requires GStreamer 1.0 development libraries
- Pipeline uses `videorate`, `videoscale`, `jpegenc` elements
- Mock stream uses `videotestsrc` for testing without RTSP camera
- Probe functionality currently disabled (mainloop issues - see `stream/probe.go`)

## Key Files to Understand

Start here for understanding the system:
1. `cmd/oriond/main.go` - Entry point and shutdown handling
2. `internal/core/orion.go` - Main orchestrator (read `Run()` method)
3. `internal/worker/person_detector_python.go` - Python bridge (extensive pseudocode comments)
4. `internal/config/config.go` - Configuration structure
5. `internal/types/*.go` - Core type definitions (Frame, Inference, Worker interfaces)


## Common Issues & Solutions

**Issue**: Worker appears hung (no inferences)
- Check `lastSeenAt` metric in health endpoint
- Watchdog will auto-restart after adaptive timeout
- Verify Python worker logs via stderr

**Issue**: High frame drop rate
- Expected behavior! System drops frames to maintain real-time
- Check `max_inference_rate_hz` - lower value = more drops
- Monitor per-worker drop rates in framebus stats

**Issue**: Python worker crashes on startup
- Verify ONNX model paths in config
- Check Python dependencies: `source venv/bin/activate && pip list`
- Run worker manually: `models/run_worker.sh --model models/yolo11s_fp32_640.onnx --confidence 0.5`

**Issue**: GStreamer pipeline errors
- Verify GStreamer installation: `gst-launch-1.0 --version`
- Test RTSP URL manually: `gst-launch-1.0 rtspsrc location=rtsp://... ! fakesink`
- Use mock stream for testing: remove `camera.rtsp_url` from config

---
---


```markdown


```


```markdown


 ```


---
