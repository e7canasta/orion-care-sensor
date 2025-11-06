# P002: Frame-Buffer as Separate Module (Heavy Worker Sharing)

**Status**: 🔮 Proposed
**Version**: 1.0
**Created**: 2025-11-06
**Last Updated**: 2025-11-06
**Target Release**: r3.0 (Q3 2026, estimated)
**Superseded by**: N/A

---

## Context

### Business Driver

**Current** (r2.0): N streams, cada uno con swarm de workers dedicado.

**Future Need** (r3.0): Heavy workers (VLM, SAM, YOLO-XL) requieren mucha memoria (8GB VRAM c/u).

**Problem**:
```
Multi-stream: 4 cámaras
VLM worker: 8GB VRAM cada uno
Naive: 4 streams × 8GB = 32GB (no entra en 1 GPU)

Constraint: 1 heavy worker compartido entre N streams
```

**Business Impact**:
- Cost: 1 GPU × shared workers < N GPUs × dedicated workers
- Latency: Acceptable (VLM no es real-time, 2-5s OK)
- Throughput: Multiplexing aumenta latency pero no bloquea real-time workers

---

### Current State (r2.0)

```
Multi-stream (N pipelines independientes):

  stream-capture(s1) → framesupplier(s1) → workers(s1) [PersonDetector, VLM, ...]
  stream-capture(s2) → framesupplier(s2) → workers(s2) [PersonDetector, VLM, ...]
  ...

Workers pesados: N instancias (1 por stream)
```

**Limitation**: N × heavy workers = resource waste (si pueden compartirse).

---

### Future Need (r3.0)

```
Shared heavy workers:

  stream-capture(s1) → framesupplier(s1) ─┐
  stream-capture(s2) → framesupplier(s2) ─┼→ [VLM-worker] (1 instancia)
  stream-capture(sN) → framesupplier(sN) ─┘

Question: ¿Cómo N suppliers entregan a 1 worker sin modificar framesupplier?
```

---

## Evolution Options

### Option A: Frame-Buffer Facade (Linux `tee` Philosophy)

**Approach**:
```
Frame-buffer = Módulo separado que implementa Worker interface

Bounded contexts:
  - framesupplier: Distribution (1 → N), unchanged
  - frame-buffer: Multiplexing (N → 1), NEW MODULE
  - heavy-worker: Inference, unchanged

Composición:
  framesupplier(s1).Subscribe(frameBuffer)  // frameBuffer es "worker"
  framesupplier(s2).Subscribe(frameBuffer)
  frameBuffer.backend = heavyWorker
```

**Implementation**:
```go
// frame-buffer implements Worker interface (facade)
type FrameBuffer struct {
    mu         sync.Mutex
    slots      map[StreamID]*Frame  // 1 slot per stream
    backend    Worker                // VLM-worker (1 instancia)
    scheduler  Scheduler             // FIFO, priority, round-robin
}

func (fb *FrameBuffer) ProcessFrame(f *Frame) Result {
    fb.mu.Lock()
    if fb.slots[f.StreamID] != nil {
        // Ya tengo frame de este stream, drop
        fb.mu.Unlock()
        return Result{Dropped: true}
    }
    fb.slots[f.StreamID] = f
    fb.mu.Unlock()

    // Entregar al backend (scheduling interno)
    result := fb.backend.ProcessFrame(f)

    fb.mu.Lock()
    delete(fb.slots, f.StreamID)  // Libero slot
    fb.mu.Unlock()

    return result
}
```

**Pros**:
- ✅ Bounded contexts limpios (framesupplier unchanged, frame-buffer separate)
- ✅ Composable (Unix philosophy: pipe + tee)
- ✅ Testeable en aislación (frame-buffer tiene su bounded context)
- ✅ Scheduling pluggable (FIFO, priority, round-robin)

**Cons**:
- ⚠️ Nuevo módulo (complejidad agregada)
- ⚠️ Multiplexing latency (N streams → 1 worker → N× slower per stream)

**Complexity**: **Medium** (nuevo módulo, pero bounded context claro)

---

### Option B: Multiplexer in Orchestrator

**Approach**:
```
Orchestrator gestiona multiplexing (no frame-buffer module)

Orchestrator:
  heavyWorker = NewVLMWorker()

  // Orchestrators compiten por heavy worker
  for _, pipeline := range pipelines {
      go orchestrateHeavyWorker(pipeline, heavyWorker)
  }

func orchestrateHeavyWorker(p Pipeline, w Worker) {
    for frame := range p.HeavyWorkerQueue {
        // Acquire lock, process, release
        w.ProcessFrame(frame)
    }
}
```

**Pros**:
- ✅ No nuevo módulo (complejidad en orchestrator)
- ✅ Flexibility (orchestrator decide scheduling)

**Cons**:
- ❌ Orchestrator responsabilidad expandida (violates SRP)
- ❌ No reusable (lógica en orchestrator, no módulo)
- ❌ Testing complejo (orchestrator + pipelines + workers)

**Complexity**: **High** (orchestrator hace demasiado)

---

### Option C: Worker Pool (Self-Assignment)

**Approach**:
```
Workers se auto-asignan a streams disponibles

WorkerPool:
  pool := NewWorkerPool(1)  // 1 heavy worker
  pool.Register(heavyWorker)

  framesupplier(s1).Subscribe(pool.GetWorker)  // pool decide qué worker asignar
  framesupplier(s2).Subscribe(pool.GetWorker)
```

**Pros**:
- ✅ Auto-scaling (pool puede crecer/shrink)

**Cons**:
- ❌ Over-engineering (para 1 worker, pool es overkill)
- ❌ Complejidad alta (pool management, health checks)

**Complexity**: **High** (premature generalization)

---

## Validation (Tests Mentales)

### Test 1: Bounded Contexts

**Option A (Frame-Buffer)**:
```
Bounded contexts:
  - framesupplier: Distribution (unchanged) ✅
  - frame-buffer: Multiplexing (NEW, clear responsibility) ✅
  - heavy-worker: Inference (unchanged) ✅

Responsibilities:
  - framesupplier: NO hace multiplexing ✅
  - frame-buffer: NO hace distribution ✅
  - Composable: framesupplier → frame-buffer → heavy-worker ✅

Test: "¿Un solo motivo para cambiar?"
  - framesupplier: Cambio en drop policy → Solo framesupplier ✅
  - frame-buffer: Cambio en scheduling → Solo frame-buffer ✅
  - heavy-worker: Cambio en modelo → Solo heavy-worker ✅

Conclusión: Bounded contexts limpios ✅
```

**Option B (Multiplexer in Orchestrator)**:
```
Bounded contexts:
  - orchestrator: Orchestration + Multiplexing (2 responsabilidades) ❌

Test: "¿Un solo motivo para cambiar?"
  - Cambio en scheduling → orchestrator cambia ❌
  - Cambio en pipeline lifecycle → orchestrator cambia ❌
  → 2 motivos para cambiar (violates SRP) ❌

Conclusión: Bounded context incorrecto ❌
```

---

### Test 2: Filosofía Linux (Composability)

**Analogía**: `pipe` + `tee` (Unix tools)

```bash
# Linux
cmd1 | tee output.txt | cmd2

Donde:
  pipe (kernel): Distribution 1 → N (como framesupplier)
  tee (userspace): Multiplexing N → 1 (como frame-buffer)
  pipe NO hace multiplexing ✅
  tee USA pipe, módulo separado ✅
```

**En Orion**:
```
stream-capture → framesupplier → frame-buffer → heavy-worker

Donde:
  framesupplier: Distribution 1 → N (como pipe)
  frame-buffer: Multiplexing N → 1 (como tee)
  framesupplier NO hace multiplexing ✅
  frame-buffer USA framesupplier (composition) ✅
```

**Test**: ¿Option A sigue filosofía Linux?
- framesupplier = pipe (simple, una cosa bien) ✅
- frame-buffer = tee (simple, una cosa bien) ✅
- Composición > Monolito ✅

**Conclusión**: Option A es Linux-style ✅

---

### Test 3: Testabilidad en Aislación

**Option A**:
```go
// Test framesupplier (sin frame-buffer)
func TestFrameSupplier(t *testing.T) {
    supplier := framesupplier.New()
    mockWorker := &MockWorker{}
    supplier.Subscribe(mockWorker)
    supplier.Publish(frame)
    assert(mockWorker.received(frame))
}

// Test frame-buffer (sin framesupplier)
func TestFrameBuffer(t *testing.T) {
    backend := &MockBackend{}
    buffer := NewFrameBuffer(backend)

    buffer.ProcessFrame(frame1)  // stream1
    buffer.ProcessFrame(frame2)  // stream2
    buffer.ProcessFrame(frame3)  // stream1 (should drop)

    assert(backend.processedCount == 2)  // frame1, frame2
}
```

**Testability**: ✅ Cada módulo testeable en aislación (low coupling)

---

**Option B**:
```go
// Test orchestrator (necesita pipelines + workers + mocks)
func TestOrchestrator(t *testing.T) {
    // Setup: 3 pipelines + 1 heavy worker + mocks
    // Complex...
}
```

**Testability**: ❌ Requiere muchos mocks (high coupling)

---

## Recommendation

### Preferred: **Option A** (Frame-Buffer Facade)

**Rationale**:
1. **Bounded contexts limpios**: framesupplier unchanged, frame-buffer separate ✅
2. **Filosofía Linux**: Composición (pipe + tee), not monolito ✅
3. **Testability**: Cada módulo testeable en aislación ✅
4. **Reusability**: frame-buffer reusable en otros contextos (no solo VLM)

**Trade-off accepted**:
- Nuevo módulo (complejidad agregada) → Justified by clean boundaries ✅

---

### Why NOT Option B (Multiplexer in Orchestrator)

**Violates**:
- SRP: Orchestrator hace 2 cosas (orchestration + multiplexing) ❌
- Bounded contexts: Responsabilidad no clara ❌
- Reusability: Lógica no reusable (atada a orchestrator) ❌

**Conclusión**: Over-coupling (dios module) ❌

---

## Impact Analysis

### Módulos Afectados (Option A)

| Módulo          | Cambios r3.0                               | Complejidad |
| --------------- | ------------------------------------------ | ----------- |
| framesupplier   | Ninguno (frame-buffer es "worker")         | Low         |
| frame-buffer    | Nuevo módulo (multiplexing N → 1)          | Medium      |
| heavy-worker    | Ninguno (procesa frames como siempre)      | Low         |
| orchestrator    | Composición (conectar suppliers a buffer)  | Low         |

**Total complexity**: **Medium** (nuevo módulo, pero bounded context claro).

---

### Complejidad Frame-Buffer (Detalle)

**Core Logic**:
```go
type FrameBuffer struct {
    mu         sync.Mutex
    slots      map[StreamID]*Frame
    backend    Worker
    scheduler  Scheduler  // FIFO, Priority, RoundRobin
}

func (fb *FrameBuffer) ProcessFrame(f *Frame) Result {
    // 1. Lock slot (1 per stream)
    // 2. Deliver to backend (scheduling)
    // 3. Unlock slot
}
```

**Scheduling Strategies**:
- **FIFO**: First-in, first-out (simple)
- **Priority**: Stream priority (critical streams first)
- **Round-Robin**: Fairness entre streams

**Complexity**: ~200 líneas (core + 3 schedulers)

---

## Decision Checkpoint

### Cuándo implementar:
- [ ] r3.0 pedido (heavy workers + multi-stream)
- [ ] Benchmarks muestran resource overhead crítico (VLM × N > 1 GPU)
- [ ] r2.0 en producción (multi-stream stable)

### Cuándo NO implementar:
- [x] r2.0 no implementado aún (premature)
- [ ] Heavy workers no pedidos (YAGNI)
- [ ] Resource overhead acceptable (1 GPU per stream OK)

---

## References

- **Discovery Session**: FrameSupplier design (2025-11-05), multi-stream evolution
- **Related Proposals**:
  - P001: Multi-Stream Scale Horizontal (habilita este proposal)
- **Related ADRs**:
  - ADR-001: sync.Cond for Mailbox (frame-buffer puede usar misma primitive)
  - ADR-004: Symmetric JIT (frame-buffer también practica JIT)
- **Related Annexes**:
  - ANNEX-001: Thinking in Chains (movimientos futuros test)
  - ANNEX-002: Bounded Contexts (frame-buffer = módulo separado)

---

## Analogía: Linux Pipes + Tee

### Unix Philosophy Applied

```bash
# Linux
camera1 | distribute | tee heavy-worker | emit
camera2 | distribute ↗

Donde:
  distribute (pipe): 1 → N (framesupplier)
  tee: N → 1 (frame-buffer)
  heavy-worker: Procesa (VLM)
```

**Principio**:
> "Write programs that do one thing well.
> Write programs to work together." — Doug McIlroy

**En Orion**:
- framesupplier: Distributes (one thing well)
- frame-buffer: Multiplexes (one thing well)
- Composable: framesupplier + frame-buffer (work together)

---

**Version History**:
- v1.0 (2025-11-06): Initial proposal (discovery session insights, Linux philosophy)
