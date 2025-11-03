# 🗺️ Plan Evolutivo para Orion 2.0

**Versión:** v1.0
**Fecha:** 2025-11-03
**Autores:** Ernesto (Visiona) + Gaby (AI Companion)

---

## 📝 Memoria de Valor de la Sesión de Diseño

### 🎯 Lo que Aprendimos como Equipo

#### **1. Context Switching: De "Orion solo" a "Orion en Care Scene"**

**Inicio:**
- Análisis de Orion como "competidor de DeepStream"
- Features aisladas (hot-reload, frame-by-frame)

**Después de leer Care Scene docs:**
- Orion es **solo el sensor objetivo** en un sistema mayor
- Care Scene = Orion + Scene Experts + Room Orchestrator + Temporal
- Arquitectura event-driven edge-first con cloud supervisor

**Valor:** No diseñar en el vacío. Orion 2.0 debe diseñarse **para** Care Scene.

---

#### **2. El Manifiesto Blues: "Pragmatismo > Purismo"**

**Quote clave:**
> "Las buenas prácticas son vocabulario de diseño - las practicas para tenerlas disponibles cuando improvises, no porque la partitura lo diga."

**Aplicación a Orion 2.0:**
- ✅ DDD para bounded contexts claros
- ✅ SOLID donde importa
- ✅ Pragmatismo para utilities
- ❌ NO Hexagonal puro "porque sí"
- ❌ NO DI everywhere "porque es best practice"

---

#### **3. Big Picture Primero, Siempre**

**Lo que hicimos bien:**
1. Leímos `Big Picture.md` → Orion 1.0
2. Leímos Care Scene docs → Sistema completo
3. Leímos Manifiesto → Filosofía de diseño

**Valor:** 30 minutos leyendo docs ahorran 3 semanas de código mal diseñado.

---

#### **4. Bounded Contexts Claros desde el Inicio**

| Bounded Context | Responsabilidad | Anti-responsabilidad |
|---|---|---|
| **Stream Capture** | Capturar frames, FPS adaptativo | ❌ NO procesa contenido |
| **Worker Lifecycle** | Spawn/monitor workers, IPC | ❌ NO conoce qué hace el worker |
| **Event Emission** | Publicar inferencias MQTT | ❌ NO interpreta eventos |
| **Worker Catalog** | Registry, resource profiles | ❌ NO ejecuta workers |

---

#### **5. La Estrategia de Negocio Define la Arquitectura**

**Insights del modelo consultivo B2B:**
- **POC €500/mes** → Orion deployable en 1 día
- **Discovery automático** → Telemetría rica
- **Upsell incremental** → Hot-reload de workers
- **i7 iGPU ($250)** → OpenVINO support

---

#### **6. Multi-Stream es el Futuro, Single-Stream es el Presente**

**Roadmap:**
- ✅ **v1.0**: Single-stream, multi-worker, hot-reload
- ✅ **v2.0**: Multi-stream (8 hab/hub), resource management
- ✅ **v3.0**: Cell orchestration, motion pooling

**Valor:** Diseño evolutivo. No anticipar v3.0 hoy.

---

## 🎸 "El Blues que Tocamos Hoy"

### **Escalas que conocíamos:**
- DDD, SOLID, Clean Architecture
- Event-driven patterns, MQTT
- Hot-reload, config management
- GPU acceleration, ONNX

### **Improvisación con contexto:**
- Aplicar DDD a Orion 2.0
- Entender Care Scene completo
- Diseñar para modelo B2B
- Priorizar multi-stream futuro

### **Pragmatismo:**
- ✅ Single-stream primero
- ✅ Worker Catalog simple (YAML)
- ✅ MQTT coreography
- ✅ OpenVINO iGPU

---

## 📅 Roadmap de 3 Fases

## Filosofía del Plan

> **"De menos a más. Llevar de a poco pieza a pieza. Diseño paso a paso."**
> — Ernesto

### **Principios:**
1. **Incremental:** Cada paso deployable y testeable
2. **Evolutivo:** Diseño emerge de feedback
3. **Domain-Driven:** Bounded contexts claros
4. **Blues Style:** Conocer escalas, improvisar con contexto

---

## 🎯 Fase 1: Foundation (v1.0 → v1.5)

**Objetivo:** Orion funcionando con Care Scene, deployable en POC

### **Sprint 1: Bounded Contexts Básicos (2 semanas)**

#### **1.1: Stream Capture Module**
```
internal/stream/
├── capture.go          # RTSP capture, reconnection
├── framebus.go         # Non-blocking fan-out (existente)
└── warm_up.go          # FPS measurement (existente)
```

**Responsabilidad:**
- ✅ Capturar frames RTSP
- ✅ Reconexión automática
- ✅ FPS adaptativo

**Anti-responsabilidad:**
- ❌ NO procesa frames
- ❌ NO decide qué capturar

**Tests:**
- Unit: Mock RTSP, validar FPS
- Integration: RTSP real, reconexión

---

#### **1.2: Worker Lifecycle Module**
```
internal/worker/
├── types.go            # Worker interfaces (existente)
├── lifecycle.go        # NUEVO: Spawn, monitor, restart
├── catalog.go          # NUEVO: Worker registry
└── person_detector_python.go  # Existente
```

**Catalog Schema (YAML):**
```yaml
# config/workers/person_detector.yaml
worker_type: person_detector
version: v2.1.0
runtime: python
entrypoint: models/person_detector.py

specification_schema:
  type: object
  properties:
    confidence_threshold:
      type: float
      default: 0.5
    roi:
      type: object
      optional: true

resource_profile:
  avg_inference_ms: 35
  memory_mb: 512
  cpu_cores: 1.0

outputs:
  - name: person_bbox
    schema:
      type: object
      properties:
        x: int
        y: int
        w: int
        h: int
        confidence: float
```

**Implementación:**
```go
// internal/worker/catalog.go
type WorkerCatalog struct {
    workers map[string]*WorkerManifest
}

func (c *WorkerCatalog) Load(dir string) error {
    // Leer YAML manifests
}

func (c *WorkerCatalog) Get(workerType string) (*WorkerManifest, error) {
    // Retornar manifest
}

// internal/worker/lifecycle.go
type WorkerManager struct {
    catalog *WorkerCatalog
    active  map[string]*WorkerInstance
}

func (m *WorkerManager) Spawn(workerType string, spec map[string]interface{}) error {
    manifest := m.catalog.Get(workerType)
    // exec.Command según manifest
    // Configurar stdin/stdout MsgPack
    // Monitor health
}
```

---

#### **1.3: Event Emission Module**
```
internal/emitter/
├── mqtt.go             # Existente (refactorizar)
└── telemetry.go        # NUEVO: Rich telemetry
```

**Output Schema (JSON):**
```json
{
  "stream_id": "hab_302",
  "worker_type": "person_detector",
  "frame_seq": 1234,
  "timestamp_capture": "2025-11-03T14:30:45.123Z",
  "timestamp_inference": "2025-11-03T14:30:45.158Z",
  "data": {
    "person_count": 1,
    "bboxes": [{"x": 320, "y": 180, "w": 100, "h": 200, "conf": 0.92}]
  },
  "metadata": {
    "orion_instance": "orion-lq-302",
    "model_version": "yolo11n-v2.1",
    "processing_time_ms": 35
  }
}
```

---

### **Sprint 2: Control Plane (2 semanas)**

#### **2.1: MQTT Control Topics**
```
internal/control/
├── handler.go          # Existente (refactorizar)
└── commands.go         # NUEVO: Command types
```

**Command Schema:**
```json
// Topic: care/control/orion-{instance_id}
{
  "command": "update_config",
  "config": {
    "stream_id": "hab_302",
    "fps": 10,
    "workers": ["person_detector", "pose_estimator"],
    "spec_per_worker": {
      "person_detector": {
        "confidence_threshold": 0.7
      },
      "pose_estimator": {
        "roi": {"x": 100, "y": 50, "w": 400, "h": 300}
      }
    }
  }
}
```

---

### **Sprint 3: Integration con Care Scene (2 semanas)**

#### **3.1: Artefactos para Room Orchestrator**

**Worker Catalog API (REST - opcional v1.5):**
```go
// cmd/catalog-server/main.go
func main() {
    catalog := worker.LoadCatalog("config/workers")

    http.HandleFunc("/workers", func(w http.ResponseWriter, r *http.Request) {
        json.NewEncoder(w).Encode(catalog.List())
    })

    http.HandleFunc("/workers/{type}", func(w http.ResponseWriter, r *http.Request) {
        manifest := catalog.Get(mux.Vars(r)["type"])
        json.NewEncoder(w).Encode(manifest)
    })

    log.Fatal(http.ListenAndServe(":8080", nil))
}
```

**Tests:**
- Integration: Room Orchestrator → Orion → Worker → MQTT
- End-to-end: Caso José simulado

---

## 🎯 Fase 2: Scale (v1.5 → v2.0)

**Objetivo:** 1 hub i7 → 4-8 habitaciones

### **Sprint 4: Multi-Stream Architecture (3 semanas)**

#### **4.1: Stream Multiplexing**
```
internal/core/
├── orion.go            # Refactorizar para multi-stream
└── stream_manager.go   # NUEVO: Gestiona N streams
```

**Architecture:**
```
Orion Instance (1 proceso Go):
  ├─ Stream 1 (hab_302) → Worker Pool 1
  ├─ Stream 2 (hab_303) → Worker Pool 2
  ├─ Stream 3 (hab_304) → Worker Pool 3
  └─ Stream 4 (hab_305) → Worker Pool 4
```

**Config:**
```yaml
# config/orion_multi.yaml
streams:
  - stream_id: hab_302
    rtsp_url: rtsp://cam-302/stream
    fps: 10
    workers:
      - person_detector
      - pose_estimator

  - stream_id: hab_303
    rtsp_url: rtsp://cam-303/stream
    fps: 5
    workers:
      - person_detector
```

---

#### **4.2: Resource Management**
```
internal/resource/
├── profiler.go         # NUEVO: Resource tracking
└── allocator.go        # NUEVO: Resource allocation
```

---

## 🎯 Fase 3: Intelligence (v2.0 → v3.0)

**Objetivo:** Focus dinámico, motion pooling

### **Sprint 5: Cell Orchestrator (4 semanas)**

#### **5.1: Motion Detection Pooling**
```
internal/cell/
├── orchestrator.go     # NUEVO: Cell coordination
└── motion_pool.go      # NUEVO: Motion detection
```

**Architecture:**
```
Cell Orchestrator:
  ├─ Monitorea 8 streams
  ├─ Detecta actividad (motion)
  ├─ Asigna recursos:
     - Hab 302 (actividad) → Full Orion
     - Hab 303-308 (sleep) → Motion pool
  └─ Balancea carga real-time
```

---

## 📊 Checklist de Sesión para Cada Sprint

### **Antes de codear:**
- [ ] Leo `CLAUDE.md` + `Big Picture.md`
- [ ] Identifico bounded context del sprint
- [ ] Propongo 2-3 opciones de diseño
- [ ] Evalúo trade-offs con Ernesto
- [ ] Elijo "quick win"

### **Durante desarrollo:**
- [ ] Commits atómicos
- [ ] Compilo después de cada paso
- [ ] Tests unitarios + integration
- [ ] Preservo API pública

### **Después de codear:**
- [ ] Pair review con Ernesto
- [ ] Actualizo `CLAUDE.md` si arquitectura cambió
- [ ] Documento lecciones
- [ ] Identifico próximos pasos

---

## 🎸 Epílogo

> **"De menos a más. Diseño paso a paso. Pragmatismo > Purismo."**

### **El Plan en Una Frase:**
- **Fase 1:** Bounded contexts básicos, single-stream sólido
- **Fase 2:** Multi-stream, resource management
- **Fase 3:** Cell orchestration, motion pooling

### **Pregunta Guía:**
> *"¿Este paso habilita el siguiente o predice los próximos 5?"*

**Habilitar ✅ | Predecir ❌**

---

**Listo para Sprint 1 en la próxima sesión** 🚀

---

**Versión:** v1.0
**Fecha:** 2025-11-03
**Autores:** Ernesto (Visiona) + Gaby (AI Companion)

---

**📚 Documentación Relacionada:**
- [C4_MODEL.md](C4_MODEL.md) - Arquitectura C4 completa
- [Big Picture.md](Big%20Picture.md) - Arquitectura Orion 1.0
- [MANIFESTO_DISENO - Blues Style.md](../../MANIFESTO_DISENO%20-%20Blues%20Style.md)
