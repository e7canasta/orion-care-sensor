# ANNEX-003: Physical Invariants (Cuando la Física Simplifica el Diseño)

**Meta-Principio**: Si Latencia A >> Intervalo B, Orden Garantizado por Física
**Aka**: "La física como simplificador de arquitectura"
**Contexto**: Sistemas real-time donde latencias y throughput son conocidos

---

## El Problema

### Anti-Pattern: Sobre-Sincronización
```
Decisión: "Necesitamos orden, agreguemos wg.Wait()"
          ↓
Código:
  for _, worker := range workers {
      go func(w Worker) {
          w.Process(frame)
          wg.Done()
      }(worker)
  }
  wg.Wait()  // Bloquea hasta que todos terminen

Problema:
- Publisher bloqueado (30fps stream → puede llegar T+1 antes de wg.Wait())
- Sincronización innecesaria (si distribution << inter-frame)
- Complejidad agregada (waitgroups, error propagation)
```

**Síntoma**: Código síncrono cuando física garantiza orden.

---

### Pattern Correcto: Fire-and-Forget
```
Decisión: "Si distribution << inter-frame, física garantiza orden"
          ↓
Análisis:
  Distribution latency: 100µs
  Inter-frame @ 30fps: 33,333µs
  Ratio: 333×

  Para que T+1 sobrepase T:
    distribution_time > inter_frame_time
    100µs > 33,333µs ← Imposible (salvo system collapse)

Código:
  for _, worker := range workers {
      go func(w Worker) {
          w.Process(frame)
          // No wg.Done(), fire-and-forget
      }(worker)
  }
  // No wg.Wait(), retorna inmediatamente
```

**Síntoma**: Código asíncrono cuando física permite.

---

## El Principio: Physical Invariants of the System

### Definición
> "Si la latencia de componente A es órdenes de magnitud menor que el intervalo de componente B,
> el orden está garantizado por la física del sistema, no por sincronización explícita."

**Matemáticamente**:
```
Si: latency(A) << interval(B)  (ejemplo: ratio > 100×)
Entonces: order(A_n, A_n+1) guaranteed by physics
No necesitamos: explicit synchronization (wg.Wait, channels buffering, etc.)
```

---

### Por Qué Funciona

**Análisis de Overtaking** (Frame T+1 sobrepasa T):

```
Escenario: Queremos que T+1 llegue antes que T termine distribución

Timeline:
  t=0:     Publish(T)   → Inicia distribution (100µs)
  t=100µs: Publish(T+1) → ¿Puede llegar antes de t=100µs? NO (T ya terminó)

Para overtaking:
  inter_frame < distribution_latency
  33,333µs < 100µs ← FALSO

Conclusión: Overtaking imposible (física lo impide)
```

**Implicación**: No necesitamos `wg.Wait()` (simplifica diseño).

---

## Tests Mentales (Durante Discovery)

### Test 1: Ratio de Latencias
**Pregunta**: "¿Cuál es el ratio latency(A) / interval(B)?"

```
Caso: FrameSupplier distribution
  latency(distribution): 100µs (measured)
  interval(frames @ 30fps): 33,333µs
  Ratio: 100µs / 33,333µs = 0.003 (A es 333× más rápido que B)

Conclusión: Ratio << 1 → Fire-and-forget correcto ✅
```

**Threshold**: Si ratio < 0.01 (A es >100× más rápido), física garantiza orden.

---

### Test 2: Tolerancia a Variabilidad
**Pregunta**: "¿Qué pasa en el peor caso (p99)?"

```
Caso: FrameSupplier distribution con jitter
  latency(distribution):
    p50: 50µs
    p99: 150µs (peor caso con GC pause)
  interval(frames @ 30fps): 33,333µs

Análisis:
  Peor caso: 150µs
  ¿150µs < 33,333µs? → SÍ (ratio = 0.0045, aún 222× más rápido)

Conclusión: Incluso p99, física garantiza orden ✅
```

**Threshold**: Si p99(A) < 0.1 × interval(B), física robusta a variabilidad.

---

### Test 3: System Collapse Detection
**Pregunta**: "¿Cuándo física NO garantiza orden?"

```
Caso: Distribution latency > inter-frame

Scenario:
  distribution: 50,000µs (50ms, sistema colapsado)
  inter-frame @ 30fps: 33,333µs

Análisis:
  T=0: Publish(frame_1) → Inicia distribution (50ms)
  T=33ms: Publish(frame_2) → T=33ms < T=50ms (frame_1 aún procesando)
  → Overtaking posible ❌

Pero:
  Si distribution tarda 50ms, sistema ya colapsó (no mantiene 30fps)
  → Problema más grave que orden (need backpressure, not wg.Wait)
```

**Conclusión**: Si física NO garantiza orden → Sistema colapsado (rediseñar, no parchar).

---

## Heurísticas para Identificar Physical Invariants

### Heurística 1: Memory Bandwidth vs Network Latency

```
Memory copy: 10 GB/s (DDR4)
Network RTT: 100ms (typical internet)

Ratio: 100ms / (1MB/10GB/s) = 100ms / 0.1ms = 1000×

Implicación:
  Si procesamos <1MB por request → Memory sempre gana
  → Zero-copy importante (pero no por latency, por throughput)
```

---

### Heurística 2: CPU vs I/O

```
CPU instruction: 1ns
Disk seek: 10ms

Ratio: 10,000,000×

Implicación:
  Cualquier lógica CPU << disk I/O
  → No optimizar CPU antes de optimizar I/O
```

---

### Heurística 3: Cache Hierarchy

```
L1 cache: 1ns
L2 cache: 10ns
RAM: 100ns
Disk: 10,000,000ns

Implicación:
  Loop unrolling (save 5 instructions) = 5ns
  Cache miss penalty = 100ns
  → Data layout > instruction count
```

---

## Aplicaciones en Orion 2.0

### Caso 1: FrameSupplier Distribution (Fire-and-Forget)

**Context**:
```
Distribution: 100µs (batch to 64 workers)
Inter-frame @ 30fps: 33,333µs
Ratio: 333×
```

**Decision**: Fire-and-forget (no wg.Wait)

**Rationale**:
- Física garantiza orden (ratio >> 100×)
- wg.Wait() innecesario (complejidad evitable)
- Si distribution > inter-frame → Sistema colapsó (no rescatable con sync)

**ADR Reference**: ADR-003 (Fire-and-forget Distribution)

---

### Caso 2: Stream-Capture GStreamer Pipeline (Async Callbacks)

**Context**:
```
GStreamer callback: appsink delivers frame
Callback latency: <1ms (memcpy + metadata)
Inter-frame @ 30fps: 33ms
Ratio: 33×
```

**Decision**: Async callback (no blocking)

**Rationale**:
- Callback << inter-frame (ratio > 30×)
- Blocking callback → GStreamer pipeline stalls (unacceptable)
- Fire-and-forget a FrameSupplier (ya validado arriba)

---

### Caso 3: Worker IPC (MsgPack Framing)

**Context**:
```
MsgPack encode/decode: 1-2ms
Inference latency: 20-2000ms (YOLO/VLM)
Ratio: 10-2000×
```

**Decision**: No pipeline IPC (simple request-response)

**Rationale**:
- IPC << inference (ratio >> 10×)
- Pipelining IPC (overlap encode/inference) = premature optimization
- Simplicity > marginal gain (<5% speedup)

**ADR Reference**: ADR-002 (MsgPack IPC Protocol)

---

## Cuando NO Aplicar (Excepciones)

### Excepción 1: Latencias Comparables

```
Component A latency: 50ms
Component B interval: 60ms
Ratio: 0.83 (comparable)

→ NO aplicar fire-and-forget (overtaking probable)
→ Necesitamos synchronization explícita
```

---

### Excepción 2: Variabilidad Alta (High Jitter)

```
Component A latency:
  p50: 10µs
  p99: 500µs (50× variabilidad)

Component B interval: 1000µs

Ratio:
  p50: 10µs / 1000µs = 0.01 (ok)
  p99: 500µs / 1000µs = 0.5 (borderline)

→ Validar p99, no p50 (worst case matters)
```

---

### Excepción 3: Hard Real-Time Requirements

```
Sistema: Safety-critical (automotive, medical)
Requirement: WCET (Worst-Case Execution Time) guarantees

→ Física NO suficiente (necesitamos formal proof)
→ Usar explicit synchronization + formal verification
```

---

## Ejemplo Completo: Fire-and-Forget Analysis

### Propuesta Inicial (Con Sync)

```go
// Con wg.Wait() (innecesario)
func (fs *FrameSupplier) Publish(frame *Frame) {
    var wg sync.WaitGroup
    for _, slot := range fs.workers {
        wg.Add(1)
        go func(s *WorkerSlot) {
            defer wg.Done()
            select {
            case s.ch <- frame:
            default:
                atomic.AddUint64(&s.drops, 1)
            }
        }(slot)
    }
    wg.Wait()  // Bloquea hasta que todos terminen
}
```

**Problema**: Publisher bloqueado (reduce throughput).

---

### Physical Invariant Analysis

```
Benchmark distribution (64 workers):
  Sequential: 32µs
  Batched: 24µs
  With wg.Wait(): 28µs (overhead de sync)

Inter-frame @ 30fps: 33,333µs

Ratio (worst case): 32µs / 33,333µs = 0.00096 (1041× más rápido)

Pregunta: ¿Puede frame T+1 sobrepasar frame T?
  Para overtaking: distribution_time > 33,333µs
  Medido: 32µs
  → Imposible (requiere slowdown de 1000×)

Si sistema tan lento:
  - 30fps → 0.03fps (collapse total)
  - Problema mayor que orden (need to restart, not sync)
```

**Conclusión**: wg.Wait() innecesario (física garantiza orden).

---

### Propuesta Final (Sin Sync)

```go
// Fire-and-forget (física garantiza orden)
func (fs *FrameSupplier) Publish(frame *Frame) {
    for _, slot := range fs.workers {
        go func(s *WorkerSlot) {
            select {
            case s.ch <- frame:
            default:
                atomic.AddUint64(&s.drops, 1)
            }
        }(slot)
    }
    // No wg.Wait(), retorna inmediatamente
}
```

**Beneficio**: Throughput aumentado (28µs → 24µs, 14% faster).

---

## Emergent Pattern: "Trust the Physics"

### Patrón
```
Cuando diseñamos sistemas real-time:
1. Medir latencias (benchmark)
2. Calcular ratios (latency / interval)
3. Si ratio << 1 (>100×) → Física garantiza propiedades
4. Simplificar diseño (no agregar sync innecesaria)
```

### Contra-Patrón
```
Cuando NO sabemos latencias:
❌ "Agreguemos sync por las dudas" (defensive programming incorrecto)
✅ "Midamos primero, decidamos después" (engineering correcto)
```

---

## Checklist (Durante Discovery)

```
☐ 1. Identificar componentes con latency/interval conocidos
☐ 2. Benchmark: Medir latencies (p50, p99)
☐ 3. Calcular ratio: latency(A) / interval(B)
☐ 4. Test: ¿Ratio < 0.01? (A es >100× más rápido)
☐ 5. Test: ¿p99 también cumple? (worst case robusto)
☐ 6. Validar: ¿Qué pasa si ratio ≈ 1? (sistema colapsó)
☐ 7. Decisión: Si física garantiza → Simplificar (no sync explícita)
☐ 8. Documentar: Rationale con benchmarks (ADR)
☐ 9. Evitar: Sync "por las dudas" (medir, no asumir)
☐ 10. Evitar: Optimizar prematuramente (física da headroom)
```

---

## Golden Rule

> **"Si la física garantiza una propiedad, no la reimplementes en código."**
> **"Medir, calcular, simplificar."**

**Corolario**: "Si necesitas sync cuando física garantiza orden → Sistema colapsado (rediseñar, no parchar)."

---

## Referencias

- **Protocol**: [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md)
- **Related**: [ANNEX-001: Thinking in Chains](./ANNEX-001_THINKING_IN_CHAINS.md)
- **ADR**: ADR-003 (Fire-and-forget Distribution)

---

**Versión**: 1.0
**Autor**: Pair-discovery session (Ernesto + Claude)
**Fecha**: 2025-11-06
**Status**: 🟢 Activo (patrón validado en FrameSupplier)
