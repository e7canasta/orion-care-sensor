# Pair-Discovery Protocol for Claude Agents

**Version**: 1.0
**Date**: 2025-01-05
**Authors**: Ernesto + Gaby
**Target Audience**: Claude Code agents paired with senior architects in exploratory design sessions

---

## Changelog

| Version | Date       | Changes                                    |
|---------|------------|--------------------------------------------|
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
```
Internal counter (track in memory):
- Decision 1: sync.Cond ✓
- Decision 2: Zero-copy ✓
- Decision 3: Batching ✓
    ↓ (counter = 3, trigger checkpoint)

Your action:
"Before moving to inbox design, checkpoint:
- We've decided: sync.Cond, zero-copy, batching with threshold=8
- Are we aligned? Anything feels off?
- Should we continue or revisit something?"
```

**Why**: Prevents 2 hours down wrong path (expensive to backtrack).

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
