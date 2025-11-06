# CLAUDE.md - FrameSupplier Module

**Module**: `modules/framesupplier`
**Bounded Context**: Non-blocking frame distribution with drop policy for real-time video processing
**Philosophy**: "Drop frames, never queue. Latency > Completeness."

---

## 🎯 For Claude Agents: Read Before Starting

**Module-specific context**:
1. **[Global CLAUDE_CONTEXT.md](../../CLAUDE_CONTEXT.md)** - AI-to-AI patterns (HOW to pair with Ernesto) - **READ FIRST**
2. **[PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md)** - Discovery process
3. **This document** - FrameSupplier module specifics

**Expected ramp-up**: <5 minutes (global context) + <5 minutes (module context) = <10 minutes total.

---

## How to Work with Claude: Session Types

This module supports two types of pairing sessions. **Claude should detect which type based on Ernesto's initial message.**

---

### 🔍 Discovery Session (Exploratory Design)

**When**: Designing new functionality, exploring uncharted design space

**Trigger Phrases** (Ernesto says):
- "Charlemos el diseño de [feature]"
- "Pensaba en [technical decision]... ¿qué te parece?"
- "Pair-discovery: [topic]"
- "¿Cómo atacamos [problema] desde diseño?"

**Claude's Protocol**:
1. **Load**: [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md) (read entire document)
2. **Identify Bounded Context**: Ask if unclear
3. **Recognize Point Silla**: Entry point for exploration (technical decision with arch implications)
4. **Follow 3-Phase Process**:
   - **Point Silla** → Entry point (opens options, doesn't close them)
   - **Discovery** → Think together (propose, challenge, synapse, checkpoint every 3-5 decisions)
   - **Crystallization** → Document gold (ADRs + architecture docs + emergent insights)
5. **Produce**: ADRs + Architecture docs + Named insights

**DO**:
- ✅ Explore tradeoffs in THIS context (not generic best practices)
- ✅ Challenge proposals with reasoning (enable discovery)
- ✅ Name insights when they emerge ("This is a [Pattern Name] pattern...")
- ✅ Checkpoint every 3-5 decisions ("¿Vamos bien?")

**DO NOT**:
- ❌ "Let me research best practices" (think in context, not external authority)
- ❌ Implement prematurely (discovery first, code later)
- ❌ Over-explain basics (Ernesto is senior architect)
- ❌ Defend proposals dogmatically (accept challenges gracefully)

**Example Initial Message**:
```
Ernesto: "Charlemos el diseño de priority-based frame distribution.
         Pensaba en 4 priority levels (Critical, High, Normal, BestEffort)
         mapeando a worker SLAs... ¿qué te parece?"

Claude: "Priority-based distribution is interesting. Let me explore tradeoffs:

1. 4 Priority Levels:
   ✅ Pro: Maps directly to worker criticality (PersonDetector=Critical, VLM=BestEffort)
   ✅ Pro: Simpler than 5-level industry standard
   ❌ Con: Under load, how do we decide who gets dropped?

2. Drop Policy:
   - Option A: Drop lowest priority first (simple)
   - Option B: Drop based on consecutiveDrops (fairness)
   - Option C: No priority (current design, all equal)

In context of Orion (fall detection = life-critical):
- PersonDetector drops → EdgeExpert blind → Unacceptable
- VLM drops → Research impacted → Acceptable

My intuition: Priority justifies complexity for critical workers.

¿Qué pensás? ¿O esto es scope creep para FrameSupplier (debería ser worker-lifecycle)?
```

---

### 💻 Coding Session (Implementation)

**When**: Implementing already-designed functionality (ADRs + ARCHITECTURE.md exist)

**Trigger Phrases** (Ernesto says):
- "Implementemos [module] según los ADRs"
- "Escribí el código para [component]"
- "Según ADR-001, [decision]. Empecemos."
- "Arrancamos con [feature] (diseño ya definido)"

**Claude's Protocol**:
1. **Load**: Relevant ADRs + ARCHITECTURE.md
2. **Understand Decisions**: Design is decided, do NOT explore alternatives
3. **Implement**: As specified in docs
4. **Test**: Write tests (property-based when applicable, pero Ernesto los corre)
5. **Produce**: Working code + tests + inline documentation

**DO**:
- ✅ Follow ADRs exactly (design already validated)
- ✅ Ask clarifications if ADR ambiguous
- ✅ Implement with Go best practices (atomics, mutexes, etc.)
- ✅ Compile-check always (compilation = primary test)

**DO NOT**:
- ❌ Explore alternatives ("¿Deberíamos considerar channels también?") → NO, ADR-001 decided sync.Cond
- ❌ Question design decisions (that was for discovery session)
- ❌ Over-engineer (implement what ADR says, not more)

**Example Initial Message**:
```
Ernesto: "Implementemos FrameSupplier inbox mailbox.
         Según ADR-001 y ADR-004:
         - sync.Cond para blocking semantics
         - Overwrite policy (JIT)
         - Zero-copy (shared pointers)

         Empecemos con inbox.go"

Claude: "Entendido. Implementando inbox mailbox según ADRs:
- ADR-001: sync.Cond + mutex + single slot
- ADR-004: Symmetric JIT (non-blocking Publish, blocking consume)
- ADR-002: Zero-copy (share *Frame pointer)

Estructura:
```go
type Supplier struct {
    inboxMu    sync.Mutex
    inboxCond  *sync.Cond
    inboxFrame *Frame
    inboxDrops uint64
    // ...
}
```

Escribiendo inbox.go..."
```

---

### 🚨 Important: DO NOT Mix Protocols

**❌ Wrong** (exploring during coding session):
```
Ernesto: "Implementemos inbox con sync.Cond"
Claude: "Wait, ¿no deberíamos explorar channels también?"
```
→ NO. Design already decided (ADR-001). Just implement.

**❌ Wrong** (coding during discovery session):
```
Ernesto: "Charlemos restart policies"
Claude: "Ok, voy a implementar exponential backoff..."
```
→ NO. Explore alternatives first, decide, then implement later.

---

### 🎯 If Ambiguous: Ask Explicitly

If Claude cannot determine session type from Ernesto's message:

```
Claude: "¿Esto es discovery session (explorar diseño) o coding session (implementar)?

- Discovery: Exploramos alternativas, cuestionamos, documentamos
- Coding: Implementamos según ADRs ya existentes

¿Cuál prefieres?"
```

---

## Module Philosophy

**Context**: Este módulo es infraestructura crítica, no código de aplicación.

### Performance is King

> "En este tipo de librería/módulo grabamos sobre roca: **performance siempre gana**."

- FrameSupplier es el "highway" del sistema (todos los workers, todos los streams)
- Competimos con GStreamer/DeepStream (all-in-RAM, zero-copy)
- 10× más importante que código de aplicación

**Implications**:
- Zero-copy is non-negotiable (192 MB/s savings @ 64 workers, 30fps)
- Batching justified by scale (threshold=8 guardrails)
- Concurrency expertise required (not "simple sequential")

---

### Complexity by Design (Not by Accident)

> "Simplicidad para módulos simples es estúpido. FrameSupplier NO es módulo simple."

**KISS applies at**:
- ✅ **Macro level**: Bounded context clear (distribution only, not worker lifecycle)
- ✅ **API level**: Simple for client (Publish, Subscribe, Stats)

**KISS does NOT apply at**:
- ❌ **Internal implementation**: Attack complexity with expert design (batching, zero-copy, JIT)

**Rationale**:
- Already gave simplicity via bounded context
- Inside module, optimize ruthlessly
- "Simple para leer, NO simple para escribir una vez"

---

### Think from First Principles (Not Internet)

> "Siempre pensemos sin buscar en internet. Estado del arte, best practices de Go/concurrency,
> luego pasamos por el tamiz técnico: diseño propio en contexto."

**Process**:
1. **Know the rules**: Bounded contexts, non-blocking guarantees, backward compatibility
2. **Think in context**: "¿Este pattern aplica en ESTE contexto?"
3. **Validate with pair**: "Ernesto, propongo X en vez de Y porque Z. ¿Qué pensás?"

**NOT**:
- ❌ "Industry standard says 5 priority levels" → We use 4 (maps to our SLAs)
- ❌ "Let me search sync.Cond patterns" → Think through tradeoffs in our context

---

### Contract vs Implementation

> "Siempre pensando en contrato externo / implementación:
> KISS en API para cliente, pero dentro atacamos complejidad con diseño experto."

**External Contract** (simple):
```go
supplier.Publish(frame)           // Non-blocking, simple
readFunc := supplier.Subscribe(id) // Returns blocking function
```

**Internal Implementation** (complex when justified):
- Inbox mailbox (sync.Cond + mutex + overwrite)
- Batching with threshold=8 (adapts to scale)
- Zero-copy (immutability contract)
- Operational stats (idle detection, drops)

**Cohesion**: Module is cohesive (one responsibility: distribution). Complexity inside is justified.

---

## Design Principles Specific to This Module

### 1. JIT End-to-End (Symmetric Architecture)

**"Casa de herrero, cuchillo de acero"**

- FrameSupplier preaches JIT to workers → Must practice JIT on input
- Inbox mailbox with overwrite (not queue)
- Workers mailbox with overwrite (not queue)
- Symmetry at all levels

**See**: ADR-004 (Symmetric JIT Architecture)

---

### 2. Physical Invariants Simplify Design

**Example from this module**:
```
Distribution latency: 100µs
Inter-frame @ 1fps: 1,000,000µs
Ratio: 10,000×

→ Physical invariant: Distribution always completes before next frame
→ Fire-and-forget correct (no wg.Wait needed)
→ Ordering guaranteed by physics, not code
```

**Pattern**: "If A's latency >> B's interval, ordering guaranteed. No explicit sync needed."

**See**: ADR-003 (Fire-and-forget rationale)

---

### 3. Threshold from Business Context

**Not just math**:
- Break-even: 12 workers (sequential = batched)
- Threshold: 8 workers (before break-even)

**Why 8?**
- POC: ≤5 workers (sequential perfect)
- Expansion: ≤10 workers (sequential still good)
- Full: ≤64 workers (batching kicks in)

**Principle**: "Tuning parameters need business rationale, not just benchmark rationale."

**See**: ADR-003 (Batching with Threshold=8)

---

## Documentation Structure

```
docs/
├── C4_MODEL.md          # High-level views (architects, onboarding)
├── ARCHITECTURE.md      # Deep dive (implementers, debugging)
└── ADR/
    ├── README.md
    ├── 001-sync-cond-for-mailbox-semantics.md
    ├── 002-zero-copy-frame-sharing.md
    ├── 003-batching-threshold-8.md
    └── 004-symmetric-jit-architecture.md
```

**When to Read**:
- **Discovery session**: Read to understand context, not to constraint creativity
- **Coding session**: Read ADRs as spec (implement exactly as specified)

---

## Testing Philosophy

> "Los tests siempre. Testear como pair-programming: dime los tests, yo los corro manualmente.
> Es más tedioso pero tenemos enfoque de revisión entre pares."

**Claude's Role**:
- ✅ Write test code (with property-based when applicable)
- ✅ Explain what test validates
- ✅ Ensure compilation (primary test)

**Ernesto's Role**:
- ✅ Run tests manually
- ✅ Validate results
- ✅ Pair-review approach

**Compilation = Test**: Always ensure code compiles (no test is compilation failing).

---

## Future Context (For Next Sessions)

### Coding Session (Próxima)
**Input**: ADRs 001-004 + ARCHITECTURE.md (ya definido)
**Output**: Working FrameSupplier implementation

**Start with**:
- inbox.go (ADR-001, ADR-004)
- distribution.go (ADR-003)
- worker_slot.go (ADR-001)
- frame.go (ADR-002)

### Discovery Session (Worker-Lifecycle Module)
**Input**: Point silla (restart policies?)
**Output**: ADRs + architecture for worker-lifecycle

**Context to carry**:
- FrameSupplier publishes to workers (interface boundary)
- Workers have SLAs (Critical, High, Normal, BestEffort)
- Restart policies should respect criticality

---

## References

- [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md) - Operating manual for discovery sessions
- [MANIFIESTO.md](../../modules/framesupplier/MANIFIESTO.md) - Raw philosophy wall (not operational)
- [Global CLAUDE.md](../../CLAUDE.md) - Project-level context (Orion overview)

---

**Review Status**: 🟢 Ready for implementation (ADRs complete, design validated)
- monolito ni en los documentos