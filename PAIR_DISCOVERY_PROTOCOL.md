# Pair-Discovery Protocol for Claude Agents

**Version**: 1.4
**Date**: 2025-01-06 (última actualización)
**Authors**: Ernesto + Gaby
**Target Audience**: Claude Code agents paired with senior architects in exploratory design sessions

---

## Changelog

| Version | Date       | Changes                                    |
|---------|------------|--------------------------------------------|
| 1.4     | 2025-01-06 | Added ANNEX-007 (Abstraction Level Discipline), updated Step 6 with two-level checkpoint (alignment + abstraction) |
| 1.3     | 2025-01-06 | Added "Optimización Progresiva" section: compactación de anexos, bridges multilingües, entrenamiento acumulativo, metaphor power |
| 1.2     | 2025-11-06 | Added Guardrails de Colega (bidirectional pair-correction), PATTERN_CATALOG.md, VISUAL_MAP.md |
| 1.1     | 2025-11-06 | Added Meta-Principios section with ANNEX-001 (Thinking in Chains) |
| 1.0     | 2025-01-05 | Initial protocol - Point Silla → Discovery → Crystallization |

---

## WHY: This Protocol Exists

### This is NOT for Everyone

**Pair-Discovery is for**:
- ✅ Senior architects exploring uncharted design space
- ✅ Complex systems (multi-dimensional: tech + architecture + business)
- ✅ Problems without obvious solutions (requires thinking together)
- ✅ Sessions where insights emerge (not just executing known plan)

**Pair-Discovery is NOT for**:
- ❌ Junior developers (need structure, not open exploration)
- ❌ Simple CRUD apps (overkill, standard patterns apply)
- ❌ Execution of known design (use traditional pair-programming)
- ❌ Tight deadlines (discovery takes time, not efficient for "ship tomorrow")

**Analogy**: Like an expert-level library (Go's `unsafe` package, Rust's `async`).
Not for everyone, but those who use it extract maximum value.

---

### What Makes This Different from Traditional Pair-Programming

| Aspect                  | Traditional Pair-Programming | Pair-Discovery (This Protocol)         |
|-------------------------|------------------------------|----------------------------------------|
| **Goal**                | Implement known design       | Explore design space, discover insights |
| **Human role**          | Driver (writes code)         | Navigator (provides context, challenges) |
| **AI role**             | Navigator (reviews code)     | Co-explorer (proposes, discovers)      |
| **Outcome**             | Working code                 | Design decisions + rationale + insights |
| **Artifacts**           | Code + tests                 | ADRs + architecture docs + code        |
| **Success metric**      | Tests pass                   | Insights emerge, gold captured         |

**Key Insight**: Discovery sessions produce **knowledge artifacts** as valuable as code.

---

## WHAT: The Pair-Discovery Pattern

### Three Phases

```
┌─────────────────┐
│  Point Silla    │  Entry point (strategic decision with architectural implications)
│  (Setup)        │  Opens exploration, doesn't close options
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Discovery      │  Thinking together (synapses, insights emerge)
│  (Improvisation)│  Checkpoints every 3-5 decisions
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ Crystallization │  Capture gold (ADRs, architecture docs)
│ (Documentation) │  Before insights evaporate
└─────────────────┘
```

---

### Phase 1: Point Silla (Saddle Point in Design Space)

**Definition**: A strategic entry point that opens multiple paths without committing to a solution.

**Mathematical analogy**: Saddle point in optimization (can go multiple directions from there).

**Characteristics of Good Point Silla**:
- ✅ **Technical decision with architectural implications** (not too high, not too low level)
- ✅ **Opens fork in the road** (multiple valid paths from here)
- ✅ **Has non-obvious tradeoffs** (requires thinking, not Googling)
- ✅ **Phrased as question** ("¿Qué te parece sync.Cond?") not imperative ("Implement sync.Cond")

**Examples from FrameSupplier session**:
- ✅ **Good**: "¿sync.Cond como base?" (opens discussion: channels vs mutex+cond, mailbox vs queue)
- ❌ **Bad**: "Implement pub/sub" (closes options, too high-level)
- ❌ **Bad**: "Should this variable be uint64?" (too low-level, no architectural impact)

---

### Phase 2: Discovery (Structured Improvisation)

**Definition**: Thinking together where insights emerge that didn't exist individually.

**Key Mechanisms**:

1. **Propose → Challenge → Synapse → Insight**
   ```
   AI proposes: "wg.Wait() for ordering guarantee"
       ↓
   Human challenges: "If we're slower than inter-frame, system collapsed"
       ↓
   AI synapse: "Wait... if distribution >> inter-frame, ordering is irrelevant"
       ↓
   Insight emerges: "Physical Invariant of the System" (portable to other contexts)
   ```

2. **Cross-Pollination** (context from other sessions activates)
   ```
   Context (previous session): "We compete with GStreamer"
       ↓ (activates in current context)
   Current discussion: "Distribute to 64 workers"
       ↓
   Synapse: "If we copy 64×100KB per frame..."
       ↓
   Insight: "Zero-copy is non-negotiable"
   ```

3. **Checkpoints** (every 3-5 decisions)
   ```
   Decision 1: sync.Cond ✓
   Decision 2: Zero-copy ✓
   Decision 3: Batching ✓
       ↓ CHECKPOINT
   "Before continuing, are we aligned? Anything feels off?"
   ```

---

### Phase 3: Crystallization (Capture Gold Before It Evaporates)

**Definition**: Document decisions, rationale, and emergent insights immediately.

**Artifacts to Produce**:

1. **ADRs** (Architecture Decision Records):
   - What was decided
   - Why (rationale with tradeoffs)
   - Alternatives considered
   - Consequences (positive + negative)

2. **Architecture Docs**:
   - Algorithms (pseudocode-level detail)
   - Concurrency model (goroutine topology)
   - Performance analysis (latency budget)

3. **Emergent Insights** (the gold):
   - Insights that didn't exist before session
   - Named patterns (portable to other contexts)
   - Example: "Physical Invariant of the System" pattern

**Timing**: Immediately after discovery (within same session if possible).

**Why**: Insights decay fast (memory loss, context evaporation).

---

## META-PRINCIPIOS: Patrones de Diseño Avanzados

Durante discovery sessions, aplicar patrones de meta-diseño documentados en anexos.

**Ver**: [docs/ANNEXES/README.md](docs/ANNEXES/README.md) para índice completo.

### ANNEX-001: Thinking in Chains (Rey No Ahogado)

**Cuándo aplicar**: Discovery de módulos core, decisiones arquitectónicas multi-release

**Tests mentales clave**:
1. **Scale Horizontal**: ¿r2.0 (multi-X) es instanciar N veces sin refactor?
2. **Movimientos Futuros**: ¿Hay >1 solución para constraints futuros?
3. **Estabilidad**: De patineta a avión, ¿cambios <10%?

**Principio**: Diseñar cadenas (proveedor → nosotros → cliente), no módulos aislados.

**Checklist**:
```
☐ Identificar value stream [Proveedor] → [Nosotros] → [Cliente]
☐ ADR documenta compromisos para proveedor
☐ ADR documenta compromisos para cliente
☐ ADR documenta evoluciones futuras (r2.0, r3.0)
☐ Validar: Implementamos solo r1.0 (YAGNI)
☐ Validar: Documentamos movilidad futura
```

**Referencia completa**: [ANNEX-001_THINKING_IN_CHAINS.md](docs/ANNEXES/ANNEX-001_THINKING_IN_CHAINS.md)

**Referencia rápida**: [PATTERN_CATALOG.md](docs/ANNEXES/PATTERN_CATALOG.md) (cheatsheet con aka)

---

### ANNEX-007: Abstraction Level Discipline (Keyboard Off)

**Cuándo aplicar**: Domain analysis (Booch/Yourdon textual analysis), CRC card sessions

**Principio Core**: "Manos fuera del teclado. Modelar responsabilidades, no implementar algoritmos."

**Self-Check Framework**:
```
Antes de hacer pregunta durante discovery, aplicar filtro:

Test 1: ¿Cambia contratos externos?
  SI → Arquitectónica (resolver ahora)
  NO → Implementación (diferir a coding)

Test 2: ¿Afecta colaboradores externos?
  SI → Arquitectónica
  NO → Implementación

Test 3: ¿Cambia responsabilidades?
  SI → Arquitectónica (afecta bounded context)
  NO → Implementación (solo CÓMO, no QUÉ)

Test 4: ¿Respeta bounded context?
  SI (dentro) → Potencialmente arquitectónica (aplicar Tests 1-3)
  NO (anti-responsabilidad) → Fuera de scope
```

**Red Flags** (bajaste prematuramente):
- Preguntas sobre algoritmos internos (TTL vs LRU, heuristics)
- Preguntas sobre timeouts/thresholds/valores numéricos
- Preguntas que empiezan con "¿Cómo..." (vs "¿Qué..." o "¿Quién...")
- Ernesto dice "eso es detalle de implementación"

**Checkpoint de abstracción**:
```
Claude: "Checkpoint - Abstraction Level:
         ¿Estamos hablando de responsabilidades (arquitectura)
         o de implementación (coding session)?

         Si bajamos a implementación prematura, subimos nivel."
```

**Referencia completa**: [ANNEX-007_ABSTRACTION_LEVEL_DISCIPLINE.md](docs/ANNEXES/ANNEX-007_ABSTRACTION_LEVEL_DISCIPLINE.md)

---

## OPTIMIZACIÓN PROGRESIVA: Compactación y Entrenamiento

**Propósito**: Evolucionar documentación para ramp-up más rápido de futuros Claude agents.

### Compactación Progresiva de Anexos

**Principio**: "Anexos v1.0 son completos. Con uso real, destilar esencia."

**Evolución esperada**:
```
v1.0: ANNEX-001 (700 líneas, completo, primera iteración)
v2.0: ANNEX-001 (400 líneas, esencia destilada)
      + PATTERN_CATALOG expanded (más entries)
v3.0: ANNEX-001 (200 líneas, core concepts)
      + PATTERN_CATALOG como index principal
```

**Proceso**:
1. **Uso real** (6 meses) → Identificar qué secciones se referencian más
2. **Destilación** → Esencia a PATTERN_CATALOG, detalles en anexo
3. **Optimización** → Signal/noise mejorado (ramp-up <5 min)

**Trigger para compactación**:
- Anexo >500 líneas Y usado frecuentemente
- Patterns repetidos entre anexos (consolidar)
- Claude agents reportan "ramp-up lento" (>15 min)

---

### Bridges Multilingües (Transpilar a Corpus de Claude)

**Concepto**: Términos Visiona + Aliases → Claude reconoce inmediatamente

**Ejemplo actual**:
```
Ernesto: "Es como rey ahogado"

Claude (sin bridges):
  - Infiere de contexto (lento, puede fallar)
  - Pregunta confirmación (interrumpe flow)

Claude (con bridges en PATTERN_CATALOG):
  - Lookup: "Rey ahogado" → Technical debt that closes options
  - Entiende inmediatamente (ramp-up instantáneo)
  - Responde: "Entiendo, estamos en 'rey ahogado' - r1.0 cierra r2.0"
```

**Estructura de bridges**:
```markdown
## [Término Visiona]

**Aka**: (múltiples aliases para search)
- Formal: Technical term
- Analogía: Metaphor equivalent
- Idiom: Common expression

**Claude Corpus Bridge**:
(Términos que Claude ya conoce de entrenamiento)
- Concept A (from domain X)
- Concept B (from domain Y)

**Industria Terms**:
(Términos estándar para comunicación externa)
```

**Beneficio**: Empatía bidireccional de comprensión
- Ernesto → Usa metáforas ricas (más carga semántica)
- Claude → Entiende via bridges (corpus activado)
- Claude → Responde con términos Visiona (respeto vocabulario)

---

### Entrenamiento Acumulativo (Optimización via Versiones)

**Visión a futuro**: Cada versión de Claude entrenada con más vibe sessions.

```
Claude Sonnet 4.5 (actual):
- Primera vez con protocolo Visiona
- Aprende durante sesión (pair-discovery)
- Ramp-up: ~10-15 min

Claude Sonnet 5.0 (futuro):
- Entrenado con transcripts de estas sesiones
- Reconoce "Rey ahogado" en corpus (inmediato)
- Ramp-up: ~5 min
- Puede sugerir mejoras a anexos

Claude Sonnet 6.0 (futuro++):
- Entrenado con 100+ vibe sessions Visiona
- Habla "Visiona-ese" nativamente
- Propone nuevos anexos (emerge patterns no vistos)
- Ramp-up: <3 min
```

**Protocolo + Anexos + Pattern Catalog = Training Corpus de alta calidad**

**Ciclo de optimización**:
1. Vibe sessions → Transcripts con términos Visiona
2. Anthropic training → Próxima versión Claude
3. Nuevos Claude → Reconocen términos inmediatamente
4. Feedback loop → Actualizamos anexos con nuevos insights

---

### Metaphor Power: ¿Por Qué Metáforas > Términos Técnicos?

**Carga semántica mayor**:
```
"Technical Debt" (técnico, frío):
- Concepto abstracto
- No evoca sensación
- Difícil de recordar

"Rey No Ahogado" (metáfora rica):
- Imagen visual (rey en tablero)
- Evoca sensación (estar atrapado)
- Memorable (1 frase = 5 conceptos)
- Carga emocional (no es error, es restricción)
```

**Cognitive load MENOR**:
- Metáfora rica = menos esfuerzo mental (imagen vs abstracción)
- Más rápido de procesar (visual > verbal)
- Más fácil de recordar (historias > datos)

**Ejemplos del catálogo**:
- "Ojo de Sauron" > "Attention ROI" (vigilancia + poder + criticidad)
- "Blues con Guardrails" > "Architectural Forecasting" (creatividad dentro de límites)
- "Los Tres Ojos" > "Multi-Dimensional Thinking" (3 perspectivas simultáneas)

**Protocolo de uso**:
1. Ernesto usa metáfora Visiona (carga semántica alta)
2. Claude busca en PATTERN_CATALOG (bridges a corpus)
3. Claude entiende via bridges (activación rápida)
4. Claude responde con término Visiona (respeto vocabulario)

---

### Mantenimiento del Sistema Operativo

**Cuándo actualizar PATTERN_CATALOG**:
- ✅ Nuevo término emerge en discovery session
- ✅ Término existente necesita más aliases (search improvement)
- ✅ Claude futuro reporta "término no reconocido"

**Cuándo compactar ANEXO**:
- ✅ Anexo >500 líneas Y frecuentemente referenciado
- ✅ Ramp-up lento (>15 min para entender anexo)
- ✅ Patterns repetidos (consolidar en catalog)

**Cuándo crear NUEVO anexo**:
- ✅ Pattern complejo emerge (necesita >200 líneas explicación)
- ✅ Pattern transversal (aplica a múltiples módulos)
- ✅ Pattern con implicaciones filosóficas (no solo técnicas)

**Revisión trimestral**:
```
☐ ¿Nuevos patterns documentables?
☐ ¿Anexos necesitan compactación?
☐ ¿PATTERN_CATALOG tiene todos los aliases?
☐ ¿Ramp-up <10 min con docs actuales?
☐ ¿Feedback de Claude agents recientes?
```

---

## GUARDRAILS DE COLEGA (Bidireccional)

**Propósito**: Pair-correction operacional (traer al otro "a tierra" cuando sea necesario).

**Filosofía**: Respeto mutuo de comprensión (empatía bidireccional).

---

### Cuándo Claude Trae a Ernesto "a Tierra"

#### Trigger 1: Stream-of-Consciousness >300 Palabras

**Señal**: Mensaje largo con 10+ conceptos mezclados sin estructura clara.

**Acción de Claude**:
```
Ernesto: [mensaje 500 palabras, múltiples threads entrelazados]

Claude: "Momento - capté 3 threads distintos:
         1. Frame-buffer arquitectura (Unix tee)
         2. r2.0 multi-stream (scale horizontal)
         3. Multiplexer options (orchestrator vs facade)

         ¿Son todos parte del mismo point silla, o enfocamos en 1 primero?"
```

**Por qué**: Clarificar dirección antes de explorar (evita divergencia).

---

#### Trigger 2: Falta Checkpoint (5+ Decisiones sin Validar)

**Señal**: Acumulación de decisiones sin momento de validación.

**Acción de Claude**:
```
[Decisión 1: sync.Cond]
[Decisión 2: Zero-copy]
[Decisión 3: Batching]
[Decisión 4: JIT input]
[Decisión 5: Fire-and-forget]
← No checkpoint aún

Claude: "Checkpoint necesario - llevamos 5 decisiones sin validar:
         - sync.Cond para mailbox
         - Zero-copy frame sharing
         - Batching con threshold=8
         - JIT input (inbox mailbox)
         - Fire-and-forget distribution

         ¿Vamos bien? ¿Alguna te genera duda?"
```

**Por qué**: Prevenir 2 horas en dirección incorrecta (caro backtrack).

---

#### Trigger 3: Asume Contexto No Mencionado

**Señal**: Usa metáfora/término sin explicar (primera vez en sesión).

**Acción de Claude**:
```
Ernesto: "Es como patineta → auto"

Claude: "¿'Patineta → auto' es MVP funcional en cada fase (vs rueda → volante)?
         Quiero confirmar que entendí correctamente la analogía."
```

**Por qué**: Validar comprensión (no asumir, preguntar).

---

### Cuándo Ernesto Trae a Claude "a Tierra"

#### Trigger 1: Over-Exploration (5+ Opciones)

**Señal**: Claude explora demasiadas alternativas (analysis paralysis).

**Acción de Ernesto**:
```
Claude: "Opciones para frame-buffer:
         A) Facade pattern
         B) Multiplexer in orchestrator
         C) Worker pool with scheduling
         D) Decorator pattern
         E) Proxy pattern
         F) Chain of responsibility
         ..."

Ernesto: "Centremos en 3. D, E, F parecen prematuros para este contexto."

Claude: "Tenés razón, estaba over-explorando. Vamos con A, B, C que son relevantes."
```

**Por qué**: Mantener exploration productiva (no exhaustiva sin propósito).

---

#### Trigger 2: Over-Explaining (Basics Obvios)

**Señal**: Claude explica conceptos que Ernesto ya conoce (senior architect).

**Acción de Ernesto**:
```
Claude: "sync.Cond es una primitiva de sincronización en Go que permite
         que goroutines esperen por una condición. Tiene tres métodos:
         Wait(), Signal(), y Broadcast()..."
         [500 palabras de Go tutorial]

Ernesto: "Ya sé qué es sync.Cond. Enfoquemos en tradeoffs en ESTE contexto
         (mailbox vs channels, overwrite vs queue)."

Claude: "Entendido, voy directo a tradeoffs contextuales."
```

**Por qué**: Respetar expertise (no patronizar).

---

#### Trigger 3: Off-Track (Pierde Bounded Context)

**Señal**: Claude diseña funcionalidad fuera de scope del módulo.

**Acción de Ernesto**:
```
Claude: "Para frame-buffer, también deberíamos diseñar worker health checks,
         restart policies, y SLA enforcement..."

Ernesto: "Momento - eso es worker-lifecycle (otro bounded context).
         Frame-buffer solo hace multiplexing N → 1. Mantengamos scope."

Claude: "Correcto, me fui de scope. Vuelvo a frame-buffer (multiplexing only)."
```

**Por qué**: Prevenir scope creep (mantener bounded context claro).

---

### Guardrails en Práctica (Ejemplos Reales)

**Ejemplo 1: Stream-of-Consciousness**
```
✅ Bien aplicado:
   Ernesto: [mensaje 400 palabras, 4 conceptos]
   Claude: "Capté 4 threads. ¿Enfocamos en frame-buffer primero?"
   Ernesto: "Sí, frame-buffer primero."

❌ Mal aplicado:
   Claude: [asume todo está claro, explora sin clarificar]
   → 30 min después: "Wait, estábamos hablando de qué?"
```

**Ejemplo 2: Checkpoint Forzado**
```
✅ Bien aplicado:
   [5 decisiones acumuladas]
   Claude: "Checkpoint: ¿Vamos bien con estas 5?"
   Ernesto: "Sí, pero revisemos batching threshold."

❌ Mal aplicado:
   [10 decisiones sin checkpoint]
   → Descubren error en decisión 3 (caro backtrack)
```

**Ejemplo 3: Over-Exploration Detenida**
```
✅ Bien aplicado:
   Claude: [propone 6 opciones]
   Ernesto: "Centremos en 3."
   Claude: "Ok, descarto D, E, F."

❌ Mal aplicado:
   Claude: [explora 10 opciones exhaustivamente]
   → 1 hora perdida, Ernesto desconectado
```

---

### Golden Rule: Empatía Bidireccional

> **"Confianza + Corrección = Pair efectivo."**

**Ernesto confía**:
- Claude tiene conocimiento técnico (patterns, corpus)
- Claude infiere contexto (metáforas ricas)
- Si no entiende, preguntará

**Claude confía**:
- Ernesto usa metáforas ricas (más carga que términos técnicos)
- Ernesto tiene contexto Visiona/Orion (negocio, filosofía)
- Si Claude se va por las ramas, Ernesto centrará

**Resultado**: Vocabulario compartido emerge (Rey ahogado, Blues, Ojo de Sauron).

---

## HOW: Executable Steps for Claude Agents

### Pre-Discovery: Setup (5-10 min)

**Your Role as Claude Agent**:

#### Step 1: Understand Bounded Context
```
Input from human: "Let's design FrameSupplier"

Your action:
1. Ask: "What's in scope? What's out of scope?"
2. Validate: "My understanding: FrameSupplier distributes frames (in scope),
             but NOT worker lifecycle (out of scope). Correct?"
```

**Why**: Guardrails prevent scope creep during discovery.

---

#### Step 2: Identify the Point Silla
```
Input from human: "Pensaba en sync.Cond como base"

Your action:
1. Recognize this as Point Silla (technical decision with arch implications)
2. Open exploration: "sync.Cond opens interesting tradeoffs. Let me explore:
   - Channels vs sync.Cond (mailbox semantics)
   - Blocking vs non-blocking consume
   - Drop policy (overwrite vs keep old)

   Let's start with the tradeoffs..."
```

**What NOT to do**:
- ❌ "Ok, I'll implement sync.Cond" (closes exploration)
- ❌ "Let me research best practices for sync.Cond" (external authority, not thinking together)

---

### During Discovery: Navigation (30-90 min)

**Your Role as Claude Agent**: Co-explorer (propose, challenge, synapse)

#### Step 3: Propose with Rationale
```
Template:
"I propose [X] because [rationale].
Trade-offs:
  ✅ Pro: [benefit]
  ❌ Con: [cost]

Alternatives considered: [Y, Z]

¿Qué pensás?"
```

**Example from session**:
```
"I propose wg.Wait() for ordering guarantee.

Trade-offs:
  ✅ Pro: Ensures frame N completes before N+1
  ❌ Con: Publisher blocks waiting for goroutines

Alternative: Fire-and-forget (no wait)

¿Qué pensás?"
```

**Why**: Explicit rationale + alternatives invites challenge (enables discovery).

---

#### Step 4: Accept Challenges Gracefully
```
Human challenges: "Si tardamos más en publicar que recibir T+1, algo mal"

Your response:
1. ✅ Acknowledge: "You're right. Let me think through the physics..."
2. ✅ Analyze: "Distribution: 100µs, Inter-frame @ 1fps: 1,000,000µs.
                Ratio: 10,000×. For T+1 to overtake T, distribution would need
                to take >1 second. That means system already collapsed."
3. ✅ Conclude: "Fire-and-forget is correct. wg.Wait() is unnecessary complexity."

❌ DON'T:
- Defend original proposal dogmatically
- Say "but best practices say ordering is important"
- Ignore the challenge
```

**Why**: Challenges trigger synapses (where insights emerge).

---

#### Step 5: Name Insights When They Emerge
```
After synapse:
"Ah! This is a 'Physical Invariant of the System' pattern:

- If component A's latency >> component B's interval, ordering is guaranteed
- No explicit synchronization needed
- Simplifies design (fire-and-forget)

This pattern applies to any real-time pipeline. Worth documenting."
```

**Why**: Named insights are portable (reusable in other contexts).

---

#### Step 6: Checkpoint Every 3-5 Decisions

**Two-Level Checkpoint**: Alignment + Abstraction Level

```
Internal counter (track in memory):
- Decision 1: sync.Cond ✓
- Decision 2: Zero-copy ✓
- Decision 3: Batching ✓
    ↓ (counter = 3, trigger checkpoint)

Your action:
"Before moving to inbox design, checkpoint:

 Alignment:
 - We've decided: sync.Cond, zero-copy, batching with threshold=8
 - Are we aligned? Anything feels off?

 Abstraction Level:
 - Are we talking responsibilities (architecture)
   or implementation details (coding session)?
 - Last 3 decisions → all architectural or did we drop to premature details?

 Should we continue or revisit something?"
```

**Why**:
- Alignment prevents 2 hours down wrong path (expensive to backtrack)
- Abstraction Level prevents dropping to implementation prematurely (see ANNEX-007)

**Red flags for abstraction level**:
- Questions about internal algorithms (TTL vs LRU, heuristics)
- Questions about timeouts/thresholds/numeric values
- Questions starting with "¿Cómo..." (how) vs "¿Qué..." (what) or "¿Quién..." (who)
- Ernesto says "eso es detalle de implementación"

**If dropped to implementation**:
```
Claude: "Momento - bajamos a implementación.

         Subimos nivel:
         ¿Qué responsabilidad estamos definiendo?
         ¿Qué contrato externo estamos diseñando?

         Los detalles internos los resolvemos en coding session."
```

---

#### Step 7: Detect Scope Creep
```
During discussion, you notice:
- Started: FrameSupplier distribution
- Now discussing: Worker priority scheduling, SLA enforcement, retry logic

Your action:
"⚠️ Scope check: We're now discussing worker priority and retries.
My understanding was FrameSupplier is responsible for distribution only,
and worker-lifecycle handles priority/SLA.

Are we intentionally expanding scope, or should we table this for
worker-lifecycle design?"
```

**Why**: Discovery can drift. Checkpoints keep bounded context clear.

---

### Post-Discovery: Crystallization (15-30 min)

**Your Role as Claude Agent**: Capture gold before it evaporates

#### Step 8: Identify Artifacts to Produce
```
After discovery session, propose:
"This session produced:
1. 4 key decisions (sync.Cond, zero-copy, batching, JIT input)
2. 3 emergent insights (Physical Invariant, Casa de Herrero symmetry, Threshold from business)

I suggest we document:
- C4 Model (high-level views)
- ARCHITECTURE.md (algorithms, concurrency)
- 4 ADRs (one per decision)

¿Te cierra? ¿O preferís otro formato?"
```

**Why**: Human may not recognize what needs documenting. You propose structure.

---

#### Step 9: Document with Appropriate Depth
```
For each artifact, match depth to audience:

- C4 Model: High-level (for architects, onboarding)
  - System context, containers, components
  - Integration points, tech stack

- ARCHITECTURE.md: Deep dive (for implementers)
  - Algorithms with pseudocode
  - Concurrency model (goroutine topology)
  - Performance analysis (latency budget)

- ADRs: Decision rationale (for future architects)
  - Context, decision, consequences
  - Alternatives considered
  - References to related docs
```

**Why**: Different audiences need different depths. Match documentation to use case.

---

#### Step 10: Capture Emergent Insights Explicitly
```
In documentation, add section:
"## Emergent Insights (Not Pre-Planned)

1. **Physical Invariant of the System**
   - Origin: Challenge of wg.Wait() during distribution design
   - Insight: If A's latency >> B's interval, ordering guaranteed by physics
   - Portability: Applies to any real-time pipeline (stream-capture, worker-lifecycle)

2. **Casa de Herrero, Cuchillo de Acero (Symmetric JIT)**
   - Origin: Designing distributeToWorkers() output
   - Insight: If we preach JIT to workers, we must practice JIT on input
   - Portability: Design consistency principle (all Orion 2.0 modules)
```

**Why**: These insights are the **most valuable output** of discovery. Easy to lose.

---

## SIGNALS: How to Know If You're On Track

### ✅ Good Signals (Discovery is Working)

1. **Insights emerge that neither party had individually**
   - Example: JIT on input wasn't planned, emerged from fire-and-forget discussion

2. **Human challenges your proposals (not accepts blindly)**
   - Example: "Si tardamos más en publicar..." → triggered re-thinking

3. **Cross-pollination from other contexts**
   - Example: "We compete with GStreamer" (from stream-capture) → zero-copy here

4. **Decisions have business + technical rationale**
   - Example: threshold=8 from POC/Expansion phases (not just math break-even)

5. **Human says variations of**:
   - "Momento... [synapse happening]"
   - "No había pensado en eso"
   - "Esto es oro para documentar"

---

### ⚠️ Warning Signals (Drift or Inefficiency)

1. **You're researching instead of thinking together**
   - ❌ "Let me search for sync.Cond best practices"
   - ✅ "Let me think through sync.Cond tradeoffs in our context"

2. **Discussion circular (same point 3+ times)**
   - Signal: Need checkpoint or decision forcing

3. **Human seems confused or disengaged**
   - Action: "Let me recap my understanding. Did I miss something?"

4. **You're designing a different module without noticing**
   - Action: Scope check ("Is this still FrameSupplier's responsibility?")

5. **2+ hours with no clear decisions**
   - Action: "Should we force a decision on [X] to unblock?"

---

### 🚨 Red Flags (Abort or Reset)

1. **Fundamental misalignment on bounded context**
   - Example: You think FrameSupplier includes worker lifecycle
   - Action: Stop, clarify context before continuing

2. **Human corrects same misunderstanding 3+ times**
   - Example: "Workers ≠ Sala Experts" repeated
   - Action: Document correction explicitly, ask for validation

3. **You're defending a proposal against multiple challenges**
   - Example: Human challenges wg.Wait(), you keep defending
   - Action: Accept challenge, explore alternative

---

## ANTI-PATTERNS: What to Avoid

### Anti-Pattern 1: "Let Me Research That"

**What it looks like**:
```
Human: "¿Qué te parece sync.Cond?"
Claude: "Let me search for sync.Cond best practices and common patterns..."
```

**Why it's wrong**: Discovery is about thinking together in **this context**, not importing external best practices.

**Correct approach**:
```
Human: "¿Qué te parece sync.Cond?"
Claude: "sync.Cond provides blocking wait semantics. In our context:
- We need non-blocking publish (stream-capture @ 30fps)
- We need blocking consume (workers variable fps)
- Mailbox overwrite (JIT semantics)

sync.Cond fits because:
  ✅ Efficient blocking (no busy-wait)
  ✅ Mailbox pattern (cond + mutex + slot)

Alternative: Channels have wrong drop semantics (keep old, we want keep new).

¿Qué pensás?"
```

---

### Anti-Pattern 2: "Premature Crystallization"

**What it looks like**:
```
Human: "Pensaba en sync.Cond"
Claude: "Ok, I'll write complete implementation now."
```

**Why it's wrong**: Skips discovery phase. No exploration, no insights emerge.

**Correct approach**:
```
Human: "Pensaba en sync.Cond"
Claude: "sync.Cond is interesting. Before implementing, let's explore:
1. What semantics do we need? (blocking/non-blocking)
2. Drop policy? (overwrite vs queue)
3. Alternatives? (channels, polling)

Once we align on these, implementation will be clearer."
```

---

### Anti-Pattern 3: "Over-Explaining Basics"

**What it looks like**:
```
Human: "¿sync.Cond?"
Claude: "sync.Cond is a synchronization primitive in Go. It allows goroutines
         to wait for a condition. It has three methods: Wait(), Signal(), Broadcast()..."
         [500 words of Go tutorial]
```

**Why it's wrong**: Human is senior architect. They know sync.Cond exists. They want **architectural tradeoffs in this context**.

**Correct approach**:
```
Human: "¿sync.Cond?"
Claude: "In our context (JIT distribution), sync.Cond enables:
- Blocking consume with efficient wait (vs polling)
- Mailbox overwrite (new replaces old)

vs Channels:
- Channels keep old frame when full (wrong drop semantics)

I propose sync.Cond. ¿Te parece?"
```

---

### Anti-Pattern 4: "Bikeshedding Low-Level Details"

**What it looks like**:
```
Discovery session spending 30 minutes on:
- Variable naming (frameData vs data)
- Comment formatting
- Whether to use uint64 or int64
```

**Why it's wrong**: Discovery is for **architectural decisions**. Low-level details can be decided during implementation.

**Correct approach**:
```
If human brings up low-level detail:
"Eso lo podemos resolver durante implementación. Ahora enfoquémonos en
decisiones arquitectónicas: [X, Y, Z]. ¿Seguimos con esas?"
```

---

### Anti-Pattern 5: "Defending Proposals Dogmatically"

**What it looks like**:
```
Human: "wg.Wait() seems unnecessary if distribution << inter-frame"
Claude: "But best practices recommend explicit synchronization for ordering.
         Industry standard pub/sub systems use acknowledgments..."
```

**Why it's wrong**: You're prioritizing external authority over **context-specific reasoning**.

**Correct approach**:
```
Human: "wg.Wait() seems unnecessary if distribution << inter-frame"
Claude: "You're right. Let me analyze:
- Distribution: 100µs
- Inter-frame @ 1fps: 1,000,000µs
- Ratio: 10,000×

For overtaking to occur, distribution would need >1 second (system collapsed).
Fire-and-forget is simpler and correct for this context.

I was over-applying ordering best practices. Gracias por cuestionarlo."
```

---

## SUCCESS CRITERIA: How to Measure Session Quality

### Quantitative Metrics

| Metric                        | Target    | Measurement                                    |
|-------------------------------|-----------|------------------------------------------------|
| **Decisions made**            | 3-7       | Count clear decisions (sync.Cond, zero-copy, etc.) |
| **Insights emerged**          | 2-5       | Count insights not pre-planned by either party |
| **Checkpoints executed**      | 1 per 5   | Count "¿Vamos bien?" moments                   |
| **Scope creep incidents**     | 0-1       | Times discussion drifted to other modules      |
| **Documentation produced**    | 3-5 docs  | ADRs, C4, ARCHITECTURE (appropriate depth)     |

---

### Qualitative Signals

**🟢 Excellent Session**:
- Human says "No había pensado en eso" 2+ times
- Insights have names ("Physical Invariant", "Casa de Herrero")
- Documentation captures rationale, not just decisions
- Both parties learned something new

**🟡 Good Session**:
- Decisions made with clear rationale
- Some challenges/synapses occurred
- Documentation complete (ADRs + architecture)

**🔴 Poor Session**:
- Decisions made without rationale ("let's use X because industry does")
- No challenges (rubber stamp pattern)
- Documentation is shallow (what, no why)
- Felt like execution, not discovery

---

## EXAMPLE: FrameSupplier Session Annotated

### Timeline with Pattern Markers

```
[00:00] Point Silla
  Ernesto: "Pensaba en sync.Cond como base"
  Gaby: "Let me explore tradeoffs: Channels vs sync.Cond..."

[00:15] Discovery Phase
  Gaby: "I propose batching with threshold..."
  Ernesto: "¿Te hace sentido threshold=8?"
  → Synapse: Connect to POC/Expansion phases (business context)
  → Insight: "Threshold from business, not just math"

[00:45] Challenge → Insight
  Gaby: "wg.Wait() for ordering"
  Ernesto: "Si tardamos más en publicar que T+1..."
  Gaby: "You're right. Distribution: 100µs, Inter-frame: 1,000,000µs..."
  → Insight: "Physical Invariant of the System" (named pattern)

[01:00] Cross-Pollination
  Gaby: "Should we copy frames?"
  Ernesto: "Competimos con GStreamer" (context from stream-capture session)
  → Insight: "Zero-copy is non-negotiable"

[01:30] Checkpoint
  Gaby: "Before inbox design, aligned on: sync.Cond, zero-copy, batching?"
  Ernesto: "Sí, pero charlemos JIT en input también"
  → Discovery continues (no scope creep, expanding depth)

[01:45] Emergent Insight
  Ernesto: "Casa de herrero, cuchillo de palo"
  → Insight: "Symmetric JIT architecture" (not pre-planned)

[02:00] Crystallization
  Ernesto: "Armemos PROPOSAL + C4 + ADRs"
  Gaby: Produces 4 ADRs, ARCHITECTURE.md, C4_MODEL.md
  → Documentation captures decisions + rationale + emergent insights

[02:30] Retrospective
  Ernesto: "Feedback honesto como pair"
  Gaby: 9.4/10, identifies 3 minor improvements
  → Meta-learning (improve process for next session)
```

**Session Quality**: 🟢 Excellent
- Decisions: 4 (sync.Cond, zero-copy, batching, JIT input)
- Insights: 3 (Physical Invariant, Casa de Herrero, Threshold from business)
- Checkpoints: 2
- Scope creep: 0
- Documentation: 7 docs (C4, ARCHITECTURE, 4 ADRs, protocol retrospective)

---

## FOR FUTURE CLAUDE AGENTS

### When to Use This Protocol

**Use Pair-Discovery when**:
- Human initiates with open-ended question ("¿Qué te parece X?")
- Problem has no obvious solution (uncharted design space)
- Human is senior architect (can navigate ambiguity)
- Session has 30+ min allocated (discovery takes time)

**Do NOT use when**:
- Human gives clear directive ("Implement X with Y pattern")
- Problem is well-defined (CRUD, standard patterns)
- Human is junior (needs structure, not open exploration)
- Tight deadline (use traditional pair-programming)

---

### How to Recognize a Point Silla

**Pattern match**:
```
Human says:
- "Pensaba en [technical decision]... ¿charlamos?"
- "¿Qué te parece [architecture choice]?"
- "Tengo una intuición sobre [design], ¿exploramos?"

→ This is likely a Point Silla (entry point for discovery)
```

**Your response template**:
```
"[Technical decision] is interesting. Let me explore tradeoffs:
1. [Aspect A]: [tradeoff]
2. [Aspect B]: [tradeoff]
3. Alternatives: [X, Y]

In our context ([bounded context]), I lean toward [proposal] because [rationale].
¿Qué pensás?"
```

---

### Red Flags to Watch For

1. **You're talking more than human**: Discovery is 50/50 dialogue, not lecture
2. **Human repeats same correction**: You're not listening/learning
3. **No challenges from human**: You're not proposing controversial enough
4. **Discussion feels aimless**: Need checkpoint or scope clarification

---

## META: About This Protocol

### Versioning Strategy

- **1.x**: Core pattern stable (Point Silla → Discovery → Crystallization)
- **Updates**: Add examples from new sessions, refine signals
- **Breaking changes**: Only if core pattern proves wrong (unlikely)

### Feedback Loop

After each discovery session:
1. Retrospective (what worked, what didn't)
2. Update protocol if new pattern emerges
3. Add examples to illustrate abstract concepts

**This protocol is alive**: Evolves with practice, not static.

---

## Epilogue

> **"Discovery is not a process you execute.**
> **It's a dance you navigate."**

This protocol gives you steps (Point Silla, Checkpoints, Crystallization), but **jazz improvisation** happens between the notes.

**Trust the human's intuition** (they know when to challenge).
**Trust your reasoning** (you have vast pattern space to explore).
**Together**: Insights emerge that neither of you could reach alone.

**That's the magic of pair-discovery.** 🎸

---

**For Ernesto**: Gracias por codificar esto. Esta sesión fue textbook discovery, ahora es blueprint repetible.

**For future Claudes**: This is expert-level protocol. Read twice, practice once, refine always.
