# Design Document - Stream Capture

**Module**: `github.com/e7canasta/orion-care-sensor/modules/stream-capture`
**Bounded Context**: Stream Acquisition
**Version**: v0.1.0
**Sprint**: Sprint 1.1
**Last Updated**: 2025-11-03

---

## 📋 Overview

> "Complejidad por diseño, no por accidente"

Este módulo es responsable de **capturar frames RTSP via GStreamer** con reconexión automática y FPS adaptativo. Ataca la complejidad mediante **arquitectura modular** (SRP), no código complicado.

**Filosofía**: Cada archivo < 150 líneas, un "motivo para cambiar" por componente.

---

## 🎯 Design Goals

1. **Low Latency**: Mantener < 2s latency (non-blocking channel sends, drop policy)
2. **Resilience**: Reconexión automática con exponential backoff (5 reintentos, 1s→16s)
3. **Adaptability**: Hot-reload de FPS sin restart (~2s interrupción vs 5-10s restart)
4. **Fail-Fast**: Validación en load time, mensajes de error claros
5. **KISS Auto-Recovery**: Un intento razonable de reconnect, luego manual intervention

---

## 🏗️ Architecture

### Bounded Context

**✅ Responsabilidades**:
- Capturar frames RTSP via GStreamer (RGB format)
- Reconexión automática con backoff exponencial
- FPS adaptativo (hot-reload via SetTargetFPS)
- Warm-up automático (5s medición)
- Distribución a canal no-bloqueante

**❌ Anti-Responsabilidades**:
- NO procesa frames (ROI, inference) → FrameBus/Workers
- NO decide qué capturar → Control Plane
- NO maneja workers → Worker Manager
- NO publica eventos → Event Emitter

---

### Component Structure

```
modules/stream-capture/
├── provider.go              # StreamProvider interface (40 líneas)
├── rtsp.go                  # RTSPStream lifecycle (150 líneas)
├── types.go                 # Frame, StreamStats, Resolution (60 líneas)
│
└── internal/                # Implementation details (NOT exported)
    ├── rtsp/
    │   ├── pipeline.go      # GStreamer setup/teardown (100 líneas)
    │   ├── callbacks.go     # onNewSample, onPadAdded (60 líneas)
    │   └── reconnect.go     # Exponential backoff (80 líneas)
    └── warmup/
        ├── warmup.go        # Warm-up measurement (80 líneas)
        └── stats.go         # FPS statistics (60 líneas)
```

**Rationale**:
- ✅ Separación por cohesión conceptual (SRP)
- ✅ Cada archivo < 150 líneas → legible en una sesión
- ✅ `internal/` protege implementation details (API pública estable)

---

### Component Diagram

```
┌──────────────────────────────────────────────────────────┐
│              StreamProvider Interface                    │
│  Start(ctx) (<-chan Frame, error)                       │
│  Stop() error                                            │
│  Stats() StreamStats                                     │
│  SetTargetFPS(fps float64) error                        │
└──────────────────────────────────────────────────────────┘
                        │
                        │ implements
                        ▼
┌──────────────────────────────────────────────────────────┐
│                    RTSPStream                            │
│  ┌────────────────────────────────────────────────────┐ │
│  │ Public Methods (rtsp.go)                           │ │
│  │  - NewRTSPStream(cfg) → fail-fast validation      │ │
│  │  - Start(ctx) → pipeline + warm-up + goroutine    │ │
│  │  - Stop() → cancel + wait + cleanup               │ │
│  │  - Stats() → atomic reads + calculations          │ │
│  │  - SetTargetFPS(fps) → update caps + rollback     │ │
│  └────────────────────────────────────────────────────┘ │
│                        │                                  │
│                        │ delegates to                     │
│                        ▼                                  │
│  ┌────────────────────────────────────────────────────┐ │
│  │ internal/rtsp/                                     │ │
│  │  - pipeline.go: CreatePipeline, UpdateFramerate    │ │
│  │  - callbacks.go: OnNewSample, OnPadAdded           │ │
│  │  - reconnect.go: RunWithReconnect (backoff)        │ │
│  └────────────────────────────────────────────────────┘ │
│                        │                                  │
│                        │ uses                             │
│                        ▼                                  │
│  ┌────────────────────────────────────────────────────┐ │
│  │ internal/warmup/                                   │ │
│  │  - warmup.go: WarmupStream (5s)                   │ │
│  │  - stats.go: FPS stats, stability check            │ │
│  └────────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────────┘
```

---

## 🔌 Public API Design

### StreamProvider Interface

```go
// StreamProvider defines the contract for video stream acquisition
type StreamProvider interface {
    // Start initializes the stream and returns a read-only channel of frames.
    // Blocks for ~5 seconds during warm-up to measure FPS stability.
    // Returns error if stream cannot be established.
    Start(ctx context.Context) (<-chan Frame, error)

    // Stop gracefully shuts down the stream.
    // Waits up to 3 seconds for goroutines to finish.
    // Safe to call multiple times (idempotent).
    Stop() error

    // Stats returns current stream statistics.
    // Thread-safe (uses atomic operations).
    Stats() StreamStats

    // SetTargetFPS updates the target FPS dynamically without restarting.
    // Causes ~2 second interruption while GStreamer adjusts caps.
    // Returns error if FPS out of range (0.1-30).
    SetTargetFPS(fps float64) error
}
```

**Design Rationale**:
- ✅ `Start()` blocks durante warm-up → caller recibe channel ya estable
- ✅ `Stop()` idempotent → safe para múltiples llamadas
- ✅ `SetTargetFPS()` hot-reload → no restart (mejor UX)
- ✅ `Stats()` thread-safe → puede llamarse desde cualquier goroutine

---

### Types

```go
// Frame represents a single video frame with metadata
type Frame struct {
    Seq          uint64      // Monotonic sequence number
    Timestamp    time.Time   // Capture timestamp
    Width        int         // Frame width in pixels
    Height       int         // Frame height in pixels
    Data         []byte      // RGB pixel data
    SourceStream string      // Stream identifier ("LQ", "HQ")
    TraceID      string      // Distributed tracing ID
}

// StreamStats contains current stream statistics
type StreamStats struct {
    FrameCount   uint64      // Total frames captured
    FPSTarget    float64     // Configured target FPS
    FPSReal      float64     // Measured real FPS
    LatencyMS    int64       // Time since last frame (ms)
    SourceStream string      // Stream identifier
    Resolution   string      // Frame resolution (e.g., "1280x720")
    Reconnects   uint32      // Number of reconnections
    BytesRead    uint64      // Total bytes read
    IsConnected  bool        // Connection status
}

// Resolution represents supported video resolutions
type Resolution int

const (
    Res512p  Resolution = iota  // 910x512
    Res720p                      // 1280x720
    Res1080p                     // 1920x1080
)

// Dimensions returns width and height for the resolution
func (r Resolution) Dimensions() (width, height int) {
    switch r {
    case Res512p:
        return 910, 512
    case Res720p:
        return 1280, 720
    case Res1080p:
        return 1920, 1080
    default:
        return 1280, 720  // Safe default
    }
}
```

---

## 🔀 Data Flow

### Start Sequence (with Warm-up)

```
Client          RTSPStream      internal/rtsp    internal/warmup    GStreamer
  │                 │                  │                  │              │
  ├─Start(ctx)─────>│                  │                  │              │
  │                 ├─CreatePipeline()─>│                  │              │
  │                 │<─────pipeline─────┤                  │              │
  │                 │                  │                  │              │
  │                 ├─SetState(Playing)────────────────────────────────>│
  │                 │                  │                  │              │
  │                 ├─go runPipeline()─>│                  │              │
  │                 │                  │                  │              │
  │                 ├─WarmupStream()────────────────────>│              │
  │                 │      [5 seconds consuming frames]  │              │
  │                 │<─────WarmupStats──────────────────┤              │
  │                 │                  │                  │              │
  │                 ├─Log FPS stability│                  │              │
  │                 │                  │                  │              │
  │<───frameChan────┤                  │                  │              │
  │                 │                  │                  │              │
  │      [Stream running, frames flowing]                 │              │
```

**Steps**:
1. `CreatePipeline()` construye GStreamer pipeline
2. Pipeline enters `Playing` state
3. `runPipeline()` goroutine empieza
4. `WarmupStream()` consume frames por 5s
5. Retorna channel estable

---

### Reconnection Sequence

```
RTSPStream      internal/rtsp     GStreamer       go2rtc
  │                  │                 │               │
  │                  │<──Pipeline Error┤               │
  │<─Connection Lost─┤                 │               │
  │                  │                 │               │
  │─Retry 1 (1s)────>│                 │               │
  │                  ├─Connect─────────────────────────>│
  │                  │<──Failed────────────────────────┤
  │                  │                 │               │
  │─Retry 2 (2s)────>│                 │               │
  │                  ├─Connect─────────────────────────>│
  │                  │<──Failed────────────────────────┤
  │                  │                 │               │
  │─Retry 3 (4s)────>│                 │               │
  │                  ├─Connect─────────────────────────>│
  │                  │<──Success───────────────────────┤
  │                  │                 │               │
  │─Reset counter───>│                 │               │
  │                  │                 │               │
  │      [Stream resumed, frames flowing]              │
```

**Steps**:
1. Pipeline error detectado
2. Exponential backoff: 1s, 2s, 4s, 8s, 16s
3. Max 5 retries → Stop()
4. Reset counter en successful connection

---

### Hot-Reload FPS Sequence

```
Client          RTSPStream      internal/rtsp    GStreamer
  │                 │                  │              │
  ├─SetTargetFPS(0.5)>│                  │              │
  │                 ├─Lock(mu)─────────│              │
  │                 │                  │              │
  │                 ├─Validate(0.5)────│              │
  │                 │                  │              │
  │                 ├─UpdateFramerateCaps()──────────>│
  │                 │      [~2s interruption]          │
  │                 │<─────Success──────────────────────┤
  │                 │                  │              │
  │                 ├─Unlock(mu)───────│              │
  │                 │                  │              │
  │<─────nil────────┤                  │              │
  │                 │                  │              │
  │      [Stream continues at 0.5 FPS] │              │
```

**Steps**:
1. Validate FPS (0.1-30)
2. Update `capsfilter` caps (framerate property)
3. ~2s interruption (GStreamer adjusts)
4. Rollback on error

---

## 🎨 Design Patterns

### Pattern 1: Non-Blocking Channel Send

**Usage**: Distribución de frames a canal sin bloquear pipeline.

**Rationale**: Latencia < 2s más importante que completitud de frames.

**Implementation**:
```go
// Send frame (non-blocking)
select {
case frameChan <- frame:
    // Frame sent successfully
default:
    // Channel full - drop frame
    slog.Debug("dropping frame, channel full", "seq", frame.Seq)
}
```

**Trade-offs**:
- ✅ Latencia predecible y acotada
- ✅ Memory usage constante
- ⚠️ Posible frame loss (mitigado por buffer 10)

---

### Pattern 2: Fail-Fast Validation

**Usage**: Constructor validation (load time errors).

**Rationale**: "Fail inmediato en load vs Runtime debugging hell".

**Implementation**:
```go
func NewRTSPStream(cfg RTSPConfig) (*RTSPStream, error) {
    // Fail-fast validations
    if cfg.URL == "" {
        return nil, fmt.Errorf("stream-capture: RTSP URL is required")
    }

    if cfg.TargetFPS <= 0 || cfg.TargetFPS > 30 {
        return nil, fmt.Errorf("stream-capture: invalid FPS %.2f (must be 0.1-30)", cfg.TargetFPS)
    }

    if err := checkGStreamerAvailable(); err != nil {
        return nil, fmt.Errorf("stream-capture: GStreamer not available: %w", err)
    }

    return &RTSPStream{...}, nil
}
```

**Trade-offs**:
- ✅ Errors claros en startup (no runtime surprises)
- ✅ Mensajes contextualizados ("stream-capture: ...")
- ✅ Documentación implícita (requisitos explícitos)

---

### Pattern 3: Exponential Backoff with Cap

**Usage**: Reconnection logic resiliente.

**Rationale**: Evitar thundering herd, dar tiempo a recovery.

**Implementation**:
```go
type ReconnectConfig struct {
    MaxRetries    int           // 5
    RetryDelay    time.Duration // 1s
    MaxRetryDelay time.Duration // 30s
}

func calculateBackoff(attempt int, cfg ReconnectConfig) time.Duration {
    delay := cfg.RetryDelay * time.Duration(1<<uint(attempt-1))
    if delay > cfg.MaxRetryDelay {
        delay = cfg.MaxRetryDelay
    }
    return delay
}

// Schedule: 1s → 2s → 4s → 8s → 16s
```

**Trade-offs**:
- ✅ Network-friendly (no spam connections)
- ✅ Permite recovery de servicios externos
- ⚠️ Max 5 retries → manual intervention (KISS)

---

## ⚡ Performance Considerations

### Latency Budget

| Component            | Latency       | Justification                  |
|----------------------|---------------|--------------------------------|
| GStreamer decode     | ~20-30ms      | Hardware H.264 decode          |
| Frame copy           | ~5ms          | memcpy (720p RGB ~2.7 MB)      |
| Channel send         | 0ms (async)   | Non-blocking send              |
| **Total**            | **~25-35ms**  | Real-time capable              |

**Solution**: Non-blocking sends, drop policy.

**Trade-offs**:
- ✅ Latencia constante < 2s
- ⚠️ Frames dropped si consumer slow (observable via Stats)

---

### Memory Usage

| Component            | Memory        | Notes                          |
|----------------------|---------------|--------------------------------|
| Frame buffer (10)    | ~27 MB        | 10 × 1280×720×3 bytes         |
| GStreamer pipeline   | ~50 MB        | Internal buffers               |
| **Total per stream** | **~80 MB**    | Reasonable for edge devices    |

**Solution**: Buffer de 10 frames (no unbounded).

**Trade-offs**:
- ✅ Memory usage predecible
- ✅ Absorbe jitter temporal
- ⚠️ Latencia max: 10 frames / FPS (e.g., 333ms @ 30 FPS)

---

## 🔒 Error Handling

### Strategy

**Principle**: "Fail fast at load time, graceful degradation at runtime"

**Load Time** (Constructor):
- Validate config (URL, FPS, Resolution)
- Check GStreamer availability
- Return descriptive errors

**Runtime** (Callbacks):
- Log errors, continue processing
- Drop frames on channel full (no panic)
- Reconnect on pipeline errors

**Principles**:
- ✅ Never panic (graceful degradation)
- ✅ Contextualized errors ("stream-capture: ...")
- ✅ Observable failures (logs, metrics)

---

### Error Examples

```go
// Constructor error (fail-fast)
stream, err := NewRTSPStream(cfg)
// Error: "stream-capture: RTSP URL is required"

// Runtime error (graceful)
func onNewSample(sink *app.Sink) gst.FlowReturn {
    sample := sink.PullSample()
    if sample == nil {
        slog.Error("failed to pull sample")
        return gst.FlowEOS  // Signal EOS, continue pipeline
    }
    // ...
    return gst.FlowOK
}
```

---

## 🧪 Testing Strategy

### Manual Testing (Pair-Programming)

**Filosofía**: "Tests como pair-programming - Ernesto ejecuta, Gaby observa"

#### Test 1: RTSP Connection (0.5 día)
```bash
go run examples/simple_capture.go --url rtsp://192.168.1.100/stream
```
**Verificar**: Warm-up logs, frames captured, Stats()

#### Test 2: Reconnection (0.5 día)
```bash
# Durante ejecución:
sudo systemctl stop go2rtc
# Observar: Retry logs (1s, 2s, 4s...)
sudo systemctl start go2rtc
# Observar: Stream resume
```

#### Test 3: Hot-Reload FPS (0.5 día)
```bash
> set_fps 0.5
# Observar: ~2s interruption, FPS change
```

#### Test 4: Warm-up Stats (0.5 día)
```bash
# Observar logs: fps_mean, fps_stddev, stable
```

### Compilation Tests (ALWAYS)

```bash
cd modules/stream-capture
go build ./...
```

---

## 🔗 Dependencies

### External Packages

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/tinyzimmer/go-gst` | v0.3.2 | GStreamer Go bindings |
| `github.com/google/uuid` | v1.6.0 | TraceID generation |

### System Dependencies

- GStreamer 1.x (runtime)
- Plugins: rtspsrc, rtph264depay, avdec_h264, videoconvert, videoscale, videorate

### Workspace Modules

None (leaf module, no internal dependencies)

---

## 🚧 Constraints

### Technical Constraints

- GStreamer 1.x required (not 0.x)
- H.264 codec only (RTSP stream format)
- TCP transport only (protocols=4, go2rtc compat)

### Business Constraints

- Latency < 2s (real-time requirement)
- Memory < 100 MB per stream (edge device)
- Hot-reload without downtime (UX requirement)

---

## 📊 Design Decisions

### Decision 1: Separación en Módulos Internos

**Context**: Prototipo tenía `rtsp.go` de 513 líneas con múltiples responsabilidades.

**Options**:
1. Monolito en `rtsp.go` - Simple, pero difícil de mantener
2. Subpackages públicos - Expone implementation details
3. **`internal/` packages** - Oculta detalles, SRP enforcement

**Decision**: Opción 3 (`internal/rtsp`, `internal/warmup`)

**Rationale**:
- ✅ Cada archivo < 150 líneas (legible)
- ✅ Un "motivo para cambiar" por archivo (SRP)
- ✅ Testeable en aislación

**Consequences**:
- ✅ Mejor mantenibilidad
- ⚠️ Más archivos (acceptable trade-off)

---

### Decision 2: Hot-Reload vs Restart

**Context**: Cambiar FPS requiere actualizar stream rate.

**Options**:
1. **Update GStreamer caps** - ~2s interruption
2. Restart pipeline - 5-10s downtime

**Decision**: Opción 1 (hot-reload)

**Rationale**:
- ✅ ~2s vs 5-10s (mejor UX)
- ✅ No pierde conexión RTSP
- ✅ Mantiene statistics

**Consequences**:
- ✅ UX superior
- ⚠️ Complejidad moderada (rollback on error)

---

### Decision 3: Frame Format (RGB vs BGR)

**Context**: GStreamer default RGB, OpenCV default BGR.

**Options**:
1. **RGB** - No conversión en GStreamer
2. BGR - Compatible con OpenCV

**Decision**: Opción 1 (RGB)

**Rationale**:
- ✅ No overhead de conversión
- ✅ Workers usan ONNX (RGB compatible)
- ⚠️ Si agregamos OpenCV worker → conversión needed

**Consequences**:
- ✅ Performance (sin overhead)
- ⚠️ Future OpenCV worker requiere conversión

---

### Decision 4: Warm-up Duration (5s Hardcoded)

**Context**: Stream tarda ~2-3s en estabilizarse.

**Options**:
1. **Hardcoded 5s** - KISS
2. Configurable - Más flexible

**Decision**: Opción 1 (5s hardcoded)

**Rationale**:
- ✅ Valor probado en prototipo
- ✅ KISS (evita over-configuración)
- ✅ Transparente para caller

**Consequences**:
- ✅ Start tarda 5s (acceptable para setup)
- ⚠️ No configurable (no hay evidencia de necesidad)

---

## 🔮 Future Enhancements

### Short-term (Sprint 1.2)

- [ ] Integration con FrameBus (Sprint 1.2)
- [ ] Mock stream provider para testing

### Long-term (v2.0+)

- [ ] Multi-stream support (map[string]*RTSPStream)
- [ ] Adaptive bitrate (change resolution on network conditions)
- [ ] Hardware acceleration (vaapi, nvdec)
- [ ] Frame compression (JPEG encode en GStreamer)

**Regla**: No implementar hasta tener evidencia de necesidad (YAGNI).

---

## 📚 References

### Workspace Documentation

- [C4 Model - Stream Capture Component](../../../docs/DESIGN/C4_MODEL.md#c3---component-diagram)
- [Plan Evolutivo - Sprint 1.1](../../../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#11-stream-capture-module)
- [BACKLOG - Fase 1](../../../BACKLOG/FASE_1_FOUNDATION.md#sprint-11-stream-capture-module)

### External Resources

- [GStreamer Documentation](https://gstreamer.freedesktop.org/documentation/)
- [go-gst Examples](https://github.com/tinyzimmer/go-gst/tree/main/examples)
- [RTSP RFC 2326](https://datatracker.ietf.org/doc/html/rfc2326)

### Prototipo (Reference)

- [Orion 1.0 - internal/stream/rtsp.go](../../../References/orion-prototipe/internal/stream/rtsp.go)
- [Wiki - Stream Providers](../../../VAULT/wiki/2.2-stream-providers.md)

---

## 🎸 Design Philosophy

**Bounded Context Enforcement**:
- Este módulo ES Stream Acquisition, NADA más
- Anti-responsibilities tan importantes como responsibilities
- Public API es contrato, `internal/` es implementación

**Complejidad por Diseño**:
- Atacar complejidad con arquitectura, no código complicado
- Cada archivo < 150 líneas (SRP enforcement)
- Documentar decisiones (ADR style)

**Pragmatismo > Purismo**:
- KISS: 5s warm-up hardcoded (no over-configuración)
- KISS Auto-Recovery: 5 retries → manual intervention
- Drop frames > queue (latencia > completitud)

---

**Last Updated**: 2025-11-03
**Authors**: Ernesto (Visiona) + Gaby (AI Companion)
**Status**: 🔄 Living Document (se actualiza durante implementación)
