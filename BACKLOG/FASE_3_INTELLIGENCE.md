# 🧠 Fase 3: Intelligence (v2.0 → v3.0)

**Milestone**: [v3.0 - Intelligence](https://github.com/e7canasta/orion-care-sensor/milestone/3)  
**Due Date**: 2025-06-30  
**Objetivo**: Focus dinámico, motion pooling, resource optimization

---

## 📋 Sprints Overview

| Sprint | Issue | Status | Estimación | Bounded Context |
|---|---|---|---|---|
| **5.1** | [#6](https://github.com/e7canasta/orion-care-sensor/issues/6) | 📅 Planned | 2 semanas | Motion Pooling |
| **5.2** | TBD | 📅 Planned | 2 semanas | Dynamic Allocation |

**Total estimado**: 4 semanas

---

## 🔨 Sprint 5.1: Motion Detection Pooling

**Issue**: [#6 - Fase 3: Intelligence - Cell Orchestration](https://github.com/e7canasta/orion-care-sensor/issues/6)

### Bounded Context

**Responsabilidad:**
- ✅ Monitorear actividad en N streams
- ✅ Detección de motion (lightweight)
- ✅ Señalización de "actividad" vs "sleep"

**Anti-responsabilidad:**
- ❌ NO es Scene Expert (eso es Care Scene)
- ❌ NO interpreta eventos clínicos
- ❌ NO toma decisiones de alertas

### Entregables

```
internal/cell/          # NUEVO
├── orchestrator.go     # Cell coordination
└── motion_pool.go      # Motion detection

cmd/cell-orchestrator/  # NUEVO (separado de Orion)
└── main.go             # Cell orchestrator service
```

### Architecture

```
Cell Orchestrator (proceso separado):
  ├─ Monitorea 8 streams vía motion pool
  │  ├─ Motion detection lightweight (OpenCV optical flow)
  │  └─ Threshold: Motion > 5% frame area
  │
  ├─ Detecta actividad:
  │  ├─ Hab 302 → Motion detected → Spawn Full Orion
  │  ├─ Hab 303-308 → No motion → Motion pool only
  │
  └─ Coordina recursos:
     ├─ Full Orion: person_detector + pose + flow (4 workers)
     └─ Motion pool: lightweight motion detection (CPU only)
```

### Motion Detection Strategy

```yaml
# config/cell_orchestrator.yaml
cell_id: cell-3A
streams:
  - stream_id: hab_302
    rtsp_url: rtsp://cam-302/stream
    mode: auto  # auto | full | motion_only
  
  - stream_id: hab_303
    rtsp_url: rtsp://cam-303/stream
    mode: auto

motion_detection:
  algorithm: optical_flow  # optical_flow | frame_diff
  threshold_percent: 5.0
  min_area_pixels: 500
  
  hysteresis:
    activate_delay_sec: 2    # 2s de motion para activar full Orion
    deactivate_delay_sec: 120  # 2min sin motion para desactivar
```

### Acceptance Criteria

- [ ] Motion pool detecta actividad en streams
- [ ] Threshold configurable vía YAML
- [ ] Spawn Full Orion cuando motion > threshold
- [ ] Apagar Full Orion después de hysteresis
- [ ] Telemetría de motion events
- [ ] Unit tests: Mock motion detection
- [ ] Integration tests: 8 streams con motion artificial

### Performance Targets

| Hardware | Streams (Motion) | Streams (Full) | Total CPU | Total Memory |
|---|---|---|---|---|
| i7-10700 8C/16T | 8 | 2 | ~60% | ~2GB |
| i9-12900K 16C/24T | 8 | 4 | ~50% | ~4GB |

### Referencias

- [C4 Model - Cell Orchestrator](../docs/DESIGN/C4_MODEL.md#c2---container-diagram)
- [Plan Evolutivo - Sprint 5.1](../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#51-motion-detection-pooling)

### Lecciones Aprendidas

_Se actualizará al completar el sprint_

---

## 🔨 Sprint 5.2: Dynamic Resource Allocation

**Issue**: TBD (se creará después de Sprint 5.1)

### Bounded Context

**Responsabilidad:**
- ✅ Asignación dinámica de workers por stream
- ✅ Priorización de streams (ej. habitación crítica)
- ✅ Balanceo de carga en tiempo real

**Anti-responsabilidad:**
- ❌ NO es Room Orchestrator (eso es Care Scene)
- ❌ NO conoce scenarios clínicos

### Entregables

```
internal/cell/
├── allocator.go        # NUEVO: Dynamic allocation logic
└── priority_queue.go   # NUEVO: Stream priority queue
```

### Allocation Strategy

**Prioridad de Streams:**
1. **Critical**: Habitación con alerta activa (ej. José en borde de cama)
2. **High**: Habitación con motion reciente (<2 min)
3. **Medium**: Habitación con motion antiguo (>2 min)
4. **Low**: Habitación sin motion (>10 min)

**Resource Allocation:**
```
i7-10700 (8 cores):
  ├─ 2 cores reservados para OS + GStreamer
  ├─ 6 cores disponibles para workers
  │
  ├─ Critical stream (hab_302):
  │  └─ 3 workers (person + pose + flow) → 3 cores
  │
  ├─ High stream (hab_303):
  │  └─ 2 workers (person + pose) → 2 cores
  │
  └─ Medium/Low streams (hab_304-308):
     └─ Motion pool only → 1 core compartido
```

### Acceptance Criteria

- [ ] Priorización de streams funcional
- [ ] Allocator asigna workers según prioridad
- [ ] Re-balanceo en tiempo real (ej. nueva alerta)
- [ ] Telemetría de allocation decisions
- [ ] Integration tests: Cambio de prioridad dinámico

### Referencias

- [Plan Evolutivo - Sprint 5.2](../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#fase-3-intelligence-v20--v30)

### Lecciones Aprendidas

_Se actualizará al completar el sprint_

---

## 📊 Estado Actual

**Completado**: 0/2 sprints (0%)  
**En Progreso**: -  
**Próximo**: Sprint 5.1 (después de Fase 2)

### Bloqueadores

- ⚠️ Dependencia de Fase 2 completada (multi-stream sólido)
- ⚠️ Investigación: ¿OpenCV optical flow o solución custom?

### Decisiones Pendientes

- [ ] ¿Cell Orchestrator es proceso separado o módulo dentro de Orion?
- [ ] ¿Motion detection en Go (via CGo) o Python subprocess?
- [ ] ¿Priorización manual (Room Orchestrator) o automática (Cell Orchestrator)?

---

## 🎯 Preparación para Fase 3

### Pre-requisitos
- ✅ Fase 2 completada (multi-stream funcionando)
- ✅ Resource profiling validado
- ✅ Telemetría rica disponible
- ✅ Performance benchmarks en hardware target

### Investigación Previa
- [ ] OpenCV optical flow performance (CPU only)
- [ ] Frame diff vs optical flow trade-offs
- [ ] Hysteresis tuning (activación/desactivación)
- [ ] Priority queue algorithms (FIFO vs weighted)

---

## 🚀 Vision: Más Allá de v3.0

### Posibles Futuras Fases

**Fase 4: GPU Acceleration (v3.0 → v4.0)**
- OpenVINO support (iGPU Intel)
- Multi-GPU support (NVIDIA)
- Batching optimization

**Fase 5: Edge-Cloud Hybrid (v4.0 → v5.0)**
- Cloud fallback para inferencias pesadas
- Federated learning
- Model versioning & A/B testing

**Fase 6: Multi-Cell Coordination (v5.0 → v6.0)**
- Cell-to-cell communication
- Global resource balancing
- Distributed motion pooling

_Estas fases son exploratorias. Se definirán después de validar v3.0 en producción._

---

**Última actualización**: 2025-11-03  
**Autor**: Ernesto + Gaby (AI Companion)
