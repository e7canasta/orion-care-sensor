🎸 Diseño: Stream Capture Module (Sprint 1.1)

1⃣ Lecciones del Prototipo que MANTENEMOS

✅ Hot-reload de FPS sin reiniciar el pipeline
- SetTargetFPS() actualiza capsfilter en runtime
- Maneja fracciones (0.5 Hz → framerate=1/2)
- ~2 segundos de interrupción vs 5-10 de restart completo

✅ Reconexión con exponential backoff
- 5 reintentos máximo
- 1s → 2s → 4s → 8s → 16s (cap 30s)
- Reset currentRetries al conectar exitosamente

✅ GStreamer pipeline no-blocking
- bus.TimedPop(50ms) para shutdown responsive
- No usa glib.MainLoop (evita complejidad)
- app.Sink con callbacks

✅ Non-blocking channel sends
- Buffer de 10 frames
- Drop en lugar de queue (mantener latencia)

✅ Atomic stats tracking
- frameCount, reconnects, bytesRead
- Lock-free desde callback thread

---
2⃣ Bounded Context: Stream Acquisition

Responsabilidades:
- ✅ Capturar frames RTSP via GStreamer
- ✅ Reconexión automática con backoff
- ✅ FPS adaptativo (hot-reload)
- ✅ Warm-up con medición de FPS real

Anti-responsabilidades:
- ❌ NO procesa frames (eso es FrameBus)
- ❌ NO decide qué capturar (eso es Control Plane)
- ❌ NO sabe de workers (eso es Worker Lifecycle)

---
3⃣ Public API del Módulo

```
// modules/stream-capture/provider.go
package streamcapture

type StreamProvider interface {
    Start(ctx context.Context) (<-chan Frame, error)
    Stop() error
    Stats() StreamStats
    SetTargetFPS(fps float64) error  // Hot-reload support
}

type RTSPConfig struct {
    URL          string
    Resolution   Resolution  // 512p, 720p, 1080p
    TargetFPS    float64
    SourceStream string      // "LQ", "HQ"
}

```


Diferencias con prototipo:
- ✅ Start() retorna channel directamente (más simple)
- ✅ Resolution es tipo enumerado (no int width/height)
- ✅ No expone pipeline, capsfilter (implementation detail)

---
4⃣ Estructura Interna del Módulo

```
modules/stream-capture/
├── go.mod                   # Módulo independiente
├── CLAUDE.md                # Guía para AI (bounded context)
├── README.md                # Human-readable overview
├── BACKLOG.md               # Sprint 1.1 tasks
├── docs/
│   ├── DESIGN.md            # Decisiones arquitectónicas
│   └── proposals/           # RFCs antes de implementar
│
├── provider.go              # Public API (StreamProvider interface)
├── rtsp.go                  # RTSPStream implementation
├── types.go                 # Frame, StreamStats, RTSPConfig
├── stream-capture_test.go   # Tests públicos
│
└── internal/
    ├── rtsp/
    │   ├── pipeline.go      # GStreamer pipeline setup
    │   ├── reconnect.go     # Exponential backoff logic
    │   └── callbacks.go     # onNewSample, pad-added
    └── warmup/
        ├── warmup.go        # Warm-up phase (5s)
        └── stats.go         # FPS statistics calculation
```

Razón de internal/:
- SRP: GStreamer pipeline setup es responsabilidad separada
- Testability: Mock internal/rtsp para tests unitarios
- Boundary enforcement: API pública estable, internals pueden cambiar

---
5⃣ Complejidad por Diseño: Separación de Concerns

Antes (prototipo): Monolito en rtsp.go (513 líneas)

rtsp.go:
- RTSPStream struct (6 categorías de estado)
- NewRTSPStream
- Start/Stop lifecycle
- SetTargetFPS (hot-reload)
- runPipeline (reconnection loop)
- connectAndStream (GStreamer setup completo)
- onNewSample (callback)
- Stats

Después (Orion 2.0): Atacar complejidad con arquitectura

provider.go (40 líneas):
- StreamProvider interface
- Constructor NewRTSPStream

rtsp.go (150 líneas):
- RTSPStream struct
- Start/Stop/Stats/SetTargetFPS (lifecycle public)
- Delegation a internal/rtsp

internal/rtsp/pipeline.go (100 líneas):
- CreatePipeline(config) → *gst.Pipeline
- UpdateFramerateCapschlfilter, fps)
- DestroyPipeline()

internal/rtsp/reconnect.go (80 líneas):
- RunWithReconnect(ctx, connectFn) error
- Exponential backoff logic
- Retry counting

internal/rtsp/callbacks.go (60 líneas):
- OnNewSample(sink, frameChan)
- OnPadAdded(srcPad, sinkElement)

internal/warmup/warmup.go (80 líneas):
- WarmupStream(frames, duration)
- FPS measurement logic

internal/warmup/stats.go (60 líneas):
- calculateFPSStats()
- CalculateOptimalInferenceRate()

Beneficio:
- ✅ Cada archivo < 150 líneas (fácil de leer)
- ✅ Un "motivo para cambiar" por archivo (SRP)
- ✅ Testeable en aislación (mock pipeline, mock reconnect)
- ✅ Reutilizable (reconnect logic se puede usar en otros módulos)

---
6⃣ Types: Frame & StreamStats

```
// modules/stream-capture/types.go
package streamcapture

type Frame struct {
    Seq          uint64
    Timestamp    time.Time
    Width        int
    Height       int
    Data         []byte      // RGB format from GStreamer
    SourceStream string      // "LQ", "HQ"
    TraceID      string      // Distributed tracing
}

type StreamStats struct {
    FrameCount   uint64
    FPSTarget    float64
    FPSReal      float64
    LatencyMS    int64
    SourceStream string
    Resolution   string
    Reconnects   uint32
    BytesRead    uint64
    IsConnected  bool
}

type Resolution int

const (
    Res512p  Resolution = iota  // 910x512
    Res720p                      // 1280x720
    Res1080p                     // 1920x1080
)

func (r Resolution) Dimensions() (width, height int) {
    switch r {
    case Res512p:
        return 910, 512
    case Res720p:
        return 1280, 720
    case Res1080p:
        return 1920, 1080
    default:
        return 1280, 720
    }
}
```


Diferencias con prototipo:
- ✅ NO incluye ROIProcessing (eso es módulo framebus)
- ✅ Resolution es enum (fail fast en load time)
- ✅ Frame ownership claro (este módulo lo crea, otros lo consumen)

---
7⃣ Fail Fast: Validación en Constructor

```
// modules/stream-capture/rtsp.go
func NewRTSPStream(cfg RTSPConfig) (*RTSPStream, error) {
    // Fail fast: validar en load time
    if cfg.URL == "" {
        return nil, fmt.Errorf("stream-capture: rtsp URL is required")
    }

    if cfg.TargetFPS <= 0 || cfg.TargetFPS > 30 {
        return nil, fmt.Errorf("stream-capture: invalid FPS %.2f (must be 0.1-30)", cfg.TargetFPS)
    }

    width, height := cfg.Resolution.Dimensions()
    if width == 0 || height == 0 {
        return nil, fmt.Errorf("stream-capture: invalid resolution %v", cfg.Resolution)
    }

    // Mensajes de error claros
    if err := checkGStreamerAvailable(); err != nil {
        return nil, fmt.Errorf("stream-capture: GStreamer not available: %w", err)
    }

    return &RTSPStream{...}, nil
}
```

Principio: "Fail inmediato en load vs Runtime debugging hell"

---
8⃣ Hot-Reload: SetTargetFPS Design

Prototipo funciona bien, lo mantenemos con mejoras:

```
// modules/stream-capture/rtsp.go
func (s *RTSPStream) SetTargetFPS(fps float64) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if fps <= 0 || fps > 30 {
        return fmt.Errorf("invalid FPS: %.2f (must be 0.1-30)", fps)
    }

    oldFPS := s.targetFPS
    s.targetFPS = fps

    // Delegate to internal/rtsp
    if s.capsfilter != nil {
        if err := rtsp.UpdateFramerateCaps(s.capsfilter, fps, s.width, s.height); err != nil {
            s.targetFPS = oldFPS  // Rollback on error
            return fmt.Errorf("failed to update FPS: %w", err)
        }
    }

    slog.Info("stream FPS updated", "old", oldFPS, "new", fps)
    return nil
}
```

Mejora sobre prototipo:
- ✅ Rollback en caso de error
- ✅ Lógica de caps en internal/rtsp/pipeline.go (SRP)

---
9⃣ Warm-up: Measurement Strategy

Prototipo ya lo hace bien, lo mejoramos con contexto:

```
// modules/stream-capture/rtsp.go
func (s *RTSPStream) Start(ctx context.Context) (<-chan Frame, error) {
    // 1. Start pipeline
    frameChan := make(chan Frame, 10)
    go s.runPipeline(ctx, frameChan)

    // 2. Warm-up phase (5 seconds)
    warmupStats, err := warmup.WarmupStream(ctx, frameChan, 5*time.Second)
    if err != nil {
        return nil, fmt.Errorf("warm-up failed: %w", err)
    }

    // 3. Log FPS stability
    if !warmupStats.IsStable {
        slog.Warn("stream FPS unstable",
            "fps_mean", warmupStats.FPSMean,
            "fps_stddev", warmupStats.FPSStdDev,
        )
    }

    return frameChan, nil
}
```

Decisión: Warm-up es transparente para el caller (ocurre dentro de Start)

---
🔟 Testing Strategy: Pair-Programming Approach

No automated tests (según filosofía del usuario), pero:

✅ Compilation tests (ALWAYS):
cd modules/stream-capture
go build ./...

✅ Manual integration tests (tú corres, yo observo):
1. Test RTSP connection (real camera)
2. Test reconnection (desconecta go2rtc)
3. Test hot-reload FPS (envía MQTT command)
4. Test warm-up stats (verifica logs)
5. Test graceful shutdown

✅ Quadrant approach:
- Q1: Unit (manual review de código)
- Q2: Integration (manual con MQTT)
- Q3: Acceptance (real RTSP camera)
- Q4: Performance (FPS stability, latency)

---
1⃣1⃣ Dependencies

```
// modules/stream-capture/go.mod
module github.com/e7canasta/orion-care-sensor/modules/stream-capture

go 1.22

require (
    github.com/tinyzimmer/go-gst v0.3.2  // GStreamer bindings
    github.com/google/uuid v1.6.0         // TraceID generation
)
```

Leaf module: No depende de otros módulos Orion (independiente)

---
1⃣2⃣ Próximos Pasos

¿Querés que:

A) Escriba el código completo del módulo empezando por provider.go y types.go?

B) Creemos primero el ADR (Architecture Decision Record) documentando estas decisiones antes de codear?

C) Actualicemos el BACKLOG.md del módulo con tasks específicas (TDD approach)?

D) Hagamos un diagrama de secuencia del flujo completo (Start → Warm-up → Frames → Stop)?

Decime por dónde arrancamos. 🎸

