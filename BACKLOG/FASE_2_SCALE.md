# 🚀 Fase 2: Scale (v1.5 → v2.0)

**Milestone**: [v2.0 - Scale](https://github.com/e7canasta/orion-care-sensor/milestone/2)  
**Due Date**: 2025-03-31  
**Objetivo**: 1 hub i7 → 4-8 habitaciones simultáneas

---

## 📋 Sprints Overview

| Sprint | Issue | Status | Estimación | Bounded Context |
|---|---|---|---|---|
| **4.1** | [#5](https://github.com/e7canasta/orion-care-sensor/issues/5) | 📅 Planned | 2 semanas | Stream Multiplexing |
| **4.2** | TBD | 📅 Planned | 1 semana | Resource Management |

**Total estimado**: 3 semanas

---

## 🔨 Sprint 4.1: Stream Multiplexing

**Issue**: [#5 - Fase 2: Multi-Stream Architecture](https://github.com/e7canasta/orion-care-sensor/issues/5)

### Bounded Context

**Responsabilidad:**
- ✅ Gestionar N streams RTSP en paralelo
- ✅ Worker pools independientes por stream
- ✅ Balanceo de carga CPU/memoria

**Anti-responsabilidad:**
- ❌ NO es cell orchestration (eso es Fase 3)
- ❌ NO es motion pooling (eso es Fase 3)

### Entregables

```
internal/core/
├── orion.go            # Refactorizar para multi-stream
└── stream_manager.go   # NUEVO: Gestiona N streams
```

### Architecture

```
Orion Instance (1 proceso Go):
  ├─ Stream 1 (hab_302) → Worker Pool 1 (person_detector, pose_estimator)
  ├─ Stream 2 (hab_303) → Worker Pool 2 (person_detector)
  ├─ Stream 3 (hab_304) → Worker Pool 3 (person_detector, flow_analyzer)
  └─ Stream 4 (hab_305) → Worker Pool 4 (person_detector)
```

### Config Schema

```yaml
# config/orion_multi.yaml
instance_id: orion-cell-3A
deployment_mode: multi_stream

streams:
  - stream_id: hab_302
    rtsp_url: rtsp://cam-302/stream
    fps: 10
    workers:
      - type: person_detector
        spec:
          confidence_threshold: 0.7
      - type: pose_estimator
        spec:
          roi: {x: 100, y: 50, w: 400, h: 300}

  - stream_id: hab_303
    rtsp_url: rtsp://cam-303/stream
    fps: 5
    workers:
      - type: person_detector
        spec:
          confidence_threshold: 0.6

  - stream_id: hab_304
    rtsp_url: rtsp://cam-304/stream
    fps: 10
    workers:
      - type: person_detector
      - type: flow_analyzer

  - stream_id: hab_305
    rtsp_url: rtsp://cam-305/stream
    fps: 5
    workers:
      - type: person_detector
```

### Acceptance Criteria

- [ ] Orion maneja 4-8 streams RTSP simultáneos
- [ ] Cada stream tiene worker pool independiente
- [ ] Fallo en 1 stream NO afecta otros streams
- [ ] MQTT topics por stream: `care/inferences/{stream_id}`
- [ ] Control commands por stream: `care/control/{instance_id}/{stream_id}`
- [ ] Resource monitoring básico (CPU/memory)
- [ ] Unit tests: Mock multi-stream
- [ ] Integration tests: 4 streams reales

### Performance Targets

| Hardware | Streams | Workers/Stream | Total Workers | FPS/Stream |
|---|---|---|---|---|
| i7-10700 8C/16T | 4 | 2 | 8 | 10 |
| i7-10700 8C/16T | 8 | 1 | 8 | 5 |
| i9-12900K 16C/24T | 8 | 2 | 16 | 10 |

### Referencias

- [C4 Model - Multi-Stream Container](../docs/DESIGN/C4_MODEL.md#c2---container-diagram)
- [Plan Evolutivo - Sprint 4.1](../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#41-stream-multiplexing)

### Lecciones Aprendidas

_Se actualizará al completar el sprint_

---

## 🔨 Sprint 4.2: Resource Management

**Issue**: TBD (se creará después de Sprint 4.1)

### Bounded Context

**Responsabilidad:**
- ✅ Resource profiling (CPU, memoria, inference time)
- ✅ Resource allocation (decisión de cuántos workers por stream)
- ✅ Telemetría rica para Room Orchestrator

**Anti-responsabilidad:**
- ❌ NO es cell orchestration
- ❌ NO toma decisiones de apagar streams

### Entregables

```
internal/resource/      # NUEVO
├── profiler.go         # Resource tracking
└── allocator.go        # Resource allocation

internal/telemetry/     # NUEVO
└── metrics.go          # Prometheus-style metrics
```

### Resource Profile Schema

```yaml
# Añadir a worker manifests
resource_profile:
  avg_inference_ms: 35
  memory_mb: 512
  cpu_cores: 1.0
  
  # NUEVO en v2.0
  gpu_required: false
  gpu_memory_mb: 0
  min_batch_size: 1
  max_batch_size: 1
```

### Telemetry Output

```json
// Topic: care/telemetry/{instance_id}
{
  "instance_id": "orion-cell-3A",
  "timestamp": "2025-03-15T10:30:00Z",
  "system": {
    "cpu_percent": 65.3,
    "memory_mb": 4096,
    "memory_percent": 45.2
  },
  "streams": [
    {
      "stream_id": "hab_302",
      "fps_actual": 9.8,
      "fps_target": 10,
      "workers": [
        {
          "worker_id": "person_detector-302-1",
          "avg_inference_ms": 38.2,
          "memory_mb": 520,
          "cpu_percent": 12.5,
          "drop_rate_percent": 0.5
        }
      ]
    }
  ]
}
```

### Acceptance Criteria

- [ ] Resource profiling activo en todos los workers
- [ ] Telemetría publicada cada 30 segundos
- [ ] Resource allocator sugiere worker count por stream
- [ ] Drop statistics por worker
- [ ] Integration tests: Validar telemetría

### Referencias

- [Plan Evolutivo - Sprint 4.2](../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#42-resource-management)

### Lecciones Aprendidas

_Se actualizará al completar el sprint_

---

## 📊 Estado Actual

**Completado**: 0/2 sprints (0%)  
**En Progreso**: -  
**Próximo**: Sprint 4.1 (después de Fase 1)

### Bloqueadores

- ⚠️ Dependencia de Fase 1 completada

### Decisiones Pendientes

- [ ] ¿Usar Prometheus para metrics o custom telemetría?
- [ ] ¿Resource allocator es automático o manual (Room Orchestrator decide)?

---

## 🎯 Preparación para Fase 2

### Pre-requisitos
- ✅ Fase 1 completada (bounded contexts sólidos)
- ✅ Single-stream funcionando sin issues
- ✅ Worker catalog probado
- ✅ Hot-reload validado

### Investigación Previa
- [ ] Benchmarks: 1 vs 4 vs 8 streams en i7
- [ ] GStreamer pipeline optimization para multi-stream
- [ ] Go goroutine profiling con pprof

---

**Última actualización**: 2025-11-03  
**Autor**: Ernesto + Gaby (AI Companion)
