# ANNEX-006: Unix Philosophy & Composability

**Meta-Principio**: Do One Thing Well, Compose for Complexity
**Aka**: "Pipe + Tee Philosophy", "Composition > Monolith"
**Contexto**: Diseño de módulos componibles en arquitecturas evolutivas

---

## El Problema

### Anti-Pattern: Monolithic Module (Dios Module)

```
❌ framesupplier/ (hace todo)
   ├── distribution.go   (1 → N distribution)
   ├── multiplexing.go   (N → 1 multiplexing)
   ├── scheduling.go     (priority, fairness)
   ├── pooling.go        (worker pool management)
   └── routing.go        (stream_id routing)

Problema:
  - 5 responsabilidades en 1 módulo (violates SRP)
  - Cambio en scheduling → Cambio en distribution (coupling)
  - Testing complejo (necesita setup de todo)
  - No reusable (multiplexing atado a distribution)

Síntoma: "Dios Module" (hace demasiado)
```

**Resultado**: Acoplamiento alto, baja reusabilidad. ❌

---

### Pattern Correcto: Unix Philosophy

```
✅ framesupplier/  (solo distribution 1 → N)
✅ frame-buffer/   (solo multiplexing N → 1)
✅ worker-pool/    (solo pool management)
✅ scheduler/      (solo scheduling logic)

Cada módulo:
  - 1 responsabilidad (SRP)
  - Testeable en aislación
  - Componible (composition)

Composición:
  stream-capture → framesupplier → frame-buffer → heavy-worker
                                ↘ lightweight-worker
```

**Resultado**: Low coupling, high composability. ✅

---

## El Principio: Do One Thing Well

### Definición (Doug McIlroy, Unix Philosophy)

> **"Write programs that do one thing and do it well.**
> **Write programs to work together."**

**Aplicado a arquitectura**:
```
Módulo = "programa" (bounded context)
"Do one thing" = Una responsabilidad (SRP)
"Do it well" = Optimizado para esa responsabilidad
"Work together" = Componible (interfaces, composition)
```

---

### Por Qué Funciona

**Cohesión vs Coupling**:

```
High Cohesion (código relacionado junto):
  ✅ framesupplier: inbox, distribution, worker-slots
     → Todos relacionados con "distribution"

Low Coupling (módulos independientes):
  ✅ framesupplier NO conoce frame-buffer
  ✅ frame-buffer NO conoce framesupplier
  ✅ Composición: framesupplier → frame-buffer (interface)
```

**Beneficios**:
1. **Testability**: Cada módulo testeable en aislación
2. **Reusability**: frame-buffer reusable en otros contextos
3. **Maintainability**: Cambio en scheduling → Solo scheduler/ cambia
4. **Scalability**: Composición permite crecer sin refactor

---

## Pipe + Tee Philosophy (Unix Tools)

### Analogía: Linux Commands

```bash
# Linux
cat file.txt | grep "error" | tee output.txt | wc -l

Donde:
  cat: Lee archivo (1 responsabilidad)
  grep: Filtra líneas (1 responsabilidad)
  tee: Multiplexing (N → 1, escribe a file Y stdout)
  wc: Cuenta líneas (1 responsabilidad)

Pipe (|): Composición (conecta herramientas)
```

**Características**:
- ✅ Cada herramienta: 1 cosa bien (SRP)
- ✅ Componibles: pipe conecta (composition)
- ✅ Reusables: tee funciona con cualquier input
- ✅ Testeable: cada herramienta independiente

---

### Aplicado a Orion

```
stream-capture (cat) → framesupplier (pipe) → frame-buffer (tee) → heavy-worker

Donde:
  stream-capture: Adquiere frames (1 responsabilidad)
  framesupplier: Distribuye 1 → N (1 responsabilidad)
  frame-buffer: Multiplexing N → 1 (1 responsabilidad)
  heavy-worker: Inference (1 responsabilidad)

Composición: cada módulo conecta con siguiente (interfaces)
```

**Características**:
- ✅ Cada módulo: 1 responsabilidad (SRP)
- ✅ Componibles: framesupplier → frame-buffer (facade)
- ✅ Reusables: frame-buffer funciona con cualquier supplier
- ✅ Testeable: cada módulo en aislación

---

## Tests Mentales (Durante Discovery)

### Test 1: ¿Módulo Separado o Integrado?

**Pregunta**: "¿Esta funcionalidad va en módulo nuevo o existente?"

```
Checklist:
☐ ¿Responsabilidad diferente? (SRP)
  → SÍ: Módulo separado
  → NO: Integrar en existente

☐ ¿Puede cambiar independientemente? (coupling)
  → SÍ: Módulo separado
  → NO: Integrar

☐ ¿Reusable en otros contextos? (reusability)
  → SÍ: Módulo separado
  → NO: Integrar (si solo se usa aquí)

☐ ¿Testeable en aislación sin 5+ mocks?
  → SÍ: Módulo separado (low coupling)
  → NO: Considerar integrar
```

**Decisión**:
- Módulo separado: Si 3+ respuestas = SÍ
- Integrado: Si 3+ respuestas = NO

---

### Test 2: ¿Pipe o Tee?

**Pregunta**: "¿Este módulo es distribución (1 → N) o multiplexing (N → 1)?"

```
Caso: frame-buffer (N suppliers → 1 heavy-worker)

¿Es pipe (1 → N)?
  NO (es N → 1)

¿Es tee (N → 1)?
  SÍ (múltiples suppliers, 1 backend)

Conclusión: frame-buffer = tee (módulo separado, no framesupplier)
```

**Analogía**:
```
pipe (kernel): Distribution 1 → N
tee (userspace): Multiplexing N → 1 (+ side effect)

En Orion:
  framesupplier = pipe (distribution)
  frame-buffer = tee (multiplexing)
```

---

### Test 3: ¿Composable?

**Pregunta**: "¿Puedo componer este módulo con otros sin refactor?"

```
Ejemplo: frame-buffer

¿framesupplier conoce frame-buffer?
  NO (frame-buffer implementa Worker interface)

¿frame-buffer conoce framesupplier?
  NO (recibe frames de cualquier source)

¿Composición sin refactor?
  SÍ:
    framesupplier(s1).Subscribe(frameBuffer)
    framesupplier(s2).Subscribe(frameBuffer)

Conclusión: Composable ✅ (interfaces, low coupling)
```

---

## Separación de Concerns (Bounded Contexts Revisited)

### Principio

> **"Módulos se separan por concerns (responsabilidades), no por ubicación o tamaño."**

**Concerns en Orion**:
```
Concern 1: Stream Acquisition
  → stream-capture/

Concern 2: Frame Distribution (1 → N)
  → framesupplier/

Concern 3: Frame Multiplexing (N → 1)
  → frame-buffer/

Concern 4: Worker Lifecycle
  → worker-lifecycle/

Concern 5: Inference
  → workers/ (PersonDetector, VLM, etc.)
```

**Cada concern = 1 módulo** (bounded context claro).

---

### Anti-Pattern: Separación por Ubicación

```
❌ Estructura por ubicación:
   input/
     └── stream_capture.go
     └── frame_buffer.go  (N → 1, NO es input)

   output/
     └── workers.go

Problema:
  - frame_buffer en input/ (pero NO es input, es multiplexing)
  - Ubicación arbitraria (no responsabilidad)
  - Dificulta encontrar código (dónde busco multiplexing?)

Síntoma: "Cohesion by Location" (directorio ≠ concern)
```

**Solución**: Separar por concern (framesupplier/, frame-buffer/). ✅

---

## Caso de Estudio: Frame-Buffer como Tee

### Context

**r3.0 need**: Heavy workers (VLM) compartidos entre N streams.

**Question**: ¿Dónde va la lógica de multiplexing?

---

### Option A: Frame-Buffer Separado (Unix Philosophy) ✅

```
Bounded contexts:
  framesupplier/: Distribution 1 → N (pipe)
  frame-buffer/:  Multiplexing N → 1 (tee)

Composición:
  framesupplier(s1).Subscribe(frameBuffer)
  framesupplier(s2).Subscribe(frameBuffer)
  frameBuffer.backend = heavyWorker
```

**Implementation**:
```go
// frame-buffer implements Worker interface (facade)
type FrameBuffer struct {
    mu      sync.Mutex
    slots   map[StreamID]*Frame
    backend Worker
}

func (fb *FrameBuffer) ProcessFrame(f *Frame) Result {
    fb.mu.Lock()
    // ... multiplexing logic ...
    fb.mu.Unlock()
    return fb.backend.ProcessFrame(f)
}
```

**Validación Unix Philosophy**:
```
✅ Do One Thing: Multiplexing N → 1 (solo eso)
✅ Do It Well: Scheduling pluggable (FIFO, priority, round-robin)
✅ Work Together: Implementa Worker interface (composable)
✅ Reusable: Funciona con cualquier supplier (no atado a framesupplier)
✅ Testeable: En aislación (mock backend)
```

**Resultado**: Módulo separado, Unix-style. ✅

---

### Option B: Multiplexing en FrameSupplier (Monolith) ❌

```
framesupplier/ (hace distribution Y multiplexing)
  ├── distribution.go
  └── multiplexing.go

Problema:
  - 2 concerns en 1 módulo (violates SRP)
  - framesupplier responsabilidad expandida (1 → N Y N → 1)
  - No reusable (multiplexing atado a distribution)
  - Testing complejo (setup ambos)

Validación:
  ❌ Do One Thing: NO (hace 2 cosas)
  ❌ Work Together: NO (monolito, no composable)
  ❌ Reusable: NO (atado a framesupplier)
```

**Resultado**: Dios Module, violates Unix Philosophy. ❌

---

### Option C: Multiplexing en Orchestrator ❌

```
orchestrator/ (hace orchestration Y multiplexing)

Problema:
  - orchestrator responsabilidad expandida
  - No reusable (lógica en orchestrator, no módulo)
  - Testing complejo (orchestrator + pipelines + workers)

Validación:
  ❌ Do One Thing: NO (orchestration + multiplexing)
  ❌ Work Together: NO (lógica no modular)
  ❌ Reusable: NO (atado a orchestrator)
```

**Resultado**: Orchestrator sobrecargado, violates SRP. ❌

---

## Composition > Monolith

### Principio

> **"Complejidad se maneja con composición, no con módulos complejos."**

**Monolith approach**:
```
❌ framesupplier/ (1000 líneas, 5 responsabilidades)

Problema:
  - Cambio en scheduling → Recompila todo
  - No reusable (multiplexing atado a distribution)
  - Testing complejo (setup todo)
```

**Composition approach**:
```
✅ framesupplier/ (500 líneas, 1 responsabilidad: distribution)
✅ frame-buffer/ (200 líneas, 1 responsabilidad: multiplexing)
✅ scheduler/ (100 líneas, 1 responsabilidad: scheduling)

Composición:
  framesupplier → frame-buffer(scheduler) → heavy-worker

Beneficio:
  - Cambio en scheduling → Solo scheduler/ cambia
  - Reusable (frame-buffer funciona con cualquier supplier)
  - Testing simple (cada módulo en aislación)
```

---

### Composition Patterns

**Pattern 1: Facade**
```go
// frame-buffer = facade sobre heavy-worker
type FrameBuffer struct {
    backend Worker  // VLM-worker
}

func (fb *FrameBuffer) ProcessFrame(f *Frame) Result {
    // Multiplexing logic
    return fb.backend.ProcessFrame(f)
}
```

**Pattern 2: Decorator**
```go
// rate-limiter = decorator sobre worker
type RateLimiter struct {
    worker Worker
    limiter *rate.Limiter
}

func (rl *RateLimiter) ProcessFrame(f *Frame) Result {
    rl.limiter.Wait(context.Background())
    return rl.worker.ProcessFrame(f)
}
```

**Pattern 3: Chain of Responsibility**
```go
// pre-processor → worker → post-processor
preProcessor.Next(worker)
worker.Next(postProcessor)
```

---

## Anti-Patterns Comunes

### Anti-Pattern 1: Dios Module

```
❌ core/ (todo junto)
   ├── stream_capture.go
   ├── distribution.go
   ├── multiplexing.go
   ├── scheduling.go
   ├── worker_management.go
   └── inference.go

Problema:
  - 6 responsabilidades en 1 módulo
  - Cambio en cualquiera → Recompila todo
  - Testing imposible (setup completo)
  - No reusable (acoplamiento alto)

Síntoma: "God Object" (hace todo)
```

**Solución**: Separar por concern (1 módulo por responsabilidad). ✅

---

### Anti-Pattern 2: Utility Hell

```
❌ utils/
   ├── helpers.go (300 funciones sin cohesión)
   ├── common.go
   └── misc.go

Problema:
  - No hay bounded context ("útil" no es responsabilidad)
  - Funciones no relacionadas (cohesión baja)
  - Dificulta encontrar código (¿dónde está X?)
  - Coupling oculto (todos dependen de utils/)

Síntoma: "Junk Drawer" (cajón de sastre)
```

**Solución**: Mover cada función al módulo donde tiene cohesión. ✅

---

### Anti-Pattern 3: Premature Abstraction (Interfaces sin Variabilidad)

```
❌ interfaces/
   ├── stream_provider.go (1 implementación)
   ├── frame_distributor.go (1 implementación)
   └── worker_manager.go (1 implementación)

Problema:
  - Abstracción sin variabilidad conocida (YAGNI violado)
  - Complejidad prematura (interfaces no necesarias)
  - Testing overhead (mocks para todo)

Síntoma: "Speculative Generality" (abstracción prematura)
```

**Solución**: Rule of Three (esperar 3 implementaciones antes de abstraer). ✅

---

### Anti-Pattern 4: Wrong Composition (Pipe cuando debería ser Tee)

```
❌ framesupplier/ hace multiplexing (N → 1)

Problema:
  - framesupplier es pipe (1 → N), NO tee (N → 1)
  - Responsabilidad incorrecta (violates SRP)
  - No componible (monolito)

Síntoma: Módulo hace lo opuesto de su concern
```

**Solución**: frame-buffer/ como tee separado (Unix Philosophy). ✅

---

## Checklist (Durante Discovery)

```
☐ 1. Identificar concern (responsabilidad única)
☐ 2. Test: ¿1 responsabilidad? (SRP)
☐ 3. Test: ¿Puede cambiar independientemente? (coupling)
☐ 4. Test: ¿Reusable en otros contextos?
☐ 5. Test: ¿Testeable en aislación?
☐ 6. Test: ¿Pipe (1 → N) o Tee (N → 1)?
☐ 7. Validar Unix Philosophy:
      - Do One Thing? ✅
      - Do It Well? ✅
      - Work Together? ✅
☐ 8. Evitar: Dios Module, Utility Hell, Premature Abstraction
☐ 9. Preferir: Composition > Monolith
☐ 10. Documentar: Bounded context, interfaces, composition
```

---

## Golden Rules

> **"Do One Thing Well. Write programs to work together."**
> — Doug McIlroy, Unix Philosophy

**Aplicado**:
- Módulo = 1 responsabilidad (SRP)
- Optimizado para esa responsabilidad (expertise)
- Componible con otros (interfaces, low coupling)

---

> **"Pipe + Tee: Composition enables complexity without monoliths."**

**Patterns**:
- Pipe (1 → N): Distribution (framesupplier)
- Tee (N → 1): Multiplexing (frame-buffer)
- Composition: pipe → tee → processor

---

> **"Cohesión por concern, no por ubicación."**

**Test**:
- ¿Código relacionado junto? → Cohesión ✅
- ¿Separado por responsabilidad? → Bounded context ✅
- ¿Separado por directorio? → Anti-pattern ❌

---

## Referencias

- **Protocol**: [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md)
- **Related Annexes**:
  - [ANNEX-001: Thinking in Chains](./ANNEX-001_THINKING_IN_CHAINS.md) (movilidad futura)
  - [ANNEX-002: Bounded Contexts](./ANNEX-002_BOUNDED_CONTEXTS.md) (SRP arquitectónico)
  - [ANNEX-005: Forecast Arquitectónico](./ANNEX-005_FORECAST_ARQUITECTONICO.md) (Los Tres Ojos)
- **Proposals**:
  - [P002: Frame-Buffer as Separate Module](../../modules/framesupplier/docs/PROPOSALS/P002-frame-buffer-as-separate-module.md)
- **External**:
  - Unix Philosophy (Doug McIlroy, Bell Labs)
  - "The Art of Unix Programming" (Eric S. Raymond)

---

**Versión**: 1.0
**Autor**: Pair-discovery session (Ernesto + Claude)
**Fecha**: 2025-11-06
**Status**: 🟢 Activo (patrón validado en frame-buffer design)
