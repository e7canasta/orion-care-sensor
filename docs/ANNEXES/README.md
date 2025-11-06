# Anexos al Protocolo de Pair-Discovery

Este directorio contiene **unidades de pensamiento** (patrones de meta-diseño) que complementan el [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md).

---

## Quick Access (Acceso Rápido)

**Para Claude Agents (ramp-up rápido)**:
1. **[PATTERN_CATALOG.md](./PATTERN_CATALOG.md)** - Cheatsheet con aka + bridges (multilingüe: Visiona ↔ Claude ↔ Industria)
2. **[VISUAL_MAP.md](./VISUAL_MAP.md)** - Mapa mental ASCII (hemisferio espacial, orientación rápida)

**Para Deep Dive (lectura completa)**:
- Anexos 001-006 (full docs, ~600 líneas c/u)

---

## Filosofía de Anexos

**Protocolo Base** (PAIR_DISCOVERY_PROTOCOL.md):
- Framework operativo (Point Silla → Discovery → Crystallization)
- Session types (Discovery vs Coding)
- DO/DON'T básicos
- **Inmutable** (salvo mejoras al proceso)

**Anexos** (este directorio):
- Patrones de meta-diseño
- Unidades de pensamiento cristalizadas
- **Crecen orgánicamente** (cada insight se documenta)

---

## Índice de Anexos

### ANNEX-001: Thinking in Chains (Rey No Ahogado)
**Meta-Principio**: Óptimo Local vs Movilidad Futura
**Status**: 🟢 Activo

**Qué resuelve**:
- Diseñar módulos que escalan sin refactor (r1.0 → r2.0 → r3.0)
- Tests mentales: Scale horizontal, movimientos futuros, estabilidad
- ADRs como contratos de cadena (proveedor → nosotros → cliente)

**Cuándo aplicar**:
- Discovery de módulos core (FrameSupplier, stream-capture, etc.)
- Decisiones que impactan arquitectura multi-release
- Validación de bounded contexts

**Referencias**:
- Caso de estudio: FrameSupplier (cambió <10% de patineta a avión)
- Checklist completo en [ANNEX-001](./ANNEX-001_THINKING_IN_CHAINS.md)

---

### ANNEX-002: Bounded Contexts (Cohesión por Dominio)
**Meta-Principio**: Módulos por Responsabilidad, No por Tamaño
**Status**: 🟢 Activo

**Qué resuelve**:
- Cuándo separar módulo (SRP aplicado a arquitectura)
- Test de cohesión: "¿Un solo motivo para cambiar?"
- Boundaries: Qué NO hace un módulo (IN SCOPE / OUT OF SCOPE)

**Cuándo aplicar**:
- Discovery de nueva arquitectura modular
- Decisión de separar/fusionar módulos
- Validación de responsabilidades (SRP)

**Tests clave**:
1. ¿Un solo motivo para cambiar?
2. ¿Independiente? (testeable en aislación)
3. ¿Expertise domain claro?

**Referencias**:
- Caso de estudio: Orion 2.0 multi-module monorepo
- Checklist completo en [ANNEX-002](./ANNEX-002_BOUNDED_CONTEXTS.md)

---

### ANNEX-003: Physical Invariants (Física Simplifica Diseño)
**Meta-Principio**: Si Latencia A >> Intervalo B, Orden Garantizado por Física
**Status**: 🟢 Activo

**Qué resuelve**:
- Cuándo NO necesitamos sincronización explícita (fire-and-forget)
- Benchmarks que se convierten en architectural decisions
- Simplificar diseño confiando en la física del sistema

**Cuándo aplicar**:
- Discovery de concurrency patterns (sync.Cond, channels, wg.Wait)
- Decisión de fire-and-forget vs explicit synchronization
- Validación de propiedades garantizadas por latencias

**Tests clave**:
1. ¿Ratio latency/interval < 0.01? (>100× más rápido)
2. ¿p99 también cumple? (worst case robusto)
3. ¿Qué pasa si ratio ≈ 1? (sistema colapsó)

**Referencias**:
- Caso de estudio: FrameSupplier fire-and-forget (ratio 333×)
- ADR-003 (Fire-and-forget Distribution)
- Checklist completo en [ANNEX-003](./ANNEX-003_PHYSICAL_INVARIANTS.md)

---

### ANNEX-004: Batching with Guardrails
**Meta-Principio**: Threshold from Business Context, Not Just Math
**Status**: 🟢 Activo

**Qué resuelve**:
- Cuándo batching es correcto (no prematuro)
- Threshold from business context (break-even vs actual threshold)
- Performance optimization sin over-engineering

**Cuándo aplicar**:
- Discovery de performance optimizations
- Decisión de activar optimización (threshold selection)
- Validación de complejidad vs beneficio

**Tests clave**:
1. ¿Break-even matemático? (benchmark)
2. ¿Fases de negocio? (POC, Expansion, Full)
3. ¿Threshold alineado con fases? (safety margin)

**Referencias**:
- Caso de estudio: FrameSupplier batching (threshold=8, break-even=12)
- ADR-003 (Batching with Threshold=8)
- Checklist completo en [ANNEX-004](./ANNEX-004_BATCHING_WITH_GUARDRAILS.md)

---

### ANNEX-005: Forecast Arquitectónico
**Meta-Principio**: Capturar Conocimiento Futuro sin Violar YAGNI
**Status**: 🟢 Activo

**Qué resuelve**:
- PROPOSAL lifecycle (Discovery → Forecast → ADR)
- Blues con Guardrails (seniority vs sobre-diseño)
- Los Tres Ojos (producto-arquitectura-código balance)
- Cuándo documentar forecast vs implementar

**Cuándo aplicar**:
- Discovery sessions donde emergen insights r2.0, r3.0
- Decisión de crear PROPOSAL vs ADR
- Validación de forecast con guardrails (no sobre-diseño)

**Tests clave**:
1. ¿Implementar o Documentar? (r2.0 pedido vs forecast)
2. ¿Sobre-Diseño o Seniority? (guardrails presentes)
3. ¿Los Tres Ojos Balanceados? (producto, arquitectura, código)

**Referencias**:
- Caso de estudio: FrameSupplier P001 (multi-stream), P002 (frame-buffer)
- Template PROPOSAL completo
- Checklist completo en [ANNEX-005](./ANNEX-005_FORECAST_ARQUITECTONICO.md)

---

### ANNEX-006: Unix Philosophy & Composability
**Meta-Principio**: Do One Thing Well, Compose for Complexity
**Status**: 🟢 Activo

**Qué resuelve**:
- Separación de concerns (bounded contexts)
- Pipe + Tee philosophy (1→N vs N→1)
- Composition > Monolith (componibilidad)
- Cuándo módulo separado vs integrado

**Cuándo aplicar**:
- Discovery de nueva funcionalidad (¿módulo nuevo?)
- Decisión de separar/fusionar módulos
- Validación de composability (Unix-style)

**Tests clave**:
1. ¿Módulo Separado o Integrado? (SRP, coupling, reusability)
2. ¿Pipe o Tee? (1→N distribution vs N→1 multiplexing)
3. ¿Composable? (interfaces, low coupling)

**Referencias**:
- Caso de estudio: frame-buffer como tee (Unix Philosophy)
- Anti-patterns: Dios Module, Utility Hell, Premature Abstraction
- Checklist completo en [ANNEX-006](./ANNEX-006_UNIX_PHILOSOPHY_COMPOSABILITY.md)

---

## Futuros Anexos (Roadmap)

### ANNEX-007: Zero-Copy Architectures
**Meta-Principio**: "Ownership transfer > Memory copy"
**Status**: 🟡 Propuesto (no escrito)

**Qué resolvería**:
- Cuándo zero-copy es crítico (throughput vs latency)
- Immutability contracts (quién es dueño de qué)
- Trade-offs: zero-copy vs simplicity

---

### ANNEX-008: JIT End-to-End (Symmetric Architecture)
**Meta-Principio**: "Casa de herrero, cuchillo de acero"
**Status**: 🟡 Propuesto (no escrito)

**Qué resolvería**:
- Cuándo JIT en ambos extremos (inlet + outlet)
- Symmetric design principles (coherencia arquitectónica)
- Drop > Queue philosophy

---

## Cómo Proponer Nuevo Anexo

Si durante pair-discovery emerge un patrón repetible:

1. **Identificar insight**: "Esto es un [Nombre] pattern..."
2. **Validar repetibilidad**: ¿Aplica en >1 módulo?
3. **Proponer anexo**: Agregar a "Futuros Anexos" con status 🟡
4. **Pair-discovery del anexo**: Charlarlo, crystallizarlo
5. **Escribir anexo**: Crear ANNEX-00X.md
6. **Actualizar índice**: Mover de 🟡 Propuesto a 🟢 Activo

---

## Uso Durante Discovery Session

### Claude debe:
1. **Leer anexos relevantes** antes de discovery (según módulo/decisión)
2. **Aplicar tests mentales** del anexo (ej: scale horizontal, movimientos futuros)
3. **Referenciar anexo en ADRs** cuando aplique (ej: "Ver ANNEX-001 para tests")
4. **Proponer nuevo anexo** si emerge patrón nuevo

### Ernesto debe:
- Referenciar anexo cuando aplique ("¿Rey ahogado?" → ANNEX-001)
- Validar que Claude aplicó tests correctamente
- Identificar nuevos patrones para anexar

---

## Versionado

- Anexos son **versionados** (v1.0, v1.1, etc.)
- Protocolo base referencia anexos (no duplica contenido)
- Cambios mayores → Nueva versión de anexo (backward compatible)

---

## Referencias

- **Protocolo Base**: [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md)
- **Global Context**: [CLAUDE_CONTEXT.md](../../CLAUDE_CONTEXT.md)
- **Proyecto**: [CLAUDE.md](../../CLAUDE.md)

---

**Última actualización**: 2025-01-06
**Anexos activos**: 6 (ANNEX-001, 002, 003, 004, 005, 006)
**Anexos propuestos**: 2 (ANNEX-007, 008)
