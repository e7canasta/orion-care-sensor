```mermaid
flowchart LR
  subgraph Orion Orchestrator - internal/core/orion.go
    Orion[core.Orion - Main Service Orchestrator]
  end

  subgraph Stream Layer - internal/stream/
    StreamProvider[StreamProvider interface]
    RTSPStream[RTSPStream - GStreamer RTSP capture]
  end

  subgraph Distribution Layer - internal/framebus/
    FrameBus[framebus.Bus - Non-blocking fan-out hub]
  end

  subgraph Worker Layer - internal/worker/
    WorkerInterface[types.InferenceWorker interface]
    PythonWorker[PythonPersonDetector - Subprocess bridge]
  end

  subgraph Control & Output - internal/control/ + internal/emitter/
    ControlHandler[control.Handler - MQTT command processor]
    MQTTEmitter[emitter.MQTTEmitter - Results publisher]
  end

  subgraph Python Subprocess - models/
    RunWorker[run_worker.sh - venv activation wrapper]
    PersonDetector[person_detector.py - ONNX inference worker]
  end

  %% Relaciones principales
  Orion -->|initializes| StreamProvider
  Orion -->|initializes| FrameBus
  Orion -->|initializes| WorkerInterface
  Orion -->|initializes| ControlHandler
  Orion -->|initializes| MQTTEmitter

  StreamProvider -.->|implements| RTSPStream

  RTSPStream --> Orion

  FrameBus -->|Distribute frame| PythonWorker

  WorkerInterface -->|implements| PythonWorker
  PythonWorker -->|exec.Command| RunWorker
  PythonWorker -->|stdin MsgPack| PersonDetector
  PersonDetector -->|stdout MsgPack| PythonWorker
  PythonWorker --> Orion

  Orion -->|Results chan *types.Inference| WorkerInterface
  StreamProvider -->|Frames chan *types.Frame| Orion

  ControlHandler -->|callbacks| Orion
  MQTTEmitter -->|consumeInferences| Orion

  RunWorker -->|spawns| PersonDetector

