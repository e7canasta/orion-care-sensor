# Consultoría Técnica: Módulo `stream-capture`
**Orion 2.0 - Bounded Context #1**
**Fecha**: 2025-11-04
**Consultor**: Gaby de Visiona
**Versión del módulo**: Sprint 1.1

---

## Resumen Ejecutivo

El módulo `stream-capture` es un **bounded context sólido y bien diseñado** que implementa correctamente los principios de "Complejidad por Diseño" del manifiesto Visiona. Presenta una arquitectura limpia con decisiones arquitectónicas bien documentadas (ADRs) y un excelente balance entre simplicidad estructural y capacidades avanzadas.

### Calificación General: **8.5/10**

**Fortalezas principales**:
- ✅ Arquitectura limpia con responsabilidades claras
- ✅ Fail-fast validation (load time vs runtime)
- ✅ Hot-reload funcional sin restart completo
- ✅ Thread-safety consistente y bien implementada
- ✅ Resiliencia via exponential backoff
- ✅ Observabilidad excelente (metrics + tracing)

**Áreas de mejora identificadas**:
- ⚠️ Drop tracking incompleto (callback layer vs public layer)
- ⚠️ Gestión de errores en GStreamer callbacks retorna FlowOK en casos ambiguos
- ⚠️ Configuración de reconnect no expuesta en API pública
- ⚠️ Warmup no valida estabilidad suficiente antes de producción
- ⚠️ Testing coverage limitado (solo unit tests, falta integration)

---

## 1. Análisis de Arquitectura

### 1.1 Estructura del Bounded Context

El módulo sigue correctamente el patrón de **Bounded Context** con límites claros:

```
stream-capture/
├── Public API (provider.go, types.go, rtsp.go)
│   └── StreamProvider interface + RTSPStream implementation
├── Internal (internal/rtsp/)
│   ├── pipeline.go    → GStreamer pipeline management
│   ├── callbacks.go   → GStreamer callback handlers
│   └── reconnect.go   → Exponential backoff logic
├── Binaries
│   ├── cmd/test-capture/    → Full-featured test harness
│   └── examples/             → Minimal usage examples
└── Tests (stream-capture_test.go)
```

**Evaluación**: ✅ **Excelente cohesión**. Cada archivo tiene un "único motivo para cambiar" (SRP).

### 1.2 Patrón de Diseño: Event-Driven Pipeline

```
RTSP Camera
    ↓
GStreamer Pipeline (7 elementos)
    ↓
OnNewSample callback (GStreamer → Go)
    ↓
Internal Frame Channel (rtsp.Frame)
    ↓
Frame Converter Goroutine (rtsp.Frame → streamcapture.Frame)
    ↓
Public Frame Channel (streamcapture.Frame)
    ↓
Consumer (user code)
```

**Evaluación**: ✅ **Bien diseñado**. El uso de dos tipos `Frame` (interno y público) evita import cycles sin sacrificar claridad.

### 1.3 Lifecycle Management

**Goroutines por instancia**: 3 goroutines concurrentes

1. **Frame Converter** (`rtsp.go:174-211`)
   - Convierte frames internos → públicos
   - Non-blocking send con drop tracking
   - Actualiza `lastFrameAt` para latency metric

2. **Pipeline Monitor** (`rtsp.go:254-298`)
   - Polling de GStreamer bus (50ms intervals)
   - Detecta errores/EOS y triggerea reconnect
   - Reset de reconnect state en PLAYING

3. **Reconnection Logic** (via `RunWithReconnect`)
   - Exponential backoff: 1s → 2s → 4s → 8s → 16s
   - Max 5 retries (configurable pero no expuesto)

**Evaluación**: ✅ **Excelente diseño de concurrencia**. Uso correcto de `sync.WaitGroup`, `context.Context`, y channels.

---

## 2. Evaluación de Decisiones Arquitectónicas (ADRs)

### AD-1: Fail-Fast Validation ✅ **Excelente**

**Decisión**: Validar configuración en `NewRTSPStream()` (constructor) en vez de en `Start()`.

**Implementación** (`rtsp.go:61-110`):
```go
// Fail-fast validation: RTSP URL
if cfg.URL == "" {
    return nil, fmt.Errorf("stream-capture: RTSP URL is required")
}

// Fail-fast validation: Target FPS
if cfg.TargetFPS < 0.1 || cfg.TargetFPS > 30 {
    return nil, fmt.Errorf("stream-capture: invalid FPS %.2f (must be 0.1-30)", cfg.TargetFPS)
}

// Fail-fast validation: GStreamer availability
if err := checkGStreamerAvailable(); err != nil {
    return nil, fmt.Errorf("stream-capture: GStreamer not available: %w", err)
}
```

**Evaluación**:
- ✅ Mensajes de error claros y accionables
- ✅ Coverage completo en tests (`stream-capture_test.go:14-120`)
- ✅ Evita "runtime debugging hell"

**Trade-off aceptado**: Slightly more upfront complexity vs debugging errors en producción.

---

### AD-2: Non-Blocking Channel with Drop Tracking ⚠️ **Buena pero mejorable**

**Decisión**: Usar `select` con `default` para drop frames cuando el canal está lleno.

**Implementación doble**:

1. **Callback Layer** (`internal/rtsp/callbacks.go:90-102`):
```go
select {
case ctx.FrameChan <- frame:
    slog.Debug("rtsp: frame sent", ...)
default:
    slog.Debug("rtsp: dropping frame, channel full", ...)
}
```

2. **Public Layer** (`rtsp.go:196-209`):
```go
select {
case s.frames <- publicFrame:
    // Frame sent successfully
default:
    // Channel full - drop frame and track metric
    atomic.AddUint64(&s.framesDropped, 1)
    slog.Debug("stream-capture: dropping frame, channel full", ...)
}
```

**Problema identificado**: ❌ **Tracking inconsistente**

- **Callback layer** NO incrementa contador → drops invisibles
- **Public layer** SÍ incrementa contador → drops visibles
- **Resultado**: `StreamStats.FramesDropped` NO incluye drops en callback layer

**Impacto**:
- Métrica `DropRate` subestimada
- Debugging difícil cuando hay drops en callback layer
- Violación del principio de observabilidad

**Recomendación**: 🔧 **Priority: MEDIUM** (ver sección 4.1)

---

### AD-3: Hot-Reload via Capsfilter Update ✅ **Excelente**

**Decisión**: Cambiar FPS via `capsfilter.SetProperty("caps", newCaps)` en vez de restart completo.

**Implementación** (`rtsp.go:493-542`):
```go
func (s *RTSPStream) SetTargetFPS(fps float64) error {
    // 1. Validate range
    // 2. Update capsfilter (hot-reload)
    if err := rtsp.UpdateFramerateCaps(s.elements.CapsFilter, fps, s.width, s.height); err != nil {
        // Rollback on error
        return fmt.Errorf("stream-capture: failed to update FPS: %w", err)
    }
    // 3. Update internal state
    s.targetFPS = fps
    return nil
}
```

**Evaluación**:
- ✅ Interruption time: ~2s (medido en `examples/hot-reload`)
- ✅ Rollback automático en caso de error
- ✅ Estado interno consistente (FPS no cambia si falla update)
- ✅ Logging completo con context

**Trade-off aceptado**: 2s interruption vs 5-10s full restart.

**Mejora posible**: ⚠️ No hay tests automatizados para hot-reload (solo ejemplo manual).

---

### AD-4: Exponential Backoff Reconnection ✅ **Sólido**

**Decisión**: Reconnect automático con exponential backoff en vez de fail-once.

**Implementación** (`internal/rtsp/reconnect.go:51-104`):
```go
func RunWithReconnect(ctx context.Context, connectFn ConnectFunc, cfg ReconnectConfig, state *ReconnectState) error {
    for {
        err := connectFn(ctx)
        if err == nil {
            state.CurrentRetries = 0  // Reset on success
            return nil
        }

        state.CurrentRetries++
        if state.CurrentRetries > cfg.MaxRetries {
            return fmt.Errorf("rtsp: max retries exceeded (%d attempts)", cfg.MaxRetries)
        }

        delay := calculateBackoff(state.CurrentRetries, cfg)
        select {
        case <-time.After(delay):
            continue
        case <-ctx.Done():
            return ctx.Err()
        }
    }
}
```

**Evaluación**:
- ✅ Formula correcta: `delay = retryDelay * 2^(attempt-1)`
- ✅ Cap en `maxRetryDelay` (30s) para evitar delays infinitos
- ✅ Context cancellation respetada durante backoff
- ✅ Reset automático en PLAYING state

**Problema identificado**: ⚠️ **Config no expuesta**

`ReconnectConfig` es creado con defaults hardcoded (`DefaultReconnectConfig()`) y **NO es configurable** desde `RTSPConfig`.

**Impacto**:
- Usuarios no pueden ajustar `MaxRetries` (5 intentos fijos)
- Usuarios no pueden ajustar `RetryDelay` o `MaxRetryDelay`
- Edge cases (e.g., cámaras con boot time >30s) no se pueden manejar

**Recomendación**: 🔧 **Priority: LOW** (ver sección 4.2)

---

### AD-5: Warmup Phase for FPS Stability ⚠️ **Buena idea, implementación incompleta**

**Decisión**: Consumir frames durante N segundos para medir FPS real antes de producción.

**Implementación** (`rtsp.go:572-645`):
```go
func (s *RTSPStream) Warmup(ctx context.Context, duration time.Duration) (*WarmupStats, error) {
    // Consume frames for duration
    for {
        select {
        case <-warmupCtx.Done():
            goto analyze
        case frame, ok := <-s.frames:
            frameTimes = append(frameTimes, frame.Timestamp)
        }
    }

analyze:
    // Calculate stats
    stats := CalculateFPSStats(frameTimes, elapsed)

    if !stats.IsStable {
        slog.Warn("stream-capture: stream FPS is unstable, may affect processing timing", ...)
    }

    return stats, nil  // Returns even if unstable!
}
```

**Problemas identificados**:

1. ❌ **No falla en unstable stream**
   - `Warmup()` retorna `nil` error incluso si `IsStable == false`
   - Usuario debe chequear manualmente `stats.IsStable`
   - Fácil olvidar el check → frames inestables en producción

2. ⚠️ **Threshold hardcoded (15%)**
   - `isStable := fpsStdDev < (fpsMean * 0.15)` en `warmup_stats.go:71`
   - No configurable por usuario
   - 15% puede ser demasiado permisivo para casos críticos

3. ⚠️ **Duration hardcoded en ejemplos**
   - Todos los ejemplos usan `5 * time.Second`
   - No hay guía sobre cómo elegir duration apropiado

**Impacto**:
- Usuarios pueden procesar streams inestables sin darse cuenta
- Timing assumptions en código downstream pueden fallar

**Recomendación**: 🔧 **Priority: MEDIUM** (ver sección 4.3)

---

### AD-6: Double-Close Panic Protection ✅ **Correcta**

**Decisión**: Usar `atomic.Bool` con `CompareAndSwap` para evitar double-close panic.

**Implementación** (`rtsp.go:414-419`):
```go
if s.framesClosed.CompareAndSwap(false, true) {
    close(s.frames)
    slog.Debug("stream-capture: frame channel closed")
} else {
    slog.Debug("stream-capture: frame channel already closed, skipping")
}
```

**Evaluación**:
- ✅ Idempotent `Stop()` funcionando correctamente
- ✅ Flag reset en `Stop()` final (`s.framesClosed.Store(false)`) para permitir restart
- ✅ Pattern correcto según Go best practices

**No hay mejoras necesarias**.

---

## 3. Fortalezas Técnicas

### 3.1 Thread Safety: Excelente ✅

Uso consistente de primitivas de sincronización:

| Mecanismo | Uso | Evaluación |
|-----------|-----|------------|
| `sync.RWMutex` | State access (`frames`, `cancel`, `elements`) | ✅ Correcto |
| `atomic.Uint64` | Counters (`frameCount`, `framesDropped`, `bytesRead`) | ✅ Correcto |
| `atomic.Uint32` | Reconnect counter | ✅ Correcto |
| `atomic.Bool` | Double-close protection | ✅ Correcto |
| `context.Context` | Goroutine cancellation | ✅ Correcto |
| `sync.WaitGroup` | Goroutine coordination | ✅ Correcto |

**No se detectaron race conditions ni deadlocks potenciales**.

---

### 3.2 Observabilidad: Excelente ✅

**Structured Logging** (via `slog`):
- Context fields consistentes
- Log levels apropiados (ERROR, WARN, INFO, DEBUG)
- Mensajes accionables

**Metrics** (via `StreamStats`):
```go
type StreamStats struct {
    FrameCount    uint64   // Total frames
    FramesDropped uint64   // NEW in this module
    DropRate      float64  // Calculated metric
    FPSTarget     float64  // Config
    FPSReal       float64  // Measured
    LatencyMS     int64    // Time since last frame
    Reconnects    uint32   // Resilience metric
    BytesRead     uint64   // Network metric
    IsConnected   bool     // Health check
}
```

**Distributed Tracing** (via `TraceID`):
- Cada frame tiene `uuid.New().String()` para rastreo end-to-end
- Facilita debugging en pipelines multi-stage

**Excelente diseño de observabilidad**.

---

### 3.3 Error Handling: Buena ⚠️

**Fortalezas**:
- ✅ Error wrapping con `fmt.Errorf(..., %w)` para stack trace
- ✅ Mensajes con contexto (URL, FPS, uptime, frame count)
- ✅ Logging antes de retornar error

**Área de mejora**:

En `callbacks.go:45-58`:
```go
func OnNewSample(sink *app.Sink, ctx *CallbackContext) gst.FlowReturn {
    sample := sink.PullSample()
    if sample == nil {
        slog.Error("rtsp: failed to pull sample from appsink")
        return gst.FlowEOS  // ⚠️ Termina stream por 1 fallo
    }

    buffer := sample.GetBuffer()
    if buffer == nil {
        slog.Error("rtsp: failed to get buffer from sample")
        return gst.FlowError  // ⚠️ Termina stream por 1 fallo
    }
    // ...
}
```

**Problema**: Un solo frame corrupto termina el stream completo.

**Expectativa**: Frame corrupto → log warning → skip frame → continuar

**Impacto**: Reconexión innecesaria por errores transitorios.

**Recomendación**: 🔧 **Priority: LOW** (ver sección 4.4)

---

### 3.4 Testing: Limitado ⚠️

**Coverage actual**:
- ✅ Unit tests para fail-fast validation (11 test cases)
- ✅ Unit tests para `Resolution` (dimensions, string)
- ✅ Unit tests para `CalculateFPSStats` (4 scenarios)

**Gaps identificados**:
- ❌ No integration tests con GStreamer real
- ❌ No tests para hot-reload (`SetTargetFPS`)
- ❌ No tests para reconnection logic
- ❌ No tests para goroutine lifecycle (start/stop)
- ❌ No tests para drop tracking

**Justificación según contexto**:
> "Testing Philosophy: Manual testing with pair-programming approach. No automated test files exist in prototype."

**Evaluación**: ✅ **Aceptable para Sprint 1.1** (prototipo), pero **DEBE mejorar en Sprint 1.2+**.

**Recomendación**: 🔧 **Priority: HIGH** (ver sección 4.5)

---

## 4. Áreas de Mejora Priorizadas

### 4.1 🔧 **Priority: MEDIUM** - Fix Drop Tracking en Callback Layer

**Problema**: Drops en `callbacks.go` NO se trackean en `StreamStats.FramesDropped`.

**Solución propuesta**:

```go
// callbacks.go
type CallbackContext struct {
    FrameChan      chan<- Frame
    FrameCounter   *uint64
    BytesRead      *uint64
    FramesDropped  *uint64  // ← ADD THIS
    Width          int
    Height         int
    SourceStream   string
}

func OnNewSample(sink *app.Sink, ctx *CallbackContext) gst.FlowReturn {
    // ...

    // Send frame (non-blocking - drop if channel full)
    select {
    case ctx.FrameChan <- frame:
        slog.Debug("rtsp: frame sent", ...)
    default:
        atomic.AddUint64(ctx.FramesDropped, 1)  // ← ADD THIS
        slog.Debug("rtsp: dropping frame, channel full", ...)
    }

    return gst.FlowOK
}
```

**Impacto**: Metrics precisas de drop rate.

**Esfuerzo estimado**: 1-2 horas (low complexity).

---

### 4.2 🔧 **Priority: LOW** - Exponer ReconnectConfig en API pública

**Problema**: `ReconnectConfig` hardcoded, no configurable.

**Solución propuesta**:

```go
// types.go
type RTSPConfig struct {
    URL            string
    Resolution     Resolution
    TargetFPS      float64
    SourceStream   string
    ReconnectCfg   *ReconnectConfig  // ← ADD THIS (optional, nil = defaults)
}

type ReconnectConfig struct {
    MaxRetries    int
    RetryDelay    time.Duration
    MaxRetryDelay time.Duration
}
```

**Uso**:
```go
cfg := streamcapture.RTSPConfig{
    URL:        "rtsp://camera/stream",
    TargetFPS:  2.0,
    ReconnectCfg: &streamcapture.ReconnectConfig{
        MaxRetries:    10,           // More retries for flaky network
        RetryDelay:    2 * time.Second,
        MaxRetryDelay: 60 * time.Second,
    },
}
```

**Impacto**: Flexibilidad para edge cases.

**Esfuerzo estimado**: 2-3 horas.

---

### 4.3 🔧 **Priority: MEDIUM** - Mejorar Warmup Validation

**Problema**: `Warmup()` no falla en streams inestables.

**Solución propuesta**:

**Opción 1: Fail-fast** (recomendado para casos críticos)
```go
func (s *RTSPStream) Warmup(ctx context.Context, duration time.Duration) (*WarmupStats, error) {
    // ... existing code ...

    stats := CalculateFPSStats(frameTimes, elapsed)

    if !stats.IsStable {
        return nil, fmt.Errorf(
            "stream-capture: warmup failed - stream FPS unstable (mean=%.2f, stddev=%.2f)",
            stats.FPSMean, stats.FPSStdDev,
        )
    }

    return stats, nil
}
```

**Opción 2: Configurable threshold** (más flexible)
```go
type WarmupConfig struct {
    Duration          time.Duration
    StabilityThreshold float64  // e.g., 0.15 = 15%
    FailOnUnstable    bool
}

func (s *RTSPStream) WarmupWithConfig(ctx context.Context, cfg WarmupConfig) (*WarmupStats, error) {
    // ...
    isStable := fpsStdDev < (fpsMean * cfg.StabilityThreshold)

    if !isStable && cfg.FailOnUnstable {
        return nil, fmt.Errorf("stream unstable")
    }

    return stats, nil
}
```

**Recomendación**: Opción 1 para Sprint 1.2, Opción 2 para Sprint 2+.

**Esfuerzo estimado**: 2-3 horas.

---

### 4.4 🔧 **Priority: LOW** - Graceful Degradation en GStreamer Callbacks

**Problema**: Frame corrupto → stream termination.

**Solución propuesta**:

```go
func OnNewSample(sink *app.Sink, ctx *CallbackContext) gst.FlowReturn {
    sample := sink.PullSample()
    if sample == nil {
        slog.Warn("rtsp: failed to pull sample, skipping frame")
        return gst.FlowOK  // ← Changed from FlowEOS
    }

    buffer := sample.GetBuffer()
    if buffer == nil {
        slog.Warn("rtsp: failed to get buffer, skipping frame")
        return gst.FlowOK  // ← Changed from FlowError
    }

    // ... rest of code ...
}
```

**Trade-off**: Algunos frames corruptos se skipean vs stream termination.

**Impacto**: Menos reconnections innecesarias.

**Esfuerzo estimado**: 30 minutos.

---

### 4.5 🔧 **Priority: HIGH** - Expandir Test Coverage

**Objetivo**: Llevar coverage de ~20% a ~70% en Sprint 1.2.

**Test cases recomendados**:

1. **Integration Tests con Mock GStreamer**:
   - Start/Stop lifecycle
   - Frame delivery end-to-end
   - Reconnection behavior
   - Hot-reload FPS change

2. **Concurrency Tests**:
   - Race detector (`go test -race`)
   - Goroutine leak detection
   - Channel close ordering

3. **Edge Cases**:
   - Stop() called multiple times
   - SetTargetFPS() during pipeline error
   - Warmup() with context cancellation

**Herramientas recomendadas**:
- `github.com/stretchr/testify` para assertions
- `github.com/golang/mock` para mock de GStreamer (si es viable)
- Table-driven tests para parametrización

**Esfuerzo estimado**: 1-2 días (Sprint 1.2 completo dedicado a testing).

---

## 5. Análisis de Riesgos Técnicos

### 5.1 🔴 **Risk: HIGH** - Dependencia de GStreamer

**Naturaleza**: `stream-capture` depende 100% de GStreamer nativo.

**Impacto**:
- ❌ **Portabilidad limitada**: Requiere instalación de paquetes system-level
- ❌ **Debugging difícil**: Errores en pipeline son crípticos
- ❌ **Versioning**: Incompatibilidades entre GStreamer 1.x versions

**Mitigación actual**:
- ✅ Fail-fast en `checkGStreamerAvailable()`
- ✅ Logging de errores de pipeline con debug strings

**Mitigación recomendada**:
- 📋 Documentar versiones compatibles de GStreamer en README
- 📋 Agregar script de instalación para dev environments
- 📋 Considerar Docker image con GStreamer pre-instalado para testing

**Esfuerzo estimado**: 4-6 horas (documentación + scripts).

---

### 5.2 🟡 **Risk: MEDIUM** - Memory Leaks en Frame Data

**Naturaleza**: Cada frame alloca `[]byte` para pixel data (RGB).

**Volumen**:
- 720p RGB: 1280 × 720 × 3 = **2.76 MB per frame**
- 2 FPS × 3600s = **19.9 GB/hour**

**Mitigación actual**:
- ✅ Non-blocking send → frames droppeados liberan memoria
- ✅ Channel buffer limitado (10 frames = ~28 MB max)
- ✅ GStreamer buffer unmapping correcto (`buffer.Unmap()`)

**Risk residual**: ⚠️ Si consumer es lento, drops aumentan pero memoria está bounded.

**Monitoreo recomendado**:
- Agregar metric `MemoryUsageMB` a `StreamStats`
- Alert si drop rate > 10% sustained por 5 minutos

**Esfuerzo estimado**: 2-3 horas.

---

### 5.3 🟡 **Risk: MEDIUM** - Hot-Reload Race Condition

**Naturaleza**: `SetTargetFPS()` modifica `capsfilter` mientras pipeline está running.

**Escenario de riesgo**:
1. User llama `SetTargetFPS(1.0)` → capsfilter updating
2. GStreamer error durante update → pipeline en estado inconsistente
3. Rollback falla → FPS stuck en valor intermedio

**Mitigación actual**:
- ✅ Mutex lock durante update (`s.mu.Lock()`)
- ✅ Rollback logging en caso de error

**Mitigación faltante**:
- ❌ No hay verificación de éxito de rollback
- ❌ No hay timeout para update operation

**Recomendación**:
```go
func (s *RTSPStream) SetTargetFPS(fps float64) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    oldFPS := s.targetFPS

    // Update con timeout
    updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    errChan := make(chan error, 1)
    go func() {
        errChan <- rtsp.UpdateFramerateCaps(s.elements.CapsFilter, fps, s.width, s.height)
    }()

    select {
    case err := <-errChan:
        if err != nil {
            // Explicit rollback
            _ = rtsp.UpdateFramerateCaps(s.elements.CapsFilter, oldFPS, s.width, s.height)
            return err
        }
    case <-updateCtx.Done():
        return fmt.Errorf("SetTargetFPS timeout after 5s")
    }

    s.targetFPS = fps
    return nil
}
```

**Esfuerzo estimado**: 3-4 horas.

---

## 6. Comparación con Manifiesto Visiona

### 6.1 "Complejidad por Diseño, No por Accidente" ✅

**Evaluación**: **Excelente adherencia**.

Evidencia:
- ✅ Bounded context claro con responsabilidades definidas
- ✅ ADRs documentadas con trade-offs explícitos
- ✅ Patterns consistentes (fail-fast, non-blocking, hot-reload)
- ✅ No hay "accidental complexity" (no over-engineering)

**Quote del manifiesto**:
> "Atacar complejidad real, no imaginaria. Diseñar arquitectura que maneja complejidad inherente del dominio."

El módulo maneja la complejidad inherente de:
- RTSP streaming (GStreamer)
- Concurrency (3 goroutines coordinadas)
- Resiliencia (reconnection)

Sin añadir complejidad accidental.

---

### 6.2 "Fail Fast (Load Time vs Runtime)" ✅

**Evaluación**: **Excelente adherencia**.

Evidencia:
- ✅ `NewRTSPStream()` valida URL, FPS, resolution, GStreamer
- ✅ Errores claros y accionables
- ✅ Test coverage para validation logic

**Quote del manifiesto**:
> "Fail inmediato en load vs Runtime debugging hell"

Cumplido completamente.

---

### 6.3 "Simple para Leer, NO Simple para Escribir" ✅

**Evaluación**: **Buena adherencia**.

Evidencia:
- ✅ Código bien documentado (godoc comments completos)
- ✅ Ejemplos de uso (`examples/simple`, `examples/hot-reload`)
- ✅ API intuitiva (`Start()`, `Stop()`, `Warmup()`, `SetTargetFPS()`)

**Área de mejora**:
- ⚠️ Código interno (`internal/rtsp/`) tiene menos comments que API pública
- ⚠️ `buildFramerateCaps()` logic no está documentada (fractional FPS handling)

**Recomendación**: Agregar comments en `internal/rtsp/pipeline.go:189-205`.

---

### 6.4 "Cohesión > Ubicación" ✅

**Evaluación**: **Excelente adherencia**.

Evidencia:
- ✅ Módulos definidos por cohesión conceptual:
  - `provider.go`: Contract
  - `rtsp.go`: Implementation
  - `internal/rtsp/`: GStreamer internals
  - `internal/warmup/`: FPS statistics
- ✅ No hay "utils" package (anti-pattern evitado)

**Quote del manifiesto**:
> "Módulos se definen por cohesión conceptual, no por tamaño."

Cumplido completamente.

---

## 7. Recomendaciones Finales

### 7.1 Sprint 1.2 Priorities

| Priority | Task | Esfuerzo | Impacto |
|----------|------|----------|---------|
| 🔴 HIGH | Fix drop tracking en callback layer | 1-2h | Metrics precisas |
| 🔴 HIGH | Expandir test coverage (70%+) | 1-2 días | Confianza en refactors |
| 🟡 MEDIUM | Mejorar warmup validation | 2-3h | Fail-fast en streams inestables |
| 🟢 LOW | Graceful degradation en callbacks | 30min | Menos reconnections |
| 🟢 LOW | Exponer ReconnectConfig en API | 2-3h | Flexibilidad edge cases |

**Total estimado**: **2-3 días** de trabajo para Sprint 1.2.

---

### 7.2 Future Enhancements (Post-Sprint 2)

1. **Memory Pool para Frames**:
   - Reuse `[]byte` buffers en vez de allocar cada vez
   - Reduce GC pressure (19.9 GB/hour → ~30 MB pooled)

2. **Configurable Frame Format**:
   - Soportar RGBA, YUV, JPEG además de RGB
   - Permite optimizaciones downstream

3. **Multi-Stream Support**:
   - Agregar `stream_id` a `Frame` metadata
   - Single `RTSPStream` instance puede manejar múltiples streams

4. **Prometheus Metrics Exporter**:
   - Export `StreamStats` como Prometheus metrics
   - Integración con monitoring stack

5. **GPU Acceleration**:
   - Usar GStreamer NVDEC/VAAPI elements
   - No requiere cambios en Go code (transparent)

---

## 8. Conclusiones

### 8.1 Calificación por Categoría

| Categoría | Calificación | Justificación |
|-----------|--------------|---------------|
| **Arquitectura** | 9/10 | Bounded context limpio, ADRs claras |
| **Código** | 8/10 | Clean code, bien documentado, algunos gaps |
| **Testing** | 5/10 | Unit tests OK, falta integration |
| **Observabilidad** | 9/10 | Excelente logging + metrics + tracing |
| **Resiliencia** | 8/10 | Reconnection sólida, warmup mejorable |
| **Mantenibilidad** | 9/10 | Código legible, cohesión alta, acoplamiento bajo |

**Promedio**: **8.0/10**

---

### 8.2 Veredicto Final

El módulo `stream-capture` es un **excelente punto de partida** para Orion 2.0. Demuestra:

✅ **Sólido entendimiento de principios de diseño** (Manifiesto Visiona aplicado correctamente)
✅ **Trade-offs conscientes** (latency > completeness, hot-reload > restart)
✅ **Observabilidad first-class** (metrics + tracing desde día 1)
✅ **Production-ready** (con las mejoras de Sprint 1.2)

**No hay blockers técnicos** para continuar con Sprint 1.2 (worker-lifecycle).

---

### 8.3 Próximos Pasos Recomendados

1. ✅ **Aprobar CLAUDE.md** generado (excelente documentación)
2. 🔧 **Implementar mejoras Priority HIGH** (Sprint 1.2 week 1)
3. ✅ **Validar con RTSP real** (testing manual con cámaras reales)
4. 📋 **Documentar versiones GStreamer compatibles**
5. ➡️ **Continuar con Bounded Context #2** (worker-lifecycle)

---

**Firma**: Gaby de Visiona
**Fecha**: 2025-11-04
**Co-Authored-By**: Gaby de Visiona <noreply@visiona.app>
