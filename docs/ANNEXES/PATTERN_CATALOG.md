# Pattern Catalog (Cheatsheet Multilingüe)

**Propósito**: Vocabulario compartido Visiona ↔ Claude ↔ Industria

**Uso**: Quick reference con aka (múltiples entry points) + bridges a corpus de Claude

---

## Rey No Ahogado

**Término Visiona**: Rey No Ahogado
**Origen**: Ajedrez (rey sin movimientos legales = stalemate)

**Aka**:
- Local Optimum with Mobility (formal)
- Chess King Stalemate (analogía directa)
- Painted into Corner (idiom inglés)
- No Movimientos Futuros

**Claude Corpus Bridge**:
- Technical debt that closes future options
- Design decisions without evolution paths
- Premature optimization constrains growth
- Architecture inflexibility

**Industria Terms**:
- Architectural Technical Debt
- Evolution Path Constraint
- Design Lock-in

**Metaphor Power**:
```
Rey atrapado en tablero = Arquitectura sin movimientos

vs "technical debt" (abstracto, no evoca sensación)
→ "Rey ahogado" evoca estar ATRAPADO (más rico semánticamente)
```

**Intent**: r1.0 NO cierra r2.0 (movilidad preservada, movimientos futuros posibles)

**Tests**:
1. ¿Scale horizontal? (r2.0 = instanciar N veces sin refactor)
2. ¿Movimientos futuros? (>1 opción para r3.0)
3. ¿Estabilidad? (de patineta a avión, cambios <10%)

**Full Doc**: [ANNEX-001](./ANNEX-001_THINKING_IN_CHAINS.md)

---

## Blues con Guardrails

**Término Visiona**: Blues con Guardrails
**Origen**: Jazz/Blues (improvisación dentro de estructura musical)

**Aka**:
- Controlled Improvisation (formal)
- Seniority vs Over-Design (contraste)
- Jazz with Structure (analogía musical)
- Forecast con YAGNI (técnico)

**Claude Corpus Bridge**:
- Exploration with constraints (process)
- Senior-level architectural forecasting (expertise)
- YAGNI-compliant forward-thinking (balance)
- Informed risk-taking (senior skillset)

**Industria Terms**:
- Architectural Forecasting
- Design Space Exploration
- Responsible Innovation

**Metaphor Power**:
```
Blues = Escala pentatónica + progresión I-IV-V (guardrails musicales)
      + Improvisación del músico (seniority)

vs "architectural forecasting" (seco, no evoca creatividad)
→ "Blues con guardrails" evoca creatividad DENTRO de límites (más rico)
```

**Intent**: Forecast r2.0 sin violar YAGNI (seniority permite explorar, guardrails protegen)

**Guardrails**:
- Bounded contexts claros (certeza de dominio)
- Tests mentales pasan (validación objetiva)
- Implementamos SOLO r1.0 (YAGNI respetado)
- Value proposition entendido (producto = norte)

**Tests**:
1. ¿Guardrails presentes? (bounded contexts, tests, YAGNI)
2. ¿Los Tres Ojos balanceados? (producto, arquitectura, código)
3. ¿PROPOSAL creado? (documentar, no implementar r2.0)

**Full Doc**: [ANNEX-005](./ANNEX-005_FORECAST_ARQUITECTONICO.md)

---

## Los Tres Ojos

**Término Visiona**: Los Tres Ojos
**Origen**: Balance multidimensional (3 perspectivas simultáneas)

**Aka**:
- Product-Architecture-Code Balance (formal)
- Norte-Rieles-Transporte (analogía de transporte)
- Triad of Concerns (formal)
- Multi-Dimensional Thinking

**Claude Corpus Bridge**:
- Multi-stakeholder thinking (product, technical, implementation)
- Systems thinking (holistic view)
- Balanced architecture (not code-centric)
- Strategic-tactical balance

**Industria Terms**:
- Product-Driven Architecture
- Holistic System Design
- Context-Aware Engineering

**Metaphor Power**:
```
3 ojos viendo simultáneamente (Ojo 1, 2, 3 activos)

vs "multi-dimensional thinking" (abstracto)
→ "Los Tres Ojos" evoca visión COMPLETA, no perder ángulo (visual, memorable)
```

**Intent**:
- **Ojo 1 (Producto)**: Norte (value proposition, business need, cómo crece)
- **Ojo 2 (Arquitectura)**: Rieles (movilidad futura, bounded contexts, evolución)
- **Ojo 3 (Código)**: Transporte (implementación simple, performance adecuado)

**Anti-Pattern**: Código como norte (optimizar código sin considerar producto/arquitectura)

**Tests**:
1. ¿Producto claro? (value proposition, business driver)
2. ¿Arquitectura habilita r2.0? (bounded contexts, movilidad)
3. ¿Código simple? (YAGNI, no prematuro)

**Full Doc**: [ANNEX-005](./ANNEX-005_FORECAST_ARQUITECTONICO.md)

---

## Ojo de Sauron

**Término Visiona**: Ojo de Sauron
**Origen**: LOTR (vigilancia omnipresente con foco único e intenso)

**Aka**:
- Attention Mechanism with Single Focus (técnico ML)
- Focal ROI (técnico CV, menos rico)
- All-Seeing Eye (analogía literaria)
- Priority Attention

**Claude Corpus Bridge**:
- Attention mechanism (ML/AI, transformers)
- Region of Interest with priority (computer vision)
- Selective processing (cognitive psychology)
- Saliency detection

**Industria Terms**:
- Attention ROI
- Priority-Based Processing
- Selective Attention
- Focus Mechanism

**Metaphor Power**:
```
Ojo que ve TODO (omnipresente) pero se enfoca en UNO (foco único)

vs "attention ROI" (técnico, frío)
→ "Ojo de Sauron" evoca vigilancia + poder + criticidad + omnipresencia
   (más rico semánticamente, memorable)
```

**Intent**: Foco en región crítica (ej: persona caída) sin perder contexto global (awareness completo)

**Use Cases**:
- Fall detection (foco en persona, contexto en sala)
- Activity recognition (foco en acción, contexto en escena)
- Priority-based inference (foco en Critical workers, contexto en todos)

**Full Doc**: TBD (futuro anexo sobre attention mechanisms)

---

## Bounded Context

**Término Visiona**: Bounded Context (adoptado de DDD)
**Origen**: Domain-Driven Design (Eric Evans)

**Aka**:
- Separation of Concerns (principio general)
- Single Responsibility (SRP arquitectónico)
- Do One Thing Well (Unix Philosophy)
- Cohesión por Dominio

**Claude Corpus Bridge**:
- Modular decomposition (computer science)
- Service boundaries (microservices)
- Responsibility assignment (SOLID principles)
- Domain modeling (DDD)

**Industria Terms**:
- Microservices Boundaries
- Module Boundaries
- Domain Boundaries

**Metaphor Power**:
```
"Bounded Context" es técnico pero claro (fronteras explícitas)

No hay metáfora más rica porque el término ya es preciso.
```

**Intent**: Módulo = 1 responsabilidad, cambios aislados (cohesión alta, coupling bajo)

**Tests**:
1. ¿Un solo motivo para cambiar? (SRP)
2. ¿Independiente? (testeable en aislación)
3. ¿Expertise domain claro? (qué experto necesito)

**Full Docs**:
- [ANNEX-002](./ANNEX-002_BOUNDED_CONTEXTS.md) (SRP arquitectónico)
- [ANNEX-006](./ANNEX-006_UNIX_PHILOSOPHY_COMPOSABILITY.md) (Do One Thing Well)

---

## Pipe + Tee

**Término Visiona**: Pipe + Tee (adoptado de Unix)
**Origen**: Unix Philosophy (Doug McIlroy, Bell Labs)

**Aka**:
- 1→N vs N→1 (formal)
- Distribution vs Multiplexing (técnico)
- Composition > Monolith (filosofía)
- Unix Philosophy

**Claude Corpus Bridge**:
- Fan-out vs fan-in (messaging patterns)
- Pub-sub vs aggregation (event systems)
- Scatter-gather (distributed systems)
- Pipeline composition (functional programming)

**Industria Terms**:
- Message Distribution/Aggregation
- Event Fan-out/Fan-in
- Stream Processing

**Metaphor Power**:
```
Pipe (|) = Distribution 1→N (stdout → stdin de N comandos)
Tee      = Multiplexing N→1 + side effect (N inputs → 1 output + file)

vs "distribution/aggregation" (técnico)
→ "Pipe + Tee" evoca composición Unix (herramientas simples que trabajan juntas)
```

**Intent**: Componer módulos simples para complejidad (no monolitos)

**Tests**:
1. ¿Pipe (1→N) o Tee (N→1)? (dirección de flujo)
2. ¿Módulo separado? (SRP, reusabilidad)
3. ¿Composable? (interfaces, low coupling)

**Examples**:
- framesupplier = pipe (1 frame → N workers)
- frame-buffer = tee (N suppliers → 1 heavy worker)

**Full Doc**: [ANNEX-006](./ANNEX-006_UNIX_PHILOSOPHY_COMPOSABILITY.md)

---

## Physical Invariant

**Término Visiona**: Physical Invariant (físico = no cambia, invariante = constante)
**Origen**: Física del sistema (latencias medidas, ratios)

**Aka**:
- Trust the Physics (slogan)
- Latency Guarantees Order (técnico)
- Fire-and-Forget (pattern resultante)
- Ratio-Based Ordering

**Claude Corpus Bridge**:
- Performance analysis (sistemas real-time)
- Ordering guarantees from timing (distributed systems)
- Latency budgets (performance engineering)
- Physical constraints enable simplicity

**Industria Terms**:
- Latency Analysis
- Timing Guarantees
- Performance Invariants

**Metaphor Power**:
```
"Physical Invariant" = La física del sistema garantiza propiedad

vs "latency analysis" (proceso, no resultado)
→ "Physical Invariant" evoca confianza en FÍSICA (inmutable, no código)
```

**Intent**: Si latency(A) << interval(B), orden garantizado por física (no necesitamos sync explícita)

**Test**: Ratio < 0.01 (A es >100× más rápido que B)

**Example**:
```
Distribution: 100µs
Inter-frame @ 30fps: 33,333µs
Ratio: 0.003 (333× más rápido)

→ Fire-and-forget correcto (física garantiza orden)
```

**Full Doc**: [ANNEX-003](./ANNEX-003_PHYSICAL_INVARIANTS.md)

---

## Threshold from Business

**Término Visiona**: Threshold from Business Context
**Origen**: Guardrails de negocio (no solo benchmarks)

**Aka**:
- Business Guardrails (filosofía)
- Break-Even + Safety Margin (técnico)
- POC-Expansion-Full Phases (contexto)
- Optimization with Context

**Claude Corpus Bridge**:
- Performance optimization with business context
- Tuning parameters from deployment phases
- Safety margins in production systems
- Context-aware thresholds

**Industria Terms**:
- Business-Driven Optimization
- Context-Aware Tuning
- Deployment Phase Planning

**Metaphor Power**:
```
"Threshold from Business" = No solo matemática, contexto de fases

vs "break-even analysis" (solo benchmarks)
→ "Threshold from Business" evoca GUARDRAILS de negocio (protege fases tempranas)
```

**Intent**: Optimización activada por fases de negocio (no solo cuando benchmarks dicen)

**Formula**: Threshold ≈ 0.6 × Break-Even (40% antes del punto matemático)

**Example**:
```
Break-even matemático: N=12 workers
Fases:
  POC: 3-5 workers
  Expansion: 8-10 workers
  Full: 32-64 workers

Threshold: 8 workers (garantiza sequential en POC, batched en Full)
```

**Full Doc**: [ANNEX-004](./ANNEX-004_BATCHING_WITH_GUARDRAILS.md)

---

## Forecast Arquitectónico

**Término Visiona**: Forecast Arquitectónico
**Origen**: PROPOSAL lifecycle (Discovery → Forecast → ADR)

**Aka**:
- PROPOSAL Lifecycle (proceso)
- Capture Gold without YAGNI Violation (balance)
- Café Inception (metáfora interna 🎸)
- Knowledge Preservation

**Claude Corpus Bridge**:
- Architectural decision capture
- Future-proofing without over-engineering
- Knowledge management in agile
- RFC/Design Doc pattern

**Industria Terms**:
- Architectural RFCs
- Design Docs
- Future Architecture Planning

**Metaphor Power**:
```
"Forecast Arquitectónico" = Predecir sin implementar (meteorólogo vs constructor)

vs "design docs" (genérico)
→ "Forecast" evoca previsión SIN commitment (forecast ≠ plan detallado)
```

**Intent**: Documentar r2.0/r3.0 sin implementar (knowledge preserved, YAGNI respetado)

**Lifecycle**:
```
Discovery → PROPOSAL (forecast) → Validated → ADR (implemented)
```

**Tests**:
1. ¿Implementar o Documentar? (r2.0 pedido = ADR, no pedido = PROPOSAL)
2. ¿Guardrails presentes? (bounded contexts, tests mentales, YAGNI)
3. ¿Los Tres Ojos? (producto-arquitectura-código balanceados)

**Full Doc**: [ANNEX-005](./ANNEX-005_FORECAST_ARQUITECTONICO.md)

---

## Patineta → Avión

**Término Visiona**: Patineta → Bicicleta → Auto → Avión
**Origen**: MVP funcional (vs Rueda → Volante → Motor)

**Aka**:
- MVP Funcional en Cada Fase (formal)
- Incremental Value Delivery (técnico)
- Skateboard Metaphor (industria)

**Claude Corpus Bridge**:
- Iterative development
- Minimum Viable Product (MVP)
- Incremental delivery
- Functional milestones

**Industria Terms**:
- Iterative MVP
- Incremental Product Development
- Functional Iterations

**Metaphor Power**:
```
Patineta → Bicicleta → Auto (cada uno FUNCIONA, te mueves)

vs Rueda → Volante → Motor (no funciona hasta el final)

→ "Patineta → Avión" evoca UTILIDAD INMEDIATA en cada fase (más claro que "iterative")
```

**Intent**: Cada release es funcional (no piezas que no funcionan solas)

**Example**:
```
r1.0: Patineta (single stream, funcional)
r2.0: Bicicleta (multi-stream, funcional)
r3.0: Auto (heavy workers, funcional)
r4.0: Avión (optimizado, funcional)

Cada release funciona, no esperamos r4.0 para tener valor.
```

**Full Doc**: [ANNEX-001](./ANNEX-001_THINKING_IN_CHAINS.md) (sección evolution)

---

## Uso del Catalog

### Para Claude Agents (IA-to-IA)

**Cuando Ernesto dice término Visiona**:
1. Buscar en catalog (aka = múltiples entry points)
2. Entender via "Claude Corpus Bridge" (mi conocimiento)
3. Responder usando término Visiona (respeto vocabulario)

**Ejemplo**:
```
Ernesto: "Estamos en situación de rey ahogado"

Claude (interno):
  - Catalog lookup: "Rey ahogado"
  - Bridge: "Technical debt that closes options, design lock-in"
  - Entiendo: r1.0 cierra r2.0 (no hay movimientos futuros)

Claude (respuesta):
  "Entiendo, estamos en 'rey ahogado' - r1.0 cierra r2.0 sin movimientos.
   ¿Tests mentales? ¿Scale horizontal? ¿Opciones para r3.0?"
```

---

### Para Humanos (Quick Reference)

**Búsqueda rápida**: Ctrl+F por aka (múltiples entry points)

**Ejemplos**:
- Buscar "Chess King" → Encuentra "Rey No Ahogado"
- Buscar "Focal ROI" → Encuentra "Ojo de Sauron"
- Buscar "YAGNI" → Encuentra "Blues con Guardrails", "Forecast Arquitectónico"

---

## Referencias

- **Protocol**: [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md)
- **Visual Map**: [VISUAL_MAP.md](./VISUAL_MAP.md) (mapa mental ASCII)
- **Full Annexes**: [README.md](./README.md) (índice completo)

---

**Versión**: 1.0
**Autor**: Pair-discovery sessions (Ernesto + Claude)
**Fecha**: 2025-11-06
**Status**: 🟢 Activo (vocabulario vivo, crece con vibe sessions)

**Compactación Futura**: v2.0 condensará anexos, este catalog será index principal.
