# P001: Multi-Stream Scale Horizontal

**Status**: 🔮 Proposed
**Version**: 1.0
**Created**: 2025-11-06
**Last Updated**: 2025-11-06
**Target Release**: r2.0 (Q1 2026, estimated)
**Superseded by**: N/A

---

## Context

### Business Driver

**Current**: Orion monitorea 1 cámara por instancia (POC, primeros clientes).

**Future Need**: Clientes con múltiples salas/habitaciones requieren N cámaras por instancia.

**Example**:
- Geriátrico: 4 habitaciones → 4 cámaras
- Hospital: 10 salas → 10 cámaras
- Nursing home: 32 habitaciones → 32 cámaras

**Business Impact**:
- Deployment cost: 1 instance × N cameras < N instances × 1 camera
- Operational simplicity: 1 config file vs N config files
- Resource utilization: Shared workers (PersonDetector procesa frames de todas las cámaras)

---

### Current State (r1.0)

```
Pipeline (single stream):
  stream-capture → framesupplier → [workers] → event-emitter

Bounded contexts:
  - stream-capture: Adquiere frames de 1 RTSP stream
  - framesupplier: Distribuye frames a N workers
  - workers: Procesan frames (PersonDetector, VLM, etc.)
```

**Limitation**: 1 stream hardcoded (no stream_id metadata).

---

### Future Need (r2.0)

```
Pipeline (multi-stream):
  stream-capture(s1) → framesupplier(s1) → [workers(s1)]
  stream-capture(s2) → framesupplier(s2) → [workers(s2)]
  ...
  stream-capture(sN) → framesupplier(sN) → [workers(sN)]

Or:
  stream-capture(s1) ──┐
  stream-capture(s2) ──┼→ framesupplier(global) → [workers(mixed)]
  stream-capture(sN) ──┘
```

**Question**: ¿N pipelines independientes? ¿O 1 pipeline global con stream_id routing?

---

## Evolution Options

### Option A: N Independent Pipelines (Scale Horizontal Puro)

**Approach**:
```
Cada stream = 1 pipeline completo (aislado)

Deployment:
  pipeline1 := NewPipeline(config1)  // stream1
  pipeline2 := NewPipeline(config2)  // stream2
  ...

Bounded contexts:
  - stream-capture(si): Instancia independiente
  - framesupplier(si): Instancia independiente
  - workers(si): Swarm dedicado a stream si
```

**Pros**:
- ✅ Zero coupling (cada pipeline aislado)
- ✅ Fault isolation (crash stream1 ≠ crash stream2)
- ✅ Simple (r1.0 code sin cambios)
- ✅ Testeable (cada pipeline independiente)

**Cons**:
- ⚠️ Resource overhead (N × workers, puede ser wasteful)
- ⚠️ No worker sharing (PersonDetector × N, desperdicio si low load)

**Complexity**: **Low** (r1.0 code unchanged, orquestador maneja N pipelines)

---

### Option B: 1 Pipeline Global con stream_id Routing

**Approach**:
```
1 pipeline global, frames con metadata stream_id

stream-capture(si) entrega frame con stream_id
framesupplier distribuye a workers (routing by stream_id)
workers procesan frames de cualquier stream

Metadata:
  Frame {
    data: []byte
    stream_id: string  // NEW
    timestamp: time.Time
  }
```

**Pros**:
- ✅ Worker sharing (1 PersonDetector procesa N streams)
- ✅ Resource efficient (no N × workers overhead)

**Cons**:
- ❌ Coupling (failure en routing afecta todos los streams)
- ❌ Complejidad (framesupplier necesita routing logic)
- ❌ Testing (edge cases: stream interleaving, fairness)

**Complexity**: **High** (refactor framesupplier, nueva responsabilidad)

---

### Option C: Hybrid (N Pipelines + Worker Sharing)

**Approach**:
```
N pipelines independientes (Option A)
Workers compartidos via frame-buffer facade (Option B beneficio)

Deployment:
  pipeline1 := NewPipeline(config1)
  pipeline2 := NewPipeline(config2)

  // Heavy worker compartido
  heavyWorker := NewVLMWorker()
  frameBuffer := NewFrameBuffer(heavyWorker)

  pipeline1.Subscribe(frameBuffer)  // frame-buffer es "worker" para pipeline1
  pipeline2.Subscribe(frameBuffer)  // frame-buffer es "worker" para pipeline2
```

**Pros**:
- ✅ Low coupling (pipelines aislados)
- ✅ Worker sharing (para workers pesados)
- ✅ Composable (frame-buffer = módulo separado)

**Cons**:
- ⚠️ Complejidad media (nuevo módulo: frame-buffer)
- ⚠️ No automático (user decide qué workers compartir)

**Complexity**: **Medium** (r1.0 unchanged, frame-buffer nuevo módulo)

---

## Validation (Tests Mentales)

### Test 1: Scale Horizontal

**Option A**:
```
r1.0: pipeline = New(config)
r2.0: pipeline1 = New(config1)
      pipeline2 = New(config2)

¿Cambios en r1.0 code? NO ✅
¿Instancias independientes? SÍ ✅

Conclusión: Scale horizontal puro ✅
```

**Option B**:
```
r1.0: framesupplier.Publish(frame)
r2.0: framesupplier.Publish(frame, stream_id)  // API change

¿Cambios en r1.0 code? SÍ (refactor) ❌
¿Backward compatible? NO (breaking change) ❌

Conclusión: NO es scale horizontal ❌
```

**Option C**:
```
r1.0: pipeline = New(config)
r2.0: pipeline1 = New(config1)
      pipeline2 = New(config2)
      frameBuffer = NewFrameBuffer(heavyWorker)
      pipeline1.Subscribe(frameBuffer)

¿Cambios en r1.0 code? NO (solo composición) ✅
¿Instancias independientes? SÍ ✅

Conclusión: Scale horizontal con worker sharing opcional ✅
```

---

### Test 2: Movimientos Futuros (r3.0+)

**¿Qué pasa si en r3.0 necesitamos worker prioritization?**

**Option A**:
- Workers aislados por stream → Priority per-stream (simple) ✅

**Option B**:
- Workers globales → Priority global (complejo, fairness entre streams) ⚠️

**Option C**:
- Workers aislados + frame-buffer opcional → Priority per-stream O global (flexible) ✅

**Conclusión**: Option A y C preservan movimientos ✅

---

### Test 3: Bounded Contexts

**Option A**:
- stream-capture(si): Responsabilidad clara (1 stream) ✅
- framesupplier(si): Responsabilidad clara (distribución para 1 stream) ✅
- orquestador: Nueva responsabilidad (gestión N pipelines) ⚠️

**Option B**:
- stream-capture(si): Responsabilidad clara ✅
- framesupplier(global): Responsabilidad expandida (routing por stream_id) ❌
- orquestador: Sin cambios ✅

**Option C**:
- stream-capture(si): Responsabilidad clara ✅
- framesupplier(si): Responsabilidad clara ✅
- frame-buffer: Nueva responsabilidad (multiplexing N → 1) ✅
- orquestador: Gestión N pipelines + composición opcional ⚠️

**Conclusión**: Option A y C mantienen bounded contexts limpios ✅

---

## Recommendation

### Preferred: **Option A** (Scale Horizontal Puro)

**Rationale**:
1. **Simplicity**: r1.0 code unchanged (YAGNI máximo)
2. **Fault isolation**: Crash stream1 ≠ crash stream2
3. **Testability**: Cada pipeline testeable en aislación
4. **Bounded contexts**: Responsabilidades claras

**Trade-off accepted**:
- Resource overhead (N × workers) → Acceptable en r2.0 (4-10 streams típico)
- Si r3.0 requiere worker sharing → Agregar Option C (frame-buffer) ✅

---

### Fallback: **Option C** (Hybrid)

**Si en r2.0 ya sabemos que workers pesados son problema**:
- Implementar Option A (N pipelines)
- Agregar frame-buffer module (worker sharing opcional)
- Composición: User decide qué workers compartir

**Beneficio**:
- Mejor de ambos mundos (aislamiento + sharing)
- Complejidad localizada (frame-buffer = módulo aparte)

---

## Impact Analysis

### Módulos Afectados (Option A)

| Módulo         | Cambios r2.0                                  | Complejidad |
| -------------- | --------------------------------------------- | ----------- |
| stream-capture | Ninguno (instanciar N veces)                  | Low         |
| framesupplier  | Ninguno (instanciar N veces)                  | Low         |
| workers        | Ninguno (instanciar N swarms)                 | Low         |
| orquestador    | Gestión N pipelines (loop, configs, shutdown) | Medium      |
| control-plane  | Routing comandos a pipeline correcto          | Medium      |

**Total complexity**: **Medium** (solo orquestador y control-plane cambian).

---

### Módulos Afectados (Option C)

| Módulo         | Cambios r2.0                                  | Complejidad |
| -------------- | --------------------------------------------- | ----------- |
| stream-capture | Ninguno                                       | Low         |
| framesupplier  | Ninguno                                       | Low         |
| frame-buffer   | Nuevo módulo (multiplexing N → 1)             | Medium      |
| workers        | Ninguno (heavies usan frame-buffer)           | Low         |
| orquestador    | Gestión N pipelines + composición frameBuffer | High        |

**Total complexity**: **High** (nuevo módulo + orquestador más complejo).

---

## Decision Checkpoint

### Cuándo implementar Option A:
- [x] r2.0 pedido explícitamente (multi-stream business need)
- [ ] POC validado en producción (r1.0 stable)
- [ ] Resource overhead acceptable (4-10 streams, no problema)

### Cuándo implementar Option C:
- [ ] r2.0 + Heavy workers (VLM, SAM) pedidos juntos
- [ ] Benchmarks muestran resource overhead crítico (>10 streams)
- [ ] frame-buffer design validado (ver PROPOSAL/P002)

### Cuándo NO implementar:
- [ ] r1.0 aún no en producción (premature)
- [ ] r2.0 no pedido (YAGNI)

---

## References

- **Discovery Session**: FrameSupplier design (2025-11-05)
- **Related ADRs**:
  - ADR-001: sync.Cond for Mailbox (habilita instancias independientes)
  - ADR-004: Symmetric JIT (cada pipeline tiene inbox independiente)
- **Related Proposals**:
  - P002: Frame-Buffer as Separate Module (Option C fallback)
- **Related Annexes**:
  - ANNEX-001: Thinking in Chains (scale horizontal test)
  - ANNEX-002: Bounded Contexts (responsabilidades claras)

---

**Version History**:
- v1.0 (2025-11-06): Initial proposal (discovery session insights)
