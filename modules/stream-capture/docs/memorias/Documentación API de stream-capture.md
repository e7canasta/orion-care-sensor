✅ Documentación API de stream-capture

# ✅ Documentación API de stream-capture - Completado

## Estado Actual

### ✅ Lo que ahora tienes (Go Best Practices):

1. **`doc.go`** (NUEVO) - Package overview con:
   - Quick start completo
   - Ejemplos de código funcionales
   - Guía de instalación de dependencias
   - Explicación de features principales
   - Performance characteristics
   - Limitaciones conocidas
   - Roadmap
   - Enlaces a arquitectura

2. **Godoc comments** (existentes) - En:
   - `provider.go` - StreamProvider interface
   - `types.go` - Frame, StreamStats, RTSPConfig, etc.
   - `rtsp.go` - NewRTSPStream

3. **Examples** (existentes) - Código ejecutable:
   - `examples/simple/` - Captura básica
   - `examples/hot-reload/` - Cambio dinámico de FPS

4. **Testing tool** - CLI para probar la librería:
   - `cmd/test-capture/` con README completo

## Cómo lo usa un cliente externo

### 1. Instalación
```bash
go get github.com/e7canasta/orion-care-sensor/modules/stream-capture
```

### 2. Ver documentación general
```bash
go doc github.com/e7canasta/orion-care-sensor/modules/stream-capture
```
**Output**: Package overview completo de doc.go

### 3. Ver función específica
```bash
go doc github.com/e7canasta/orion-care-sensor/modules/stream-capture.NewRTSPStream
```
**Output**: Constructor con ejemplos y validaciones

### 4. Ver interface
```bash
go doc github.com/e7canasta/orion-care-sensor/modules/stream-capture.StreamProvider
```
**Output**: Contract completo con ejemplos de uso

### 5. Ver tipo
```bash
go doc github.com/e7canasta/orion-care-sensor/modules/stream-capture.RTSPConfig
```
**Output**: Estructura de configuración con fields documentados

### 6. Explorar ejemplos
```bash
cd examples/simple
cat main.go  # Código listo para ejecutar
```

### 7. En pkg.go.dev (cuando se publique)
URL: `https://pkg.go.dev/github.com/e7canasta/orion-care-sensor/modules/stream-capture`

Mostrará automáticamente:
- Package overview (doc.go)
- Index de tipos/funciones
- Godoc de cada símbolo exportado
- (Opcional) Example functions si se agregan en *_test.go

## Comparación con otras librerías Go

| Librería | README | doc.go | Godoc | Examples | Test tool | Sphinx/Doxygen |
|----------|--------|--------|-------|----------|-----------|----------------|
| **stream-capture** | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ (no se usa) |
| go-gst | ✅ | ❌ | ⚠️ | ⚠️ | ❌ | ❌ |
| pion/webrtc | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| ffmpeg-go | ✅ | ❌ | ⚠️ | ✅ | ❌ | ❌ |
| stdlib (io, net/http) | N/A | ✅ | ✅ | ✅ | N/A | ❌ |

**Conclusión**: stream-capture ahora sigue (y supera) el estándar del ecosistema Go.

## Próximos pasos opcionales (no críticos)

### A. Example Functions en *_test.go
Agregar funciones `ExampleXxx` que aparecen en godoc:

```go
// En stream-capture_test.go
func ExampleNewRTSPStream() {
    cfg := RTSPConfig{
        URL:        "rtsp://camera/stream",
        Resolution: Res720p,
        TargetFPS:  2.0,
    }
    stream, _ := NewRTSPStream(cfg)
    defer stream.Stop()
    
    frameChan, _ := stream.Start(context.Background())
    frame := <-frameChan
    fmt.Printf("Frame: %dx%d\n", frame.Width, frame.Height)
    // Output: Frame: 1280x720
}
```

### B. README.md en módulo raíz
Si stream-capture se usará como librería standalone (no solo dentro de Orion),
agregar un README.md con:
- Badge de godoc
- Instalación
- Quick start
- Link a pkg.go.dev

### C. CONTRIBUTING.md
Si aceptarás contribuciones externas.

## Arquitectura de documentación actual

```
modules/stream-capture/
├── doc.go                    # ✅ Package overview (API reference)
├── provider.go               # ✅ Godoc en StreamProvider interface
├── types.go                  # ✅ Godoc en Frame, StreamStats, RTSPConfig
├── rtsp.go                   # ✅ Godoc en NewRTSPStream
├── examples/
│   ├── simple/               # ✅ Ejemplo ejecutable básico
│   └── hot-reload/           # ✅ Ejemplo ejecutable hot-reload
├── cmd/test-capture/         # ✅ Testing tool
│   └── README.md             # ✅ Documentación del tool
└── docs/
    ├── ARCHITECTURE.md       # ✅ Para devs avanzados (4+1 views)
    ├── C4_MODEL.md           # ✅ Para onboarding (C1-C4 diagrams)
    └── adr/                  # ✅ Decisiones técnicas (ADRs)
```

## Comandos útiles

```bash
# Ver toda la documentación del package
go doc -all github.com/e7canasta/orion-care-sensor/modules/stream-capture

# Ver solo exports (sin details)
go doc github.com/e7canasta/orion-care-sensor/modules/stream-capture

# Ver función específica con ejemplos
go doc github.com/e7canasta/orion-care-sensor/modules/stream-capture.NewRTSPStream

# Generar HTML local (simula pkg.go.dev)
godoc -http=:6060
# Luego abrir: http://localhost:6060/pkg/github.com/e7canasta/orion-care-sensor/modules/stream-capture/
```

