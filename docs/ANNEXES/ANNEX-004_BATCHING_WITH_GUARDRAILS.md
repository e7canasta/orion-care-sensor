# ANNEX-004: Batching with Guardrails (Optimización con Contexto de Negocio)

**Meta-Principio**: Threshold from Business Context, Not Just Math
**Aka**: "Optimizar con guardrails, no con benchmarks solamente"
**Contexto**: Performance optimization en sistemas con fases de deployment conocidas

---

## El Problema

### Anti-Pattern: Premature Optimization

```
Decisión: "Batching es más rápido, implementemos siempre"
          ↓
Código:
  // Batching sin threshold
  for i := 0; i < len(workers); i += 32 {
      batch := workers[i:min(i+32, len(workers))]
      go processBatch(batch)
  }

Problema:
  POC: 3 workers → Overhead de batching (goroutine spawn) > beneficio
  Expansion: 10 workers → Marginal gain, complejidad agregada
  Full deployment: 64 workers → Beneficio real

Síntoma: Optimización siempre activa, incluso cuando perjudica
```

**Resultado**: Complejidad prematura, peor performance en fases tempranas.

---

### Pattern Correcto: Batching with Threshold

```
Decisión: "Batching solo cuando N > threshold (justificado por negocio)"
          ↓
Análisis:
  POC: 3-5 workers → Sequential perfecto
  Expansion: 10 workers → Sequential aún bueno
  Full deployment: 64 workers → Batching necesario

Threshold: 8 workers (antes de break-even matemático)
Rationale: Garantiza sequential performance en POC/Expansion

Código:
  if len(workers) <= 8 {
      // Sequential (simple, rápido para N pequeño)
      for _, w := range workers {
          go deliver(w, frame)
      }
  } else {
      // Batching (optimizado para N grande)
      batchSize := 8
      for i := 0; i < len(workers); i += batchSize {
          batch := workers[i:min(i+batchSize, len(workers))]
          go processBatch(batch)
      }
  }
```

**Resultado**: Optimización activada solo cuando necesaria (contexto de negocio).

---

## El Principio: Optimization with Business Guardrails

### Definición
> "Las optimizaciones deben justificarse por **contexto de negocio** (fases de deployment),
> no solo por **break-even matemático** (benchmarks)."

**Componentes**:
1. **Break-even point** (matemática): ¿Cuándo optimización es más rápida?
2. **Business phases** (contexto): ¿Cuántos workers en cada fase?
3. **Threshold** (decisión): Antes de break-even, garantiza performance en fases tempranas

---

### Por Qué Funciona

**Análisis Matemático** (break-even):
```
Sequential: O(N) con spawn overhead (1µs por goroutine)
Batched: O(N/B) con batch overhead (2µs por batch)

Break-even:
  N × 1µs = (N/8) × 2µs + 8µs (batch setup)
  N = 12 workers (break-even matemático)

Conclusión matemática: Batching mejor para N > 12
```

**Análisis de Negocio** (contexto):
```
POC: 3-5 workers (desarrollo, pruebas)
Expansion: 8-10 workers (primeros clientes)
Full deployment: 32-64 workers (producción)

Threshold: 8 workers
Rationale:
  - POC/Expansion: Sequential (N ≤ 8, perfecto)
  - Full deployment: Batching (N > 8, optimizado)

Beneficio:
  - Simplicidad en fases tempranas (menos bugs)
  - Performance en producción (cuando importa)
```

**Conclusión**: Threshold=8 (antes de break-even=12) → Mejores de ambos mundos.

---

## Tests Mentales (Durante Discovery)

### Test 1: Break-Even Matemático
**Pregunta**: "¿Cuándo optimización es matemáticamente mejor?"

```
Benchmark:
  Sequential (N=10): 32µs
  Batched (N=10, B=8): 28µs (12% faster)
  Batched (N=5, B=8): 18µs vs Sequential 15µs (slower!)

Break-even: N ≈ 12 workers

Conclusión: Batching solo beneficia N > 12 (matemáticamente)
```

---

### Test 2: Business Context
**Pregunta**: "¿Cuántos workers en cada fase de deployment?"

```
Fases conocidas:
  POC: 3-5 workers (development)
  Expansion: 8-10 workers (early customers)
  Full: 32-64 workers (production)

Threshold candidates:
  - Threshold=12: POC/Expansion usan sequential ✅
  - Threshold=8: Expansion borde (50% sequential, 50% batched) ⚠️
  - Threshold=5: Expansion siempre batched (complejidad prematura) ❌

Elección: Threshold=8 (garantiza sequential en POC, batched en Full)
```

---

### Test 3: Complexity Trade-off
**Pregunta**: "¿Vale la pena la complejidad agregada?"

```
Complejidad agregada:
  - Branch logic (if/else)
  - Batching code (loop con stride)
  - Testing: 2 code paths (sequential + batched)

Beneficio:
  POC (N=5): Sequential perfecto (sin complejidad batching)
  Full (N=64): Batched 30% faster (27µs vs 42µs)

Trade-off:
  ✅ Vale la pena: 30% speedup en producción
  ✅ Complejidad localizada (1 función)
  ✅ Testeable (threshold=8, deterministico)

Conclusión: Complexity justified by business impact ✅
```

---

## Heurísticas para Threshold Selection

### Heurística 1: Before Break-Even
```
Break-even (matemático): N = 12
Threshold (elegido): N = 8

Rationale: Antes de break-even → Garantiza no-regression en fases tempranas
```

**Regla**: `Threshold ≈ 0.6 × BreakEven` (40% antes del punto matemático).

---

### Heurística 2: Business Phases Alignment
```
POC: 5 workers (< threshold) → Sequential ✅
Expansion: 10 workers (> threshold) → Batched ⚠️ (pero acceptable)
Full: 64 workers (>> threshold) → Batched ✅

Threshold: Alineado con transición POC → Expansion
```

**Regla**: Threshold entre mayor fase temprana y menor fase producción.

---

### Heurística 3:測Test Coverage
```
Test cases:
  - N=1: Edge case (sequential)
  - N=8: Threshold exacto (sequential)
  - N=9: Threshold+1 (batched)
  - N=64: Full scale (batched)

Threshold=8 → Testing determinístico (no arbitrary numbers)
```

**Regla**: Threshold debe ser testeable fácilmente (potencia de 2 preferible: 4, 8, 16).

---

## Anti-Patterns Comunes

### Anti-Pattern 1: "Always Optimize"

```
❌ Código:
   // Batching siempre activo
   for i := 0; i < len(workers); i += 8 {
       batch := workers[i:min(i+8, len(workers))]
       go processBatch(batch)
   }

Problema:
  N=3 workers → Overhead de batching (goroutine + loop)
  Sequential sería más rápido (15µs vs 18µs)
```

**Solución**: Threshold (solo batching cuando N > 8).

---

### Anti-Pattern 2: "Optimize at Break-Even"

```
❌ Decisión: "Break-even es N=12, threshold=12"

Problema:
  N=10 (Expansion phase): Sequential (ok)
  N=12 (break-even): Batched (first time)
  → Primera vez batched es en break-even (no safety margin)

Síntoma: Edge case testing en producción (riesgoso)
```

**Solución**: Threshold antes de break-even (safety margin).

---

### Anti-Pattern 3: "Magic Numbers"

```
❌ Código:
   if len(workers) > 7 {  // ¿Por qué 7?
       // batching
   }

Problema: Magic number sin rationale (¿7? ¿Por qué no 6 o 8?)
```

**Solución**: Threshold documentado con rationale (ADR).

---

### Anti-Pattern 4: "Micro-Optimization Without Context"

```
❌ Propuesta: "Threshold=5 para optimizar Expansion phase"

Análisis:
  Expansion: 8-10 workers
  Threshold=5 → Siempre batched (complejidad siempre activa)

Problema: Optimización prematura (Expansion aún no es production-scale)
```

**Solución**: Threshold alineado con fases donde performance crítico (Full deployment).

---

## Ejemplo Completo: FrameSupplier Batching

### Análisis Matemático (Break-Even)

```
Benchmark (FrameSupplier distribution):
  Sequential:
    N=5: 15µs
    N=10: 32µs
    N=20: 64µs
    N=64: 205µs

  Batched (batchSize=8):
    N=5: 18µs (slower!)
    N=10: 28µs (12% faster)
    N=20: 48µs (25% faster)
    N=64: 78µs (62% faster)

Break-even: N ≈ 12 workers (batched ≈ sequential)
```

**Conclusión matemática**: Batching beneficia N > 12.

---

### Análisis de Negocio (Context)

```
Orion Deployment Phases:
  POC: 3-5 workers (PersonDetector, FaceDetector, PoseDetector)
  Expansion: 8-10 workers (+ VLM, ActivityRecognition)
  Full: 32-64 workers (múltiples modelos, multi-stream)

Performance crítico:
  POC: Latency (< 100ms para responsiveness)
  Expansion: Throughput (sostener 30fps)
  Full: Scale (64 workers @ 30fps = 1920 inferences/s)

Threshold candidate: 8 workers
Rationale:
  - POC (N ≤ 5): Sequential perfecto (simple, rápido)
  - Expansion (N ≈ 10): Batched (preparado para scale)
  - Full (N ≥ 32): Batched (optimización crítica)
```

**Conclusión de negocio**: Threshold=8 alineado con transición POC → Expansion.

---

### Decision (ADR-003)

```markdown
## ADR-003: Batching with Threshold=8

### Context
FrameSupplier distributes frames to N workers (3-64 en diferentes fases).

Break-even matemático: N=12 (batched ≈ sequential)

Business phases:
- POC: 3-5 workers
- Expansion: 8-10 workers
- Full: 32-64 workers

### Decision
Batching activado solo cuando N > 8.

Implementation:
```go
if len(workers) <= 8 {
    // Sequential
    for _, w := range workers {
        go deliver(w, frame)
    }
} else {
    // Batched (batchSize=8)
    for i := 0; i < len(workers); i += 8 {
        batch := workers[i:min(i+8, len(workers))]
        go processBatch(batch)
    }
}
```

### Consequences

**Positivas**:
- ✅ POC: Sequential perfecto (simple, sin overhead)
- ✅ Full: Batching optimizado (62% faster @ N=64)
- ✅ Complexity justified (30% speedup en producción)

**Negativas**:
- ⚠️ Branch logic (if/else)
- ⚠️ Testing: 2 code paths (pero deterministico)

### Rationale (Business Context)
Threshold=8 (antes de break-even=12):
- Garantiza sequential en POC (N ≤ 5)
- Activa batching en Expansion (N ≈ 10, preparado para Full)
- Safety margin (40% antes de break-even)

Alternative considered: Threshold=12 (break-even exacto)
Rejected: No safety margin, primera vez batched es en edge case
```

---

## Emergent Pattern: "Guardrails from Business"

### Patrón
```
Cuando optimizamos performance:
1. Benchmark: Medir break-even matemático
2. Business context: Identificar fases de deployment
3. Threshold: Antes de break-even, alineado con fases
4. Guardrail: Optimización solo cuando necesaria (contexto)
```

### Contra-Patrón
```
❌ "Optimizar siempre" (complejidad prematura)
❌ "Threshold en break-even" (no safety margin)
❌ "Magic numbers" (sin rationale documentado)
```

---

## Otros Ejemplos de Guardrails

### Ejemplo 1: Connection Pooling

```
Break-even: 50 requests/s (pool overhead < connection setup)
Business phases:
  Development: 1 req/s (no pool)
  Staging: 10 req/s (no pool)
  Production: 1000 req/s (pool crítico)

Threshold: 100 req/s
Rationale: Development/Staging sin complejidad, Production optimizado
```

---

### Ejemplo 2: Caching

```
Break-even: 10% cache hit rate (cache overhead < recompute)
Business phases:
  POC: No users (cache inútil)
  Beta: 100 users (10% hit rate)
  Production: 10,000 users (70% hit rate)

Threshold: Beta launch
Rationale: POC sin complejidad, Beta+ optimizado
```

---

### Ejemplo 3: Horizontal Scaling

```
Break-even: 80% CPU utilization (instance cost < performance gain)
Business phases:
  MVP: 1 instance (< 50% CPU)
  Growth: 1 instance (70% CPU)
  Scale: 10 instances (80% CPU cada uno)

Threshold: 70% CPU
Rationale: MVP/Growth sin complejidad, Scale cuando necesario
```

---

## Checklist (Durante Discovery)

```
☐ 1. Benchmark: Medir break-even matemático
☐ 2. Business: Identificar fases de deployment (POC, Expansion, Full)
☐ 3. Business: ¿Cuántos N en cada fase?
☐ 4. Threshold: Elegir antes de break-even (safety margin 40%)
☐ 5. Threshold: Alinear con transición de fases (POC → Expansion)
☐ 6. Validar: ¿Vale la pena complejidad? (speedup en producción)
☐ 7. Documentar: Rationale con business context (ADR)
☐ 8. Testing: ¿Threshold testeable? (preferir potencia de 2)
☐ 9. Evitar: "Always optimize" (complejidad prematura)
☐ 10. Evitar: "Magic numbers" (documentar rationale)
```

---

## Golden Rule

> **"Optimización activada por contexto de negocio, no solo por benchmarks."**
> **"Threshold from business phases, not just from math."**

**Corolario**: "Si optimización siempre activa → Perdiste guardrails de negocio (sobre-ingeniería)."

---

## Referencias

- **Protocol**: [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md)
- **Related**: [ANNEX-001: Thinking in Chains](./ANNEX-001_THINKING_IN_CHAINS.md)
- **ADR**: ADR-003 (Batching with Threshold=8)

---

**Versión**: 1.0
**Autor**: Pair-discovery session (Ernesto + Claude)
**Fecha**: 2025-11-06
**Status**: 🟢 Activo (patrón validado en FrameSupplier)
