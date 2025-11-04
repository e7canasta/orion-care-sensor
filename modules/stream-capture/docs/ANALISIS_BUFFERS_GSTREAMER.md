# 🔍 Análisis de Buffers - GStreamer Pipeline

**Fecha**: 2025-11-04
**Módulo**: stream-capture
**Pregunta**: ¿Solo mantenemos 1 frame en buffer? ¿Es parte de la configuración de GStreamer?

---

## ✅ Respuesta Corta

**SÍ**, la configuración está **CORRECTA** y solo mantiene el último frame.

**Ubicación**: `internal/rtsp/pipeline.go:105-107`

```go
appsink.SetProperty("sync", false)       // No sync with clock (real-time)
appsink.SetProperty("max-buffers", 1)    // ✅ Keep only latest frame
appsink.SetProperty("drop", true)        // ✅ Drop old frames
```

---

## 📊 Análisis Detallado

### 1. Configuración de `appsink` (Pipeline Final)

**Elemento**: `appsink` (sink final que expone frames a Go)

| Propiedad | Valor | Efecto |
|-----------|-------|--------|
| `sync` | `false` | No sincroniza con clock → latencia mínima |
| `max-buffers` | `1` | ✅ **Solo 1 frame en cola** |
| `drop` | `true` | ✅ **Dropea frames viejos si nuevo llega antes de consumir** |

**Comportamiento**:
```
Frame 1 llega → Buffer [Frame 1]
Frame 2 llega antes de consumir Frame 1 → Buffer [Frame 2] (Frame 1 dropeado)
Frame 3 llega antes de consumir Frame 2 → Buffer [Frame 3] (Frame 2 dropeado)
```

**Conclusión**: ✅ **Correcto** - Mantiene solo el frame más reciente

---

### 2. Configuración de `rtspsrc` (Source)

**Elemento**: `rtspsrc` (fuente RTSP)

**Ubicación**: `internal/rtsp/pipeline.go:56-57`

```go
rtspsrc.SetProperty("protocols", 4)      // TCP only
rtspsrc.SetProperty("latency", 200)      // ⚠️ 200ms buffering
```

**Análisis**:

| Propiedad | Valor | Efecto |
|-----------|-------|--------|
| `protocols` | `4` (TCP) | Usa TCP → más buffering por confiabilidad |
| `latency` | `200ms` | ⚠️ **Buffer de 200ms en rtspsrc** |

**Comportamiento**:
- rtspsrc mantiene ~200ms de datos en buffer interno (jitter buffer)
- Esto es para compensar variaciones de red (jitter)
- **NO** es un buffer de frames completos, es buffer de paquetes RTSP

**Trade-off**:
- ✅ Resiliente a jitter de red
- ⚠️ Agrega ~200ms de latencia base

---

### 3. Otros Elementos del Pipeline

**Elementos intermedios** (NO tienen configuración de buffering explícita):
- `rtph264depay`: Depayloader RTP (usa buffers internos pequeños)
- `avdec_h264`: Decoder H.264 (puede mantener 1-2 frames para B-frames)
- `videoconvert`: Conversión RGB (sin buffering)
- `videoscale`: Scaling (sin buffering)
- `videorate`: Rate control (mantiene último frame para drop-only mode)
- `capsfilter`: Caps enforcement (sin buffering)

**Nota sobre `videorate`**:

**Ubicación**: `internal/rtsp/pipeline.go:86-87`

```go
videorate.SetProperty("drop-only", true)      // ✅ Only drop frames, never duplicate
videorate.SetProperty("skip-to-first", true)  // Skip to first frame on start
```

**Comportamiento**:
- `drop-only=true` → **Solo dropea frames**, nunca duplica
- Esto significa que si el stream viene a 30 FPS y configuramos 2 FPS:
  - videorate **dropea 28 de cada 30 frames**
  - **NO mantiene buffer**, solo pasa 1 de cada 15 frames

---

## 🎯 Análisis de Latencia Total

### Pipeline Flow con Buffering

```
RTSP Camera (30 FPS)
    ↓
┌───────────────────────────────────────────┐
│ rtspsrc (latency=200ms)                   │  ← ⚠️ ~6 frames @ 30 FPS
│ - Jitter buffer: 200ms de paquetes        │
└───────────────────────────────────────────┘
    ↓
┌───────────────────────────────────────────┐
│ rtph264depay                               │  ← ~1 frame
│ - RTP depayload                            │
└───────────────────────────────────────────┘
    ↓
┌───────────────────────────────────────────┐
│ avdec_h264                                 │  ← ~1-2 frames (H.264 decoding)
│ - H.264 decoder                            │
└───────────────────────────────────────────┘
    ↓
┌───────────────────────────────────────────┐
│ videoconvert → videoscale                  │  ← 0 frames (passthrough)
└───────────────────────────────────────────┘
    ↓
┌───────────────────────────────────────────┐
│ videorate (drop-only, target 2 FPS)        │  ← ~1 frame
│ - Drops 28/30 frames                       │
└───────────────────────────────────────────┘
    ↓
┌───────────────────────────────────────────┐
│ capsfilter                                 │  ← 0 frames (passthrough)
└───────────────────────────────────────────┘
    ↓
┌───────────────────────────────────────────┐
│ appsink (max-buffers=1, drop=true)        │  ← ✅ 1 frame MÁXIMO
│ - Solo mantiene último frame               │
└───────────────────────────────────────────┘
    ↓
Go Application (frameChan buffer=10)         ← ✅ 10 frames MÁXIMO
```

### Cálculo de Latencia

**Latencia Total Estimada**:
```
rtspsrc buffer:       200 ms  (configurado)
rtph264depay:          ~5 ms  (processing)
avdec_h264:          ~15 ms  (H.264 decode)
videoconvert/scale:   ~5 ms  (RGB conversion + scale)
videorate:            ~2 ms  (drop logic)
appsink:              ~1 ms  (buffer handoff)
Go channel:           ~1 ms  (channel send)
────────────────────────────
Total:              ~229 ms
```

**Nota**: Esto es **latencia base** (processing pipeline), no incluye latencia de red (RTSP).

---

## 🎯 Recomendaciones

### ✅ Configuración Actual es Correcta

La configuración actual respeta la filosofía **"drop frames, never queue"**:

1. ✅ `appsink.max-buffers=1` → Solo 1 frame en buffer final
2. ✅ `appsink.drop=true` → Dropea frames viejos
3. ✅ `videorate.drop-only=true` → Solo dropea, nunca duplica
4. ✅ Go channel buffer=10 → Suficiente para consumer processing

---

### ⚠️ Oportunidad de Optimización: Reducir Latencia de rtspsrc

**Problema**: `rtspsrc.latency=200ms` agrega latencia base.

**Propuesta**: Hacer `latency` configurable por preset:

```go
type PipelinePreset int

const (
    PresetLowLatency  PipelinePreset = iota  // latency=50ms
    PresetBalanced                          // latency=200ms (actual)
    PresetHighQuality                       // latency=500ms
)

// En RTSPConfig
type RTSPConfig struct {
    // ... campos existentes ...
    Preset PipelinePreset  // NEW: Configurable preset
}

// En pipeline.go
func CreatePipeline(cfg PipelineConfig) (*PipelineElements, error) {
    // ...

    var latency int
    switch cfg.Preset {
    case PresetLowLatency:
        latency = 50   // ✅ 50ms buffer → latencia ~79ms total
    case PresetBalanced:
        latency = 200  // Default actual
    case PresetHighQuality:
        latency = 500  // Para redes inestables
    default:
        latency = 200
    }

    rtspsrc.SetProperty("latency", latency)
    // ...
}
```

**Trade-offs**:

| Preset | Latency Total | Jitter Tolerance | Use Case |
|--------|---------------|------------------|----------|
| LowLatency (50ms) | ~79ms | Bajo | LAN estable, real-time crítico |
| Balanced (200ms) | ~229ms | Medio | Default, buena relación |
| HighQuality (500ms) | ~529ms | Alto | WAN inestable, calidad > latencia |

---

### 📊 Verificación en Código Go

**Channel Buffer en rtsp.go:91**:

```go
s := &RTSPStream{
    // ...
    frames: make(chan Frame, 10),  // ✅ Buffer de 10 frames
}
```

**Análisis**:
- Buffer de 10 frames es **razonable** para consumer processing
- Si consumer es más lento que 2 FPS, empezará a dropear (comportamiento deseado)
- **NO** es un buffer de "queue", es un buffer de "smoothing"

**Comportamiento bajo carga**:

```
Productor (GStreamer): 2 FPS
Consumer (Application): Procesamiento varía (0.1s - 1s por frame)

Escenario A: Consumer rápido (0.1s/frame)
    Channel buffer: [Frame1] → Consumer procesa → [Frame2] → ...
    ✅ Sin drops

Escenario B: Consumer lento (0.6s/frame)
    T=0.0s: [Frame1]
    T=0.5s: [Frame1, Frame2]
    T=0.6s: [Frame2] (Frame1 procesado)
    T=1.0s: [Frame2, Frame3]
    T=1.2s: [Frame3] (Frame2 procesado)
    ✅ Sin drops (buffer de 10 frames absorbe variaciones)

Escenario C: Consumer muy lento (>5s/frame)
    Channel se llena → rtsp.go:193 bloquea
    ⚠️ PROBLEMA: Debería dropear, no bloquear
    📝 FIX: Ver Quick Win #2 en PLAN_ACCION_QUICK_WINS.md
```

---

## ✅ Conclusión

### Estado Actual

| Componente | Configuración | Estado | Notas |
|------------|---------------|--------|-------|
| **appsink** | `max-buffers=1, drop=true` | ✅ CORRECTO | Solo 1 frame en buffer final |
| **videorate** | `drop-only=true` | ✅ CORRECTO | No duplica frames |
| **rtspsrc** | `latency=200ms` | ⚠️ CONFIGURABLE | Podría ser preset |
| **Go channel** | `buffer=10` | ✅ CORRECTO | Smoothing buffer |
| **Go send pattern** | Blocking (sin default) | ❌ INCORRECTO | Debería dropear |

---

### Respuesta a tu Pregunta

**"¿En GStreamer pipeline deberíamos solo mantener el buffer 1 frame, el último no más?"**

**Respuesta**: **SÍ, y ya está configurado así**.

**Evidencia**:
- `appsink.max-buffers=1` → ✅ Solo 1 frame en appsink
- `appsink.drop=true` → ✅ Dropea frames viejos
- `videorate.drop-only=true` → ✅ No duplica frames

**Único ajuste necesario**: Fix en `rtsp.go:193` para agregar `default` case y dropear cuando Go channel está lleno (ver Quick Win #2).

---

## 🎸 Filosofía de Diseño Validada

**"Drop frames, never queue"** → ✅ **Implementado correctamente en GStreamer**

**Único gap**: Go layer (rtsp.go:193) no respeta completamente este principio.

---

**Análisis realizado por**: Gaby de Visiona
**Metodología**: Pipeline archaeology + GStreamer documentation
**Conclusión**: "Tienes razón, y el código ya lo hace bien (casi)" 🎸
