# Introduction


This page introduces the care-orion repository, its purpose as a real-time AI inference system for video streams, and the key architectural principles that guide its design. For build and deployment instructions

## What is Care-Orion?

Care-Orion is a real-time AI inference system that processes video streams for surveillance and monitoring applications. The system performs person detection on RTSP camera feeds using YOLO models, publishing inference results via MQTT for downstream consumption. It is designed to operate as a long-running service with runtime reconfigurability and automatic recovery from failures.

The system implements a hybrid architecture where Go handles orchestration, stream processing, and control plane management, while Python subprocesses execute ONNX inference. This separation allows the system to leverage Go's concurrency primitives for real-time coordination while utilizing Python's ML ecosystem for model inference.


## Core Architecture

**Component Responsibilities Mapping to Code**

| Component            | Primary File(s)                                                              | Key Responsibilities                                                                                     |
| -------------------- | ---------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| Service Orchestrator | `internal/core/orion.go`                                                     | Component lifecycle management, health monitoring via `checkWorkerHealth()`, frame pipeline coordination |
| Stream Providers     | `internal/stream/rtsp.go`  <br>`internal/stream/mock.go`                     | GStreamer pipeline management, frame capture, FPS control via `SetTargetFPS()`                           |
|                      |                                                                              |                                                                                                          |
| Frame Bus            | `internal/framebus/bus.go`                                                   | Non-blocking fan-out via `Distribute()`, drop policy implementation, worker registration                 |
| Python Workers       | `internal/worker/person_detector_python.go`  <br>`models/person_detector.py` | Subprocess lifecycle, MsgPack protocol, ONNX inference via `PersonDetector.process_frame()`              |
| Control Handler      | `internal/control/handler.go`                                                | MQTT command processing via `handleMessage()`, hot-reload orchestration                                  |
| MQTT Emitter         | `internal/emitter/mqtt.go`                                                   | Result publishing to `care/inferences/{instance_id}` topic                                               |
| Configuration        | `internal/config/config.go`                                                  | YAML parsing, validation, runtime updates                                                                |

> Nota: El Roi Procesor no va ser parte del Go Go informara el ROI y dejara que los Workers Trabajen ese Roi.

**Sources:** [cmd/oriond/main.go1-100](cmd/oriond/main.go#L1-L100) [internal/core/orion.go1-50](internal/core/orion.go#L1-L50)

## Key Architectural Principles

### 1. Pragmatic Performance: Intentional Frame Dropping

The system prioritizes **real-time responsiveness over completeness**. Rather than queuing frames during processing backlogs, the system intentionally drops frames to maintain sub-2-second latency. This design choice is implemented through:

- Non-blocking channel operations using `select` with `default` cases throughout the codebase
- Frame bus with explicit drop semantics in `internal/framebus/bus.go`
- Worker input channels with bounded capacity (5 frames)
- GStreamer `videorate` element controlling upstream frame rate

Frame drops are expected behavior, not failures. The system logs drop statistics but continues operation.

**Sources:** [CLAUDE.md14-15](CLAUDE.md#L14-L15) [CLAUDE.md184-188](CLAUDE.md#L184-L188) [internal/framebus/bus.go1-50](internal/framebus/bus.go#L1-L50)

### 2. MQTT-Centric Control Plane

All runtime configuration and monitoring flows through MQTT topics following the `care/*` hierarchy:

This architecture enables multiple concurrent clients to interact with a single Orion instance without conflicts, supporting both operational control and real-time observability.

**Sources:** [CLAUDE.md12](CLAUDE.md#L12-L12) [internal/control/handler.go1-50](internal/control/handler.go#L1-L50) [internal/emitter/mqtt.go1-50](internal/emitter/mqtt.go#L1-L50)

### 3. Hot-Reload Capabilities

The system supports runtime reconfiguration without service restart for most parameters:

|Configuration|Reload Mechanism|Interruption|Implementation|
|---|---|---|---|
|Inference Rate|Stream pipeline restart|~2 seconds|`RTSPStream.SetTargetFPS()` updates GStreamer caps|
|Model Size|Python worker reload|None|JSON command via stdin, `PersonDetector.set_model_size()`|
|Attention ROIs|Thread-safe update|None|`Processor.SetAttentionROIs()` with `sync.RWMutex`|
|Auto-focus Strategy|Thread-safe update|None|`Processor.SetAutoFocusStrategy()` with `sync.RWMutex`|
|Pause/Resume|Flag toggle|None|`Orion.isPaused` atomic boolean|

Commands are sent via MQTT to `care/control/{instance_id}` and processed by `internal/control/handler.go`. For complete command reference, see [Command Reference](3.2-command-reference.md).

**Sources:** [CLAUDE.md159-165](CLAUDE.md#L159-L165) [internal/control/handler.go1-100](internal/control/handler.go#L1-L100) [internal/stream/rtsp.go1-50](internal/stream/rtsp.go#L1-L50)

### 4. Hybrid Go-Python Architecture

The system leverages Go for orchestration and Python for inference:

**Communication Protocol:**

- **MsgPack encoding** for frame data and results (5x faster than JSON, no base64 overhead for binary data)
- **Length-prefix framing** (4-byte big-endian) for message delimitation
- **Structured stderr logging** parsed by Go for monitoring
- **JSON control commands** multiplexed on stdin for hot-reload

This protocol is defined in `internal/worker/person_detector_python.go` and implemented in `models/person_detector.py`.

**Sources:** [CLAUDE.md7-12](CLAUDE.md#L7-L12) [CLAUDE.md113-150](CLAUDE.md#L113-L150) [internal/worker/person_detector_python.go1-100](internal/worker/person_detector_python.go#L1-L100) [models/person_detector.py1-50](models/person_detector.py#L1-L50)

### 5. ROI Attention System with Multi-Model Selection

The system implements an intelligent attention mechanism that selects between two YOLO models based on region size:

- **YOLO320** (320x320): Used for crops <102,400 pixels (~320×320) - ~20ms inference
- **YOLO640** (640x640): Used for crops ≥102,400 pixels - ~50ms inference

ROI sources follow a four-tier priority system implemented in `internal/roiprocessor/processor.go`:

1. **External ROIs** (MQTT commands) - Highest priority
2. **Full frame** - Fallback when no ROIs available

This multi-model strategy provides 3-5x performance improvement by using the faster model on focused regions where person activity is likely. For detailed ROI processing logic, see [ROI Attention System](2.3-roi-attention-system).

## System Design Philosophy

Care-Orion is designed with specific operational constraints in mind:

1. **Real-time processing over batch accuracy**: The system assumes that processing every 1-2 seconds is sufficient for surveillance use cases. Missing frames is acceptable; falling behind is not.
    
2. **Operational flexibility**: Configuration changes should apply without service restart when possible. This enables rapid iteration during deployment and reduces operational overhead.
    
3. **Fail-safe defaults**: The worker watchdog implements automatic recovery for transient failures, but requires manual intervention for persistent issues. This prevents infinite restart loops while maintaining high availability.
    
4. **Observable by default**: All components emit structured logs and metrics. The `inference-logger` TUI and MQTT topic hierarchy provide real-time visibility without requiring external observability infrastructure.
    
5. **Testable architecture**: The mock stream provider and MQTT control plane enable comprehensive integration testing without requiring physical cameras. See [Integration Tests](#6.2-integration-tests).
    


## Next Steps


- **For architecture deep-dive**: See [Architecture Overview](1.2-architecture-overview.md) for detailed component interactions

