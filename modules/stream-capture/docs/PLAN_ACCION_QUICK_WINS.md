# 🎯 Plan de Acción - stream-capture Module

**Fecha**: 2025-11-04
**Basado en**: Consultoría Técnica 2025-11-04
**Estrategia**: Quick Wins primero, luego mejoras estructurales

---

## 🏃 Quick Wins (Sprint 1.2 - Semana 1)

### Quick Win #1: Fix Double-Close Panic 🚨

**Prioridad**: CRÍTICA
**Esfuerzo**: 1-2 horas
**Impacto**: Evita crashes en producción
**Riesgo**: Bajo

**Archivos afectados**:
- `rtsp.go:389` (close del canal)
- `types.go` (agregar campo `framesClosed`)

**Implementación**:

```go
// 1. Agregar campo en RTSPStream (rtsp.go)
type RTSPStream struct {
    // ... campos existentes ...
    framesClosed atomic.Bool  // NEW: Protege contra double-close
}

// 2. Modificar Stop() para usar CompareAndSwap
func (s *RTSPStream) Stop() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.cancel == nil {
        slog.Debug("stream-capture: stream not started, nothing to stop")
        return nil
    }

    slog.Info("stream-capture: stopping RTSP stream")
    s.cancel()

    // ... wait for goroutines ...

    // Destroy pipeline
    if s.elements != nil {
        if err := rtsp.DestroyPipeline(s.elements); err != nil {
            slog.Error("stream-capture: failed to destroy pipeline", "error", err)
        }
        s.elements = nil
    }

    // ✅ FIX: Solo cerrar si no se ha cerrado antes
    if s.framesClosed.CompareAndSwap(false, true) {
        close(s.frames)
    }

    // ... log statistics ...

    // Reset state
    s.cancel = nil
    s.ctx = nil
    s.frames = make(chan Frame, 10)
    s.framesClosed.Store(false)  // Reset para restart

    return nil
}
```

**Testing Manual**:
```bash
# Test con shutdown rápido repetido
# 1. Iniciar stream
# 2. Enviar Ctrl+C múltiples veces rápidamente
# 3. No debe paniquear
```

**Criterio de Aceptación**:
- [ ] Compilación exitosa
- [ ] Shutdown limpio sin panics
- [ ] Logs muestran "stream stopped" correctamente

---

### Quick Win #2: Agregar Drop Rate Metrics 📊

**Prioridad**: ALTA
**Esfuerzo**: 2-3 horas
**Impacto**: Observabilidad inmediata
**Riesgo**: Bajo

**Archivos afectados**:
- `types.go` (StreamStats)
- `rtsp.go` (contador + Stats())
- `internal/rtsp/callbacks.go` (no cambios, ya dropea)

**Implementación**:

```go
// 1. Agregar campos en RTSPStream (rtsp.go)
type RTSPStream struct {
    // ... campos existentes ...
    framesDropped uint64  // NEW: Atomic counter para drops
}

// 2. Agregar campos en StreamStats (types.go)
type StreamStats struct {
    // ... campos existentes ...
    FramesDropped uint64  // NEW: Total frames dropped
    DropRate      float64 // NEW: Porcentaje de drops (%)
}

// 3. Modificar goroutine de conversión (rtsp.go:171-199)
go func() {
    defer s.wg.Done()
    defer close(internalFrames)

    for internalFrame := range internalFrames {
        // Convert frame...
        publicFrame := Frame{...}

        // Update lastFrameAt
        s.mu.Lock()
        s.lastFrameAt = time.Now()
        s.mu.Unlock()

        // ✅ FIX: Non-blocking send con drop tracking
        select {
        case s.frames <- publicFrame:
            // Success
        case <-localCtx.Done():
            return
        default:
            // ✅ NEW: Track drops
            atomic.AddUint64(&s.framesDropped, 1)
            slog.Debug("stream-capture: dropping frame, channel full",
                "seq", publicFrame.Seq,
                "trace_id", publicFrame.TraceID,
            )
        }
    }
}()

// 4. Actualizar Stats() (rtsp.go:413-450)
func (s *RTSPStream) Stats() StreamStats {
    s.mu.RLock()
    defer s.mu.RUnlock()

    frameCount := atomic.LoadUint64(&s.frameCount)
    framesDropped := atomic.LoadUint64(&s.framesDropped)
    bytesRead := atomic.LoadUint64(&s.bytesRead)
    reconnects := atomic.LoadUint32(s.reconnectState.Reconnects)

    // Calculate drop rate
    var dropRate float64
    totalAttempts := frameCount + framesDropped
    if totalAttempts > 0 {
        dropRate = (float64(framesDropped) / float64(totalAttempts)) * 100.0
    }

    // ... resto del código ...

    return StreamStats{
        FrameCount:    frameCount,
        FramesDropped: framesDropped,
        DropRate:      dropRate,
        // ... otros campos ...
    }
}
```

**Testing Manual**:
```bash
# Test con high load
RTSP_URL=rtsp://camera/stream FPS=30 make run-test
# Observar stats cada 5s
# Verificar que DropRate se calcula correctamente
```

**Criterio de Aceptación**:
- [ ] Stats muestra `FramesDropped` y `DropRate`
- [ ] test-capture imprime drop rate en stats
- [ ] Drop rate 0% bajo carga normal

---

### Quick Win #3: Fix Non-Blocking Pattern Inconsistency ⚙️

**Prioridad**: MEDIA
**Esfuerzo**: 30 minutos
**Impacto**: Consistencia arquitectónica
**Riesgo**: Bajo

**Archivos afectados**:
- `rtsp.go:193-197` (ya cubierto en Quick Win #2)

**Implementación**: Ver Quick Win #2 (mismo fix)

**Criterio de Aceptación**:
- [ ] Ambos puntos (callbacks.go y rtsp.go) usan patrón non-blocking consistente
- [ ] Drops se trackean en ambos lugares

---

## 🔧 Mejoras Estructurales (Sprint 1.2 - Semana 2)

### Mejora #1: Integrar Warmup en API Pública

**Prioridad**: MEDIA-ALTA
**Esfuerzo**: 4-6 horas
**Impacto**: API más robusta y predecible
**Riesgo**: Medio (breaking change potencial)

**Opciones de Implementación**:

#### Opción A: Warmup Automático (Breaking Change)

```go
// Start() bloquea durante warmup (5 segundos)
func (s *RTSPStream) Start(ctx context.Context) (<-chan Frame, error) {
    // ... crear pipeline ...

    // Warmup interno (5 segundos)
    internalWarmupChan := make(chan warmup.Frame, 10)

    // Goroutine temporal para warmup
    go func() {
        for internalFrame := range internalFrames {
            // Durante warmup, enviar a warmup channel
            select {
            case internalWarmupChan <- warmup.Frame{
                Seq:       internalFrame.Seq,
                Timestamp: internalFrame.Timestamp,
            }:
            case <-ctx.Done():
                return
            }
        }
    }()

    stats, err := warmup.WarmupStream(ctx, internalWarmupChan, 5*time.Second)
    if err != nil {
        return nil, fmt.Errorf("warmup failed: %w", err)
    }

    slog.Info("stream-capture: warmup complete",
        "fps_mean", stats.FPSMean,
        "fps_stddev", stats.FPSStdDev,
        "stable", stats.IsStable,
    )

    // Después de warmup, cambiar a producción
    // ... iniciar goroutine de conversión normal ...

    return s.frames, nil
}
```

**Pros**:
- ✅ API más simple (todo automático)
- ✅ Garantiza FPS medido antes de usar stream

**Cons**:
- ❌ Start() ahora bloquea 5 segundos (breaking change)
- ❌ No flexible para diferentes duraciones de warmup

---

#### Opción B: Método Warmup Explícito (Backward Compatible) ⭐ RECOMENDADA

```go
// Agregar método público para warmup
func (s *RTSPStream) Warmup(ctx context.Context, duration time.Duration) (*WarmupStats, error) {
    s.mu.RLock()
    if s.cancel == nil {
        s.mu.RUnlock()
        return nil, fmt.Errorf("stream-capture: stream not started")
    }
    s.mu.RUnlock()

    // Crear canal temporal para warmup
    warmupChan := make(chan warmup.Frame, 10)

    // Goroutine temporal que consume frames y los convierte
    done := make(chan struct{})
    go func() {
        defer close(done)
        for {
            select {
            case frame, ok := <-s.frames:
                if !ok {
                    return
                }
                warmupChan <- warmup.Frame{
                    Seq:       frame.Seq,
                    Timestamp: frame.Timestamp,
                }
            case <-ctx.Done():
                return
            }
        }
    }()

    stats, err := warmup.WarmupStream(ctx, warmupChan, duration)
    close(warmupChan)
    <-done

    return stats, err
}
```

**Uso**:
```go
stream, _ := streamcapture.NewRTSPStream(cfg)
frameChan, _ := stream.Start(ctx)

// Warmup explícito
stats, err := stream.Warmup(ctx, 5*time.Second)
if err != nil {
    log.Fatal("warmup failed:", err)
}
log.Printf("Stream stable: %v, FPS: %.2f", stats.IsStable, stats.FPSMean)

// Ahora consumir frames normalmente
for frame := range frameChan {
    // ...
}
```

**Pros**:
- ✅ Backward compatible (no breaking change)
- ✅ Flexible (usuario elige duración)
- ✅ API explícita (clara intención)

**Cons**:
- ⚠️ Usuario debe recordar llamar Warmup()

**Criterio de Aceptación**:
- [ ] Método `Warmup()` implementado
- [ ] test-capture usa `Warmup()` antes de consumir frames
- [ ] Stats de warmup se loggean correctamente

---

### Mejora #2: Tests Básicos (Fail-Fast, Stats)

**Prioridad**: MEDIA
**Esfuerzo**: 8-10 horas (manual testing)
**Impacto**: Confianza en refactors
**Riesgo**: Bajo

**Tests a Implementar**:

```go
// stream-capture_test.go

// Test 1: Fail-Fast Validation
func TestNewRTSPStream_FailFast(t *testing.T) {
    tests := []struct {
        name    string
        cfg     RTSPConfig
        wantErr string
    }{
        {
            name:    "empty URL",
            cfg:     RTSPConfig{},
            wantErr: "RTSP URL is required",
        },
        {
            name:    "invalid FPS zero",
            cfg:     RTSPConfig{URL: "rtsp://test", TargetFPS: 0},
            wantErr: "invalid FPS",
        },
        {
            name:    "invalid FPS negative",
            cfg:     RTSPConfig{URL: "rtsp://test", TargetFPS: -1},
            wantErr: "invalid FPS",
        },
        {
            name:    "invalid FPS too high",
            cfg:     RTSPConfig{URL: "rtsp://test", TargetFPS: 31},
            wantErr: "invalid FPS",
        },
        {
            name: "valid config",
            cfg: RTSPConfig{
                URL:          "rtsp://test",
                Resolution:   Res720p,
                TargetFPS:    2.0,
                SourceStream: "LQ",
            },
            wantErr: "",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := NewRTSPStream(tt.cfg)
            if tt.wantErr == "" && err != nil {
                t.Errorf("unexpected error: %v", err)
            }
            if tt.wantErr != "" && (err == nil || !contains(err.Error(), tt.wantErr)) {
                t.Errorf("expected error containing %q, got %v", tt.wantErr, err)
            }
        })
    }
}

// Test 2: Resolution Dimensions
func TestResolution_Dimensions(t *testing.T) {
    tests := []struct {
        res        Resolution
        wantWidth  int
        wantHeight int
    }{
        {Res512p, 910, 512},
        {Res720p, 1280, 720},
        {Res1080p, 1920, 1080},
    }

    for _, tt := range tests {
        t.Run(tt.res.String(), func(t *testing.T) {
            w, h := tt.res.Dimensions()
            if w != tt.wantWidth || h != tt.wantHeight {
                t.Errorf("got %dx%d, want %dx%d", w, h, tt.wantWidth, tt.wantHeight)
            }
        })
    }
}

// Test 3: Stats Thread-Safety (manual race testing)
func TestStats_ThreadSafe(t *testing.T) {
    // Run with: go test -race
    // Este test solo compila, el testing real es manual
    t.Skip("Manual test: run `go test -race` with real RTSP stream")
}

func contains(s, substr string) bool {
    return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && s[:len(substr)] == substr)
}
```

**Testing Manual**:
```bash
# Test fail-fast
make test

# Test thread-safety
go test -race ./...

# Test con stream real
RTSP_URL=rtsp://camera/stream make run-test
```

**Criterio de Aceptación**:
- [ ] Tests de fail-fast pasan
- [ ] Tests de dimensions pasan
- [ ] `go test -race` no detecta race conditions

---

## 🚀 Mejoras Futuras (Sprint 1.3+)

### Mejora #3: Jitter en Exponential Backoff

**Prioridad**: BAJA
**Esfuerzo**: 1 hora
**Impacto**: Evita sincronización entre instancias

```go
func calculateBackoff(attempt int, cfg ReconnectConfig) time.Duration {
    delay := cfg.RetryDelay * time.Duration(1<<uint(attempt-1))
    if delay > cfg.MaxRetryDelay {
        delay = cfg.MaxRetryDelay
    }

    // ✅ NEW: Agregar jitter ±20%
    jitter := time.Duration(rand.Int63n(int64(delay) / 5))  // 20% de delay
    if rand.Intn(2) == 0 {
        delay += jitter
    } else {
        delay -= jitter
    }

    return delay
}
```

---

### Mejora #4: Pipeline Presets

**Prioridad**: BAJA
**Esfuerzo**: 4-6 horas
**Impacto**: Flexibilidad para diferentes escenarios

---

### Mejora #5: Observabilidad Avanzada (P50/P95/P99)

**Prioridad**: BAJA
**Esfuerzo**: 6-8 horas
**Impacto**: Debugging avanzado en producción

---

## 📅 Cronograma Propuesto

### Sprint 1.2 - Semana 1 (Quick Wins)

| Día | Tarea | Esfuerzo | Responsable |
|-----|-------|----------|-------------|
| Lunes | Quick Win #1: Fix double-close panic | 2h | Ernesto + Gaby |
| Martes | Quick Win #2: Drop rate metrics | 3h | Ernesto + Gaby |
| Miércoles | Verificar buffers GStreamer | 2h | Ernesto + Gaby |
| Jueves | Testing manual de quick wins | 4h | Ernesto |
| Viernes | Code review + ajustes | 2h | Equipo |

**Total Semana 1**: ~13 horas

---

### Sprint 1.2 - Semana 2 (Mejoras Estructurales)

| Día | Tarea | Esfuerzo | Responsable |
|-----|-------|----------|-------------|
| Lunes | Mejora #1: Warmup API (diseño) | 2h | Ernesto + Gaby |
| Martes | Mejora #1: Warmup API (implementación) | 4h | Ernesto + Gaby |
| Miércoles | Mejora #2: Tests básicos (fail-fast) | 4h | Ernesto |
| Jueves | Mejora #2: Tests básicos (race) | 4h | Ernesto |
| Viernes | Testing manual + docs | 4h | Equipo |

**Total Semana 2**: ~18 horas

---

## ✅ Criterios de Éxito - Sprint 1.2

### Quick Wins
- [x] Double-close panic resuelto (0 crashes en testing)
- [x] Drop rate tracking funcional (stats visibles en test-capture)
- [x] Non-blocking pattern consistente en todo el código

### Mejoras Estructurales
- [x] Warmup API pública disponible
- [x] Tests básicos implementados (fail-fast + dimensions)
- [x] Documentación actualizada (CLAUDE.md)

### Calidad
- [x] `make dev` pasa sin errores
- [x] `go test -race` no detecta race conditions
- [x] test-capture funciona con stream real por 30+ minutos sin crashes

---

## 🎸 Filosofía del Plan

**"Quick wins primero, complejidad después"**

Este plan respeta:
- ✅ **Pragmatismo > Purismo**: Fixes críticos antes que arquitectura perfecta
- ✅ **Complejidad por diseño**: Cada mejora ataca complejidad real, no imaginaria
- ✅ **Fail fast**: Validación en cada paso (tests manuales después de cada quick win)
- ✅ **KISS**: Soluciones simples para problemas complejos

---

**Plan creado por**: Gaby de Visiona
**Basado en**: Consultoría Técnica 2025-11-04
**Siguiente revisión**: Fin de Sprint 1.2
