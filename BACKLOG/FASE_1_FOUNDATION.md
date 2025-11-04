# 🎯 Fase 1: Foundation (v1.0 → v1.5)

**Milestone**: [v1.5 - Foundation](https://github.com/e7canasta/orion-care-sensor/milestone/1)  
**Due Date**: 2025-01-31  
**Objetivo**: Orion funcionando con Care Scene, deployable en POC

---

## 📋 Sprints Overview

| Sprint | Issue | Status | Estimación | Bounded Context |
|---|---|---|---|---|
| **1.1** | [#1](https://github.com/e7canasta/orion-care-sensor/issues/1) | ⬜ Todo | 2 semanas | Stream Capture |
| **1.2** | [#2](https://github.com/e7canasta/orion-care-sensor/issues/2) | ⬜ Todo | 2 semanas | Worker Lifecycle |
| **2** | [#3](https://github.com/e7canasta/orion-care-sensor/issues/3) | ⬜ Todo | 2 semanas | Control Plane |
| **3** | [#4](https://github.com/e7canasta/orion-care-sensor/issues/4) | ⬜ Todo | 2 semanas | Integration |

**Total estimado**: 8 semanas

---

## 🔨 Sprint 1.1: Stream Capture Module

**Issue**: [#1 - Sprint 1.1: Stream Capture Module](https://github.com/e7canasta/orion-care-sensor/issues/1)

### Bounded Context

**Responsabilidad:**
- ✅ Capturar frames RTSP
- ✅ Reconexión automática
- ✅ FPS adaptativo

**Anti-responsabilidad:**
- ❌ NO procesa frames
- ❌ NO decide qué capturar

### Entregables

```
internal/stream/
├── capture.go          # RTSP capture, reconnection
├── framebus.go         # Non-blocking fan-out (existente)
└── warm_up.go          # FPS measurement (existente)
```

### Acceptance Criteria

- [ ] RTSP stream se captura correctamente
- [ ] Reconexión automática en caso de fallo
- [ ] FPS se mide durante warm-up (5 segundos)
- [ ] Frames se distribuyen a FrameBus sin bloqueo
- [ ] Unit tests: Mock RTSP, validar FPS
- [ ] Integration tests: RTSP real, reconexión

### Referencias

- [C4 Model - Stream Capture Component](../docs/DESIGN/C4_MODEL.md#c3---component-diagram)
- [Plan Evolutivo - Sprint 1.1](../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#11-stream-capture-module)

### Lecciones Aprendidas

_Se actualizará al completar el sprint_

---

## 🔨 Sprint 1.2: Worker Lifecycle Module

**Issue**: [#2 - Sprint 1.2: Worker Lifecycle Module](https://github.com/e7canasta/orion-care-sensor/issues/2)

### Bounded Context

**Responsabilidad:**
- ✅ Spawn workers Python
- ✅ Monitor health (watchdog adaptativo)
- ✅ Restart on failure (1 intento)
- ✅ Load worker manifests (YAML)

**Anti-responsabilidad:**
- ❌ NO conoce qué hace el worker
- ❌ NO interpreta resultados

### Entregables

```
internal/worker/
├── types.go            # Worker interfaces (existente)
├── lifecycle.go        # NUEVO: Spawn, monitor, restart
├── catalog.go          # NUEVO: Worker registry
└── person_detector_python.go  # Existente

config/workers/
└── person_detector.yaml  # NUEVO: Worker manifest
```

### Worker Catalog Schema

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
        x: float
        y: float
        w: float
        h: float
        confidence: float
```

### Acceptance Criteria

- [ ] Worker manifests se cargan desde YAML
- [ ] Workers Python se spawnean via `exec.Command`
- [ ] Health monitor detecta workers muertos
- [ ] Restart automático (1 intento) en caso de fallo
- [ ] Worker registry mantiene catálogo de workers
- [ ] Unit tests: Mock subprocess, validar lifecycle
- [ ] Integration tests: Spawn real Python worker

### Referencias

- [C4 Model - Worker Lifecycle Component](../docs/DESIGN/C4_MODEL.md#worker-lifecycle-manager)
- [Plan Evolutivo - Sprint 1.2](../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#12-worker-lifecycle-module)

[[MANIFESTO_DISENO - Blues Style]]


### Lecciones Aprendidas

_Se actualizará al completar el sprint_

---

## 🔨 Sprint 2: MQTT Control Plane Refactor

**Issue**: [#3 - Sprint 2: MQTT Control Plane Refactor](https://github.com/e7canasta/orion-care-sensor/issues/3)

### Bounded Context

**Responsabilidad:**
- ✅ Recibir comandos MQTT
- ✅ Validar schemas
- ✅ Aplicar configuraciones (pause/resume/rate/workers)
- ✅ Responder con status

**Anti-responsabilidad:**
- ❌ NO ejecuta workers
- ❌ NO procesa inferencias

### Entregables

```
internal/control/
├── handler.go          # Existente: MQTT subscriber
├── validator.go        # NUEVO: Schema validation
└── commands.go         # NUEVO: Command types
```

### Command Schema

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

### Acceptance Criteria

- [ ] Comandos MQTT se reciben en `care/control/{instance_id}`
- [ ] Schema validation antes de aplicar config
- [ ] Hot-reload de workers sin reiniciar stream
- [ ] Hot-reload de FPS reinicia stream (~2s interrupción)
- [ ] Status se publica en respuesta a `get_status`
- [ ] Unit tests: Mock MQTT, validar comandos
- [ ] Integration tests: MQTT real, hot-reload workers

### Referencias

- [C4 Model - Control Handler Component](../docs/DESIGN/C4_MODEL.md#control-handler)
- [Plan Evolutivo - Sprint 2](../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#sprint-2-mqtt-control-plane-2-semanas)

### Lecciones Aprendidas

_Se actualizará al completar el sprint_

---

## 🔨 Sprint 3: Integration con Care Scene

**Issue**: [#4 - Sprint 3: Integration con Care Scene](https://github.com/e7canasta/orion-care-sensor/issues/4)

### Bounded Context

**Responsabilidad:**
- ✅ Exponer Worker Catalog vía REST (opcional)
- ✅ Validar integración MQTT con Room Orchestrator
- ✅ Publicar inferencias en formato Care Scene
- ✅ Recibir comandos de Room Orchestrator

### Entregables

```
cmd/catalog-server/     # NUEVO (opcional v1.5)
├── main.go             # REST API para Worker Catalog

tests/integration/      # NUEVO
└── care_scene_test.go  # Test end-to-end
```

### Worker Catalog API (Opcional)

```
GET /workers              → Lista de workers disponibles
GET /workers/{type}       → Manifest específico
```

### Acceptance Criteria

- [ ] Worker Catalog API expone manifests vía REST (opcional)
- [ ] Integración MQTT con Room Orchestrator validada
- [ ] Inferencias publicadas en `care/inferences/{instance_id}`
- [ ] Formato JSON compatible con Scene Experts
- [ ] Test end-to-end: Simulación caso José
- [ ] Integration tests: Room Orchestrator → Orion → Worker → MQTT

### Caso José (End-to-End Test)

**Scenario**: Detección de persona en borde de cama
1. Room Orchestrator envía comando `update_config` con worker `person_detector`
2. Orion spawns worker y empieza captura
3. Frame muestra persona en borde de cama
4. Worker detecta bounding box con confidence > 0.7
5. Orion publica inferencia en `care/inferences/orion-hab-302`
6. Scene Expert "Edge Detection" recibe inferencia
7. Edge Expert emite evento `edge.detected` en `care/events/hab-302`

### Referencias

- [C4 Model - System Context](../docs/DESIGN/C4_MODEL.md#c1---system-context-diagram)
- [Plan Evolutivo - Sprint 3](../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#sprint-3-integration-con-care-scene-2-semanas)

### Lecciones Aprendidas

_Se actualizará al completar el sprint_

---

## 📊 Estado Actual

**Completado**: 0/4 sprints (0%)  
**En Progreso**: Sprint 1.1  
**Próximo**: Sprint 1.2

### Bloqueadores

_Ninguno por ahora_

### Decisiones Pendientes

- [ ] ¿Worker Catalog API REST es necesario en v1.5 o posponer a v2.0?
- [ ] ¿Schema validation con librería externa o implementación custom?

---

## 📝 Session Checklist (Para Cada Sprint)

### Antes de codear:
- [ ] Leo `CLAUDE.md` + `Big Picture.md`
- [ ] Identifico bounded context del sprint
- [ ] Propongo 2-3 opciones de diseño
- [ ] Evalúo trade-offs con Ernesto
- [ ] Elijo "quick win"

### Durante desarrollo:
- [ ] Commits atómicos
- [ ] Compilo después de cada paso
- [ ] Tests unitarios + integration
- [ ] Preservo API pública

### Después de codear:
- [ ] Pair review con Ernesto
- [ ] Actualizo `CLAUDE.md` si arquitectura cambió
- [ ] Documento lecciones en este backlog
- [ ] Identifico próximos pasos

---

**Última actualización**: 2025-11-03  
**Autor**: Ernesto + Gaby (AI Companion)
