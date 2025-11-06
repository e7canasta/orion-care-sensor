# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

---

## 🎯 For Claude Agents: Start Here

**IMPORTANT**: Before working on this project, read these documents IN ORDER:

1. **[CLAUDE_CONTEXT.md](./CLAUDE_CONTEXT.md)** - AI-to-AI knowledge transfer (philosophical patterns, pairing-specific context) - **READ FIRST**
2. **[PAIR_DISCOVERY_PROTOCOL.md](./PAIR_DISCOVERY_PROTOCOL.md)** - Discovery session process (Point Silla → Discovery → Crystallization)
3. **This document** (CLAUDE.md) - Project technical overview

**Why this order matters**:
- CLAUDE_CONTEXT.md = HOW to think and pair with Ernesto (cognitive patterns)
- PAIR_DISCOVERY_PROTOCOL = WHAT process to follow (discovery workflow)
- CLAUDE.md = WHAT you're working on (project context)

**Expected ramp-up**: <10 minutes to effective pair-discovery (vs hours of trial-error).

---

## Project Overview

**Orion** is a real-time AI inference system for video surveillance, specifically designed for geriatric patient monitoring. It operates as a "smart sensor" that observes and reports visual data via structured inference outputs, following the philosophy: **"Orión Ve, No Interpreta"** (Orion Sees, Doesn't Interpret).

### Technology Stack

- **Go 1.x**: Main orchestration, streaming pipeline, control plane, concurrency management
- **Python 3.x**: ONNX inference workers (YOLO11 person detection)
- **GStreamer**: Video capture and frame processing from RTSP streams
- **MQTT**: Event-driven control plane and data distribution
- **ONNX Runtime**: ML model inference with multi-model support (YOLO320/640)
- **MsgPack**: High-performance binary serialization for Go-Python IPC (5x faster than JSON+base64)

## Development Commands

### Building and Running

```bash
# Build the binary
make build

# Run the service
make run

# Run with debug logging
./bin/oriond --debug

# Run with custom config
./bin/oriond --config path/to/orion.yaml
```

### Configuration

- Primary config: `config/orion.yaml`
- MQTT topics:
  - Control: `care/control/{instance_id}`
  - Data: `care/inferences/{instance_id}`

### Testing

**Testing Philosophy**: Manual testing with pair-programming approach. No automated test files exist in prototype. The user runs tests manually while you observe and review.

- Integration testing via MQTT control commands
- Always verify compilation as primary "test"

### Python Environment

```bash
# Python workers use virtual environment
# Activated by: models/run_worker.sh
# Dependencies: worker-catalog/inference-workers/object-detection/peopledetection-worker/requirements.txt
```

## Architecture: The Big Picture

### Core Design Philosophy

1. **"Complejidad por diseño, no por accidente"** - Attack complexity through architecture, not complicated code
2. **Pragmatic Performance**: Real-time responsiveness > completeness. Intentional frame dropping, not queuing
3. **Non-blocking channels**: Drop frames to maintain <2s latency ("Drop frames, never queue")
4. **Hybrid Go-Python**: Go for orchestration/concurrency, Python for ML inference via subprocess
5. **MQTT-centric control**: Hot-reload capabilities without service restart
6. **KISS Auto-Recovery**: One restart attempt only - persistent failures require manual intervention

### Architectural Pattern

**Event-Driven Microkernel with Streaming Pipeline**

```
RTSP Camera → GStreamer → consumeFrames() → FrameBus (Fan-out)
                                                                ↓
                                                    ┌───────────┴──────────┐
                                                    ↓                      ↓
                                            Worker 1 (Python)      Worker 2 (Python)
                                                    ↓                      ↓
                                              ONNX Inference        ONNX Inference
                                                    ↓                      ↓
                                            Results Channel ←──────────────┘
                                                    ↓
                                            consumeInferences()
                                                    ↓
                                              MQTT Emitter
```

### Key Components

**Entry Point**: `cmd/oriond/main.go`
- Simple entry: parses flags (`--config`, `--debug`), sets up logging, signal handling, graceful shutdown

**Core Orchestrator**: `internal/core/orion.go`
- Manages lifecycle of all components
- Coordinates 3 primary goroutines:
  - `consumeFrames`: Reads stream, applies ROI, distributes to workers
  - `consumeInferences`: Collects results, publishes to MQTT
  - `watchWorkers`: Health monitoring with adaptive watchdog (max(30s, 3×inference_period))

**Stream Providers**: `internal/stream/`
- `rtsp.go`: GStreamer pipeline (`rtspsrc → videorate → videoscale → jpegenc`)
- `mock.go`: Test stream generator
- `warmup.go`: 5-second warm-up to measure real FPS

**FrameBus**: `internal/framebus/bus.go`
- Non-blocking fan-out to multiple workers
- Drop policy when worker channels full
- Per-worker drop statistics

**Python Worker Bridge**: `internal/worker/person_detector_python.go`
- Spawns Python subprocess via `exec.Command`
- MsgPack protocol with 4-byte length-prefix framing
- 4 goroutines: processFrames, readResults, logStderr, waitProcess
- Timeout protection: 2s for stdin writes

**Python Inference Worker**: `models/person_detector.py`
- Multi-model support (YOLO320 for small ROIs ~20ms, YOLO640 for full frames ~50ms)
- Model selection based on `roi_processing.target_size`
- Hot-reload command support via stdin JSON commands

**Control Handler**: `internal/control/handler.go`
- MQTT command processor with callback-based design
- Supports: pause/resume, rate adjustment, model hot-reload, ROI commands

**MQTT Emitter**: `internal/emitter/mqtt.go`
- Publishes to `care/inferences/{instance_id}`
- JSON payloads with timing metrics and metadata

### Go-Python IPC Protocol

**Go → Python (stdin):**
```
[4-byte length prefix][MsgPack payload]
{
  "frame_data": bytes,  // Raw JPEG, no base64
  "width": int,
  "height": int,
  "meta": {
    "instance_id": str,
    "room_id": str,
    "seq": int,
    "roi_processing": { "target_size": 320|640 }
  }
}
```

**Python → Go (stdout):**
```
[4-byte length prefix][MsgPack payload]
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

### Hot-Reload Capabilities

| Configuration  | Reload Mechanism   | Interruption | Implementation              |
| -------------- | ------------------ | ------------ | --------------------------- |
| Inference Rate | Stream restart     | ~2 seconds   | `RTSPStream.SetTargetFPS()` |
| Model Size     | Python reload      | None         | JSON command via stdin      |
| Attention ROIs | Thread-safe update | None         | `sync.RWMutex`              |
| Pause/Resume   | Flag toggle        | None         | Atomic boolean              |

## Code Patterns and Conventions

### Non-Blocking Channel Operations
```go
select {
case ch <- value:
    // Success
default:
    // Drop and continue (log drop stats)
}
```

### Dependency Injection via Callbacks
```go
CommandCallbacks{
    OnGetStatus: o.getStatus,
    OnPause: o.pauseInference,
    // ...
}
```

### Thread Safety
- Use `sync.RWMutex` for shared state
- Use `atomic` operations for counters
- Use `context.Context` for cancellation propagation

### Structured Logging
- `slog` with JSON handler
- Context fields: `instance_id`, `room_id`, `worker_id`
- Log levels: ERROR, WARNING, INFO, DEBUG

### Error Handling
- Log errors but continue processing (graceful degradation)
- Don't crash on single frame failure
- Worker failures trigger watchdog, not system shutdown

## Configuration Structure

**Primary config**: `config/orion.yaml`

Key sections:
- `instance_id`, `room_id`: Instance identification
- `camera.rtsp_url`: RTSP stream URL (optional, falls back to mock)
- `stream.resolution`, `stream.fps`: Stream settings (512p/720p/1080p)
- `models.person_detector.model_path`: YOLO model path
- `models.person_detector.max_inference_rate_hz`: Inference rate limit (e.g., 1.0)
- `mqtt.broker`: MQTT broker address

## MQTT Control Commands

```bash
# Get status
mosquitto_pub -t care/control/{instance_id} -m '{"command":"get_status"}'

# Pause inference
mosquitto_pub -t care/control/{instance_id} -m '{"command":"pause"}'

# Resume inference
mosquitto_pub -t care/control/{instance_id} -m '{"command":"resume"}'

# Change inference rate
mosquitto_pub -t care/control/{instance_id} -m '{"command":"set_inference_rate","rate_hz":2.0}'

# Change model size
mosquitto_pub -t care/control/{instance_id} -m '{"command":"set_model_size","size":"m"}'

# Shutdown
mosquitto_pub -t care/control/{instance_id} -m '{"command":"shutdown"}'
```

## Key Architectural Decisions

### AD-1: Non-Blocking Channels with Drop Policy
**Why**: Latency > completeness. Prefer dropping frames over queuing to maintain <2s latency.
- Avoids head-of-line blocking
- Predictable, bounded latency
- Drop statistics tracked for observability

### AD-2: Go-Python IPC via MsgPack over stdin/stdout
**Why**: Low latency (~1-2ms overhead), process isolation, simplicity
- MsgPack: 5x faster than JSON+base64
- No base64 overhead for binary data
- Subprocess isolation: Python crash doesn't kill Go orchestrator

### AD-3: KISS Auto-Recovery (One Restart)
**Why**: Simplicity > automation. One restart attempt only.
- Persistent failures indicate deeper issues (corrupt model, missing deps)
- Prevents infinite restart loops
- Requires manual intervention for persistent failures

### AD-4: Adaptive Watchdog Timeout
**Why**: Adapt to configured inference rate
- Formula: `max(30s, 3 × inference_period)`
- Example: 1 Hz inference → 3s expected, 30s timeout (safety margin)

### AD-5: MQTT for Control Plane
**Why**: IoT/edge deployment patterns, asynchronous control, NAT-friendly
- Works behind firewalls (outbound connections only)
- Event-driven by design
- Standard IoT protocol

## ROI Attention System

- **Multi-model selection**: YOLO320 (small ROIs) vs YOLO640 (full frames)
- **Dynamic threshold**: Model selection based on ROI area
- **Performance**: YOLO320 ~20ms, YOLO640 ~50ms inference time
- **Future**: ROI processing will be moved to Python workers (currently in Go)

## Documentation

Extensive architecture documentation in:
- `/home/visiona/Work/OrionWork/VAULT/arquitecture/ARCHITECTURE.md` - 4+1 architectural views
- `/home/visiona/Work/OrionWork/VAULT/D002 About Orion.md` - Design decisions and trade-offs
- `/home/visiona/Work/OrionWork/VAULT/arquitecture/wiki/` - Component-level wiki



```

VAULT/
│
├── 📘 ENTRADA & FILOSOFÍA
│   ├── D001 Bienvenido.md .................... [PUNTO DE ENTRADA]
│   ├── D002 About Orion.md ................... [FILOSOFÍA: "Ve, No Interpreta"]
│   └── D004 Analisis de Disenio y Codigo.md .. [PRINCIPIOS DE DISEÑO]
│
├── 🏛️ ARQUITECTURA GLOBAL
│   ├── D003 The Big Picture.md ............... [ANCLAJE: VISIÓN GENERAL]
│   └── arquitecture/
│       ├── ARCHITECTURE.md ................... [ANCLAJE: VISTAS 4+1]
│       └── another document of Arquitectura .. [CATÁLOGO DE DECISIONES]
│
├── 📚 WIKI TÉCNICA (Referencia Detallada)
│   ├── 2-core-service-oriond.md .............. [ANCLAJE: ORION CORE]
│   ├── 2.1-service-orchestration.md .......... [Ciclo de Vida]
│   ├── 2.2-stream-providers.md ............... [RTSP/GStreamer]
│   ├── 2.4-frame-distribution.md ............. [ANCLAJE: FRAMEBUS]
│   ├── 2.5-python-worker-bridge.md ........... [Go-Python IPC]
│   │
│   ├── 3-mqtt-control-plane.md ............... [ANCLAJE: PLANO DE CONTROL]
│   ├── 3.1-topic-structure.md ................ [Jerarquía de Topics]
│   ├── 3.2-command-reference.md .............. [Catálogo de Comandos]
│   ├── 3.3-hot-reload-mechanisms.md .......... [ANCLAJE: HOT-RELOAD]
│   │
│   ├── 4-python-inference-workers.md ......... [ANCLAJE: WORKERS PYTHON] ⭐
│   ├── 4.1-person-detector.md ................ [Implementación Detector]
│   └── 4.2-model-management.md ............... [Gestión de Modelos]
│
├── 🎤 NARRATIVA & CONTEXTO NEGOCIO
│   ├── El Viaje de un Fotón.md ............... [Narrativa de Negocio]
│   ├── Nuestro sistema de IA.md .............. [Filosofía de Diseño - Talk]
│   └── Orion_Ve,_Sala_Entiende.md ............ [Overview del Sistema - Podcast]
│
└── 🔬 PROPUESTAS & INVESTIGACIÓN
    ├── la resolución de entrada.md ........... [Nota de Investigación]
    └── Double-Close Panic.md ................. [Log de Fix Técnico]
    
```

## Development Workflow




### When Adding New Features

1. **Understand the Big Picture**: Review VAULT documentation before coding
2. **Complexity by Design**: Attack complexity through architecture, not code tricks
3. **Fail Fast**: Validate at load time, not runtime
4. **Cohesion > Location**: Modules defined by conceptual cohesion, not size
5. **One Reason to Change**: Each module has a single responsibility (SRP)

### Code Review Standards

- "Simple para leer, NO simple para escribir una vez"
- Clean design ≠ simplistic design
- Modularity reduces complexity when applied correctly
- Document architectural decisions (ADR style)

### Commit Standards

- Co-authored by: `Gaby de Visiona <noreply@visiona.app>`
- Do NOT include "Generated with Claude Code" footer (implicit in co-author)
- Focus on "why" rather than "what" in commit messages
- Follow existing commit style (see `git log`)

## System Positioning

**Orion is NOT**:
- A competitor to Frigate NVR (end-user product)
- A competitor to DeepStream/DL Streamer (monolithic frameworks)
- An interpretation or decision engine

**Orion IS**:
- A configurable "smart sensor" for distributed architectures
- Best-in-class for event-driven AI sensor deployments
- A building block for larger monitoring systems
- Hardware-agnostic (ONNX enables GPU acceleration in Python without Go changes)

## Scalability Paths

- **Horizontal**: Add new worker types (pose, facial recognition) via FrameBus fan-out
- **Vertical**: GPU acceleration in Python (transparent to Go)
- **Multi-stream**: Add `stream_id` metadata (minor changes needed)
- **Distributed**: Stateless design ready for Kubernetes

## Known Issues / Technical Debt

- MsgPack upgrade not yet documented (code exceeds documentation)
- Probe functionality disabled (GStreamer mainloop issues)
- ROI Processor planned to be removed from Go (workers will handle ROIs)
- No Makefile in prototype (binary pre-built in `bin/oriond`)


---

## Orion 2.0 Architecture

### Documentation Structure

**Strategic Documents**:
- [Plan Evolutivo](docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md) - 3-phase roadmap (v1.5 → v2.0 → v3.0)
- [C4 Model](docs/DESIGN/C4_MODEL.md) - Complete architectural blueprint (4 levels)
- [Architecture Decision Records](docs/DESIGN/ADR/README.md) - Technical memory of key decisions

**Backlog & Planning**:
- [BACKLOG/README.md](BACKLOG/README.md) - Overview general
- [BACKLOG/FASE_1_FOUNDATION.md](BACKLOG/FASE_1_FOUNDATION.md) - Sprints 1.1, 1.2, 2, 3
- [BACKLOG/FASE_2_SCALE.md](BACKLOG/FASE_2_SCALE.md) - Multi-stream architecture
- [BACKLOG/FASE_3_INTELLIGENCE.md](BACKLOG/FASE_3_INTELLIGENCE.md) - Cell orchestration

### Multi-Module Monorepo (Orion 2.0)

**Decision**: [ADR-001: Multi-Module Monorepo Layout](docs/DESIGN/ADR/001-multi-module-monorepo-layout.md)

Orion 2.0 migrates to a multi-module monorepo using Go workspaces:

```
OrionWork/
├── go.work                      # Workspace declaration
├── modules/
│   ├── stream-capture/          # BC: Stream Acquisition (Sprint 1.1)
│   ├── worker-lifecycle/        # BC: Worker Lifecycle (Sprint 1.2)
│   ├── framebus/                # BC: Frame Distribution
│   ├── control-plane/           # BC: Control Plane (Sprint 2)
│   ├── event-emitter/           # BC: Event Emission
│   └── core/                    # BC: Application Core
└── cmd/oriond/                  # Main binary
```

**Key Benefits**:
- ✅ Independent evolution per bounded context
- ✅ Localized documentation (CLAUDE.md, BACKLOG.md per module)
- ✅ Configurable recipes (edge-device vs datacenter)
- ✅ Semantic versioning per module
- ✅ Boundary enforcement via Go toolchain

**Module Documentation Structure**:
Each module includes:
- `CLAUDE.md` - AI companion guide (bounded context, API, anti-responsibilities)
- `README.md` - Human-readable overview
- `BACKLOG.md` - Sprint-specific tasks
- `docs/DESIGN.md` - Architectural decisions
- `docs/proposals/` - RFCs before implementation

### GitHub Integration

- **Repo**: https://github.com/e7canasta/orion-care-sensor
- **Project**: https://github.com/users/e7canasta/projects/7
- **Milestones**: v1.5 (2025-01-31), v2.0 (2025-03-31), v3.0 (2025-06-30)

Backlog markdown files are source of truth, synced with GitHub issues.



 ---  
 
### 🎯 Matriz de Complementariedad  
  
| Aspecto       | ARCHITECTURE.md (781 líneas)        | C4_MODEL.md (622 líneas)                 | Overlap  | Verdict             |     |
| ------------- | ----------------------------------- | ---------------------------------------- | -------- | ------------------- | --- |
| Audiencia     | Desarrolladores expertos            | Claude Code + Team onboarding            | 20%      | ✅ Complementario    |     |
| Propósito     | Referencia técnica enciclopédica    | Vistas arquitectónicas visuales          | 30%      | ✅ Complementario    |     |
| Estilo        | Deep dive (state machines, tables)  | High-level overview (C1→C4 progression)  | 0%       | ✅ Complementario    |     |
| Diagramas     | Estructurales (component diagrams)  | Contextuales (ecosystem, containers)     | 40%      | ⚠ Algo de overlap   |     |
| Code examples | Pseudocódigo + Go snippets          | Sequence diagrams + class diagrams       | 10%      | ✅ Complementario    |     |
  
 ---  

### 🔍 Análisis Detallado  
  
 Contenido ÚNICO en C4_MODEL.md (622 líneas)  
  
 Valor diferencial: Visión macro → micro progresiva  
  
 1. ✅ C1: System Context (Líneas 24-74)  
   - Stream-capture en ecosistema Orion 2.0  
   - Interacciones con Camera, MQTT, Worker Lifecycle  
   - NO está en ARCHITECTURE.md → Agrega contexto crítico  
 2. ✅ C2: Container Diagram (Líneas 76-140)  
   - Technology stack visual (Go → GStreamer → VAAPI → Kernel)  
   - CGo boundaries  
   - Parcialmente en ARCH (sección 3.2 tiene pipeline, pero no tech stack completo)  
 3. ✅ Thread Safety Model (Código, Líneas 452-481)  
   - 3 goroutines + atomic operations  
   - Diagrama visual de concurrency  
   - NO está en ARCHITECTURE.md → Crítico para entender race conditions  
 4. ✅ Sequence Diagrams (Líneas 303-383)  
   - Lifecycle completo: New → Start → Warmup → SetTargetFPS → Stop  
   - Formato diferente a ARCH (state machines vs sequences)  
 5. ✅ 8 ADRs completos con rationale (Líneas 485-622)  
   - ARCHITECTURE.md tiene 6 ADRs (AD-1 a AD-6)  
   - C4 tiene 8 ADRs más detallados (trade-offs, code refs)  
  
 ---  
### Contenido ÚNICO en ARCHITECTURE.md (781 líneas)  
  
 Valor diferencial: Deep dive técnico  
  
 1. ✅ Pipeline Element Configuration Table (Líneas 347-366)  
   - 12 elementos con todas las properties  
   - Ejemplo: rtspsrc.latency=200ms, appsink.max-buffers=1  
   - NO está en C4 → Referencia operacional crítica  
 2. ✅ Implementation Comparison Table (Líneas 154-165)  
   - RTSPStream vs MockStream (7 features)  
   - NO está en C4 → Útil para testing decisions  
 3. ✅ Hot-Reload State Machine (Líneas 403-450)  
   - Mermaid state diagram detallado  
   - C4 tiene sequence diagram (diferente view del mismo proceso)  
   - Complementario (state vs sequence)  
 4. ✅ Frame Channel Buffering Rationale (Líneas 549-583)  
   - Explicación profunda del non-blocking pattern  
   - C4 tiene versión resumida en AD-1 → ARCH es más exhaustivo  
 5. 🚧 Secciones TODO (Líneas 453-610)  
   - Warmup algorithm pseudocode  
   - VAAPI acceleration details  
   - Statistics & Telemetry  
   - Error Categorization  
   - C4 tiene esto completo → Oportunidad de sync  
  
 ---  
### 🎸 Overlap Identificado (~30-40%)  
  
 Coexntenido duplicado pero con enfoques diferentes:  
  
| Sección             | ARCHITECTURE.md                      | C4_MODEL.md                 | Tipo Overlap                                        |     |
| ------------------- | ------------------------------------ | --------------------------- | --------------------------------------------------- | --- |
| GStreamer Pipeline  | Mermaid flowcharts (Software/VAAPI)  | Mermaid graph (C2) + table  | ⚠ Duplicación visual (OK - diferentes niveles)      |     |
| Reconnection Logic  | State machine (Líneas 481-546)       | AD-5 + tabla (C4)           | ⚠ Duplicación moderada (OK - diferentes formatos)   |     |
| Design Decisions    | 6 ADRs (AD-1 a AD-6)                 | 8 ADRs (más detallados)     | ⚠ C4 es superset → Merge?                           |     |
---

## How to Work with Claude: Session Types

Orion 2.0 development supports two types of pairing sessions. **Claude should auto-detect which type based on Ernesto's initial message.**

### 🔍 Discovery Session (Exploratory Design)

**When**: Designing new modules or exploring uncharted design space

**Trigger Phrases**:
- "Charlemos el diseño de [module/feature]"
- "Pensaba en [technical decision]... ¿qué te parece?"
- "Pair-discovery: [topic]"
- "¿Cómo atacamos [problema] desde diseño?"

**Protocol**: See [PAIR_DISCOVERY_PROTOCOL.md](./PAIR_DISCOVERY_PROTOCOL.md) for complete operating manual.

**Key Points**:
- Explore tradeoffs in THIS context (not generic best practices)
- Think from first principles (not "let me research")
- Challenge proposals to enable discovery
- Checkpoint every 3-5 decisions
- Produce: ADRs + Architecture docs + Named insights

**Example**:
```
Ernesto: "Pair-discovery: worker-lifecycle restart policies.
         Pensaba en KISS (one-shot retry)... ¿qué te parece?"

Claude: "Restart policies for workers with different SLAs. Interesting.
        Let me explore tradeoffs:
        - KISS one-shot: Simple, fail-fast
        - Exponential backoff: Resilient, but masks failures
        - Hybrid (SLA-based): Critical=3 retries, BestEffort=0

        In context of fall detection (life-critical)...
        ¿Qué pensás?"
```

---

### 💻 Coding Session (Implementation)

**When**: Implementing already-designed functionality (ADRs exist)

**Trigger Phrases**:
- "Implementemos [module] según los ADRs"
- "Escribí el código para [component]"
- "Según ADR-X, [decision]. Empecemos."

**Protocol**: Read relevant ADRs + ARCHITECTURE.md, implement as specified.

**Key Points**:
- Do NOT explore alternatives (design already decided)
- Follow ADRs exactly
- Ask clarifications if ADR ambiguous
- Ensure compilation (primary test)
- Write tests (Ernesto runs manually)

**Example**:
```
Ernesto: "Implementemos FrameSupplier inbox mailbox.
         Según ADR-001 (sync.Cond) y ADR-004 (JIT input)."

Claude: "Entendido. Implementando según ADRs:
        - sync.Cond + mutex + single slot
        - Non-blocking Publish, blocking consume
        - Zero-copy (shared pointers)

        Empezando con inbox.go..."
```

---

### 🚨 DO NOT Mix Protocols

**Wrong** (exploring during coding):
```
Ernesto: "Implementemos con sync.Cond"
Claude: "¿Deberíamos explorar channels también?" ❌
```

**Wrong** (coding during discovery):
```
Ernesto: "Charlemos restart policies"
Claude: "Ok, voy a implementar backoff..." ❌
```

---

### If Ambiguous

If Claude cannot determine session type:
```
Claude: "¿Esto es discovery (explorar diseño) o coding (implementar según ADRs)?

- Discovery: Exploramos alternativas, cuestionamos, documentamos
- Coding: Implementamos según diseño ya definido

¿Cuál preferís?"
```

---

## Module-Specific Context

Each module has its own CLAUDE.md with:
- Bounded context definition
- Module-specific philosophy
- Session type examples
- References to ADRs

**Example module paths**:
- `modules/framesupplier/CLAUDE.md`
- `modules/stream-capture/CLAUDE.md` (future)
- `modules/worker-lifecycle/CLAUDE.md` (future)

**Always read module CLAUDE.md** before starting work on that module.

---
