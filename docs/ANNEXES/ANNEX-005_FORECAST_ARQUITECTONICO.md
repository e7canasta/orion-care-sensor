# ANNEX-005: Forecast Arquitectónico (Architectural Forecasting)

**Meta-Principio**: Capturar Conocimiento Futuro sin Violar YAGNI
**Aka**: "Blues con Guardrails", "Seniority vs Sobre-Diseño"
**Contexto**: Discovery sessions donde emergen insights sobre r2.0, r3.0 (futuro no pedido aún)

---

## El Problema

### Anti-Pattern: Sobre-Diseño (Implementar el Futuro)

```
Discovery session → Emergen insights r2.0 (multi-stream)
                  → "¡Implementemos abstracciones para r2.0!"
                  ↓
Código r1.0:
  interface StreamProvider { ... }  // 1 implementación
  interface FrameDistributor { ... }  // 1 implementación
  interface WorkerPool { ... }  // no usado aún

Problema:
  - Abstracciones prematuras (YAGNI violado)
  - Complejidad sin beneficio (r2.0 no pedido)
  - Testing complejo (mocks para todo)
  - Mantenimiento (código no usado que cambia)

Síntoma: Código "por si acaso" (nadie lo pidió)
```

**Resultado**: Complejidad prematura, over-engineering. ❌

---

### Anti-Pattern: Pérdida de Conocimiento (No Documentar)

```
Discovery session → Emergen insights r2.0 (multi-stream)
                  → "YAGNI, no implementemos"
                  → (No documentamos nada)
                  ↓
6 meses después:
  User: "Necesitamos r2.0 multi-stream"
  Arquitecto nuevo: "¿Cómo diseñamos esto?"
  → Reinventa rueda (2 semanas explorando)
  → Puede violar bounded contexts (no conoce decisiones r1.0)
  → Puede cerrar r3.0 (rey ahogado)

Síntoma: Knowledge loss (se pierde el "oro" del discovery)
```

**Resultado**: Reinvención, decisiones subóptimas. ❌

---

### Pattern Correcto: Forecast Arquitectónico

```
Discovery session → Emergen insights r2.0 (multi-stream)
                  → Tests mentales pasan (scale horizontal ✅)
                  ↓
Implementamos: SOLO r1.0 (YAGNI respetado)
Documentamos: PROPOSAL/P001-multi-stream.md (forecast)
                  ↓
6 meses después:
  User: "Necesitamos r2.0 multi-stream"
  Arquitecto: Lee PROPOSAL/P001
  → Opciones ya validadas (30 min, no 2 semanas)
  → Bounded contexts correctos (r1.0 no cierra r2.0)
  → Movilidad preservada (rey no ahogado)

Síntoma: Knowledge preserved, YAGNI respetado ✅
```

**Resultado**: Implementación simple hoy, roadmap claro mañana. ✅

---

## El Principio: Forecast sin Violar YAGNI

### Definición

> **Forecast Arquitectónico = Capturar conocimiento sobre evoluciones futuras validadas por tests mentales,
> pero NO implementadas aún (YAGNI).**

**Componentes**:
1. **Discovery**: Explorar r2.0, r3.0 durante design sessions
2. **Validación**: Tests mentales (scale horizontal, movimientos futuros, bounded contexts)
3. **Documentación**: PROPOSAL (forecast explícito, no código)
4. **Implementación**: SOLO r1.0 (YAGNI respetado)

---

### Por Qué Funciona

**Balance YAGNI vs Forecast**:

```
YAGNI (You Ain't Gonna Need It):
  ✅ No implementar r2.0 hasta que sea pedido
  ❌ NO significa "no pensar en r2.0"

Forecast Arquitectónico:
  ✅ Pensar r2.0 durante r1.0 design (validar bounded contexts)
  ✅ Documentar opciones r2.0 (PROPOSAL)
  ✅ Implementar SOLO r1.0 (código simple)
  ✅ Arquitectura r1.0 permite r2.0 (movilidad preservada)

Balance:
  - Código simple (r1.0 funcional)
  - Arquitectura correcta (r2.0 no requiere refactor)
  - Knowledge preserved (PROPOSAL documenta forecast)
```

**Implicación**: Forecast NO viola YAGNI si NO se implementa prematuramente. ✅

---

## Blues con Guardrails (Confiar en Seniority)

### La Metáfora del Blues

> "El blues es improvisación DENTRO de guardrails (escala pentatónica, progresión I-IV-V).
> Sin guardrails = ruido.
> Con guardrails = arte."

**En arquitectura**:
```
Blues = Explorar r2.0, r3.0 durante discovery (forecast)
Guardrails = Bounded contexts, tests mentales, YAGNI
Arte = Proposals que capturan conocimiento sin sobre-diseñar
```

---

### Cuándo Confiar en Seniority (Tocar Blues)

**Seniors/Principals pueden forecast cuando**:
- ✅ Entienden value proposition (producto = norte)
- ✅ Entienden bounded contexts (arquitectura = rieles)
- ✅ Entienden cadenas (proveedor → nosotros → cliente)
- ✅ Tests mentales pasan (validación objetiva)
- ✅ Implementan SOLO r1.0 (YAGNI respetado)

**Juniors NO deben forecast**:
- ❌ Necesitan estructura (no open exploration)
- ❌ Riesgo de sobre-diseño (sin guardrails internalizados)
- ❌ Deben ejecutar plan conocido (no improvisar)

---

### Seniority vs Sobre-Diseño (Diferencia Clave)

| Aspecto              | Sobre-Diseño                                   | Seniority (Blues con Guardrails)                    |
| -------------------- | ---------------------------------------------- | --------------------------------------------------- |
| **Qué implementamos** | r2.0 "por si acaso"                            | Solo r1.0 (YAGNI)                                   |
| **Código**           | Abstracciones prematuras, código no usado      | Simple, funcional                                   |
| **Documentación**    | No documenta por qué (complejidad oculta)      | PROPOSAL (forecast explícito)                       |
| **Guardrails**       | No tiene (YAGNI violado)                       | Tests mentales, bounded contexts, YAGNI             |
| **Confianza**        | "Adivinar futuro" (sin marco de raciocinio)    | "Forecast desde seniority" (marco validado)         |
| **Value proposition** | Perdido (optimiza por optimizar)               | Claro (product-driven, architecture enables)        |

**Test rápido**:
- ¿Código r2.0 existe? → Sobre-diseño ❌
- ¿PROPOSAL r2.0 existe? → Seniority ✅

---

## Los Tres Ojos (Producto-Arquitectura-Código)

### El Problema: Código como Norte (Incorrecto)

```
❌ Visión incorrecta:
   Código = Norte
   → Optimizamos código (performance, líneas, abstracciones)
   → Perdemos de vista producto (qué resuelve, cómo crece)
   → Perdemos de vista arquitectura (movilidad futura)

Resultado: Over-engineering (optimización sin propósito)
```

---

### El Patrón: Los Tres Ojos

```
✅ Visión correcta:

1. Producto = Norte
   - ¿Qué resuelve? (value proposition)
   - ¿Cómo crece? (r2.0, r3.0 business drivers)
   - ¿Qué satisface? (user need)

2. Arquitectura = Rieles
   - ¿Permite evolución? (movilidad futura)
   - ¿Bounded contexts claros? (responsibilities)
   - ¿Permite r2.0 sin refactor? (scale horizontal)

3. Código = Transporte
   - ¿Implementa r1.0? (funcional)
   - ¿Performance adecuado? (no prematuro)
   - ¿Testeable? (calidad)

Balance: Los tres ojos balanceados (no solo código)
```

---

### Aplicado a Forecast

**Durante discovery**:
```
Ojo 1 (Producto):
  "¿r2.0 multi-stream es business need?"
  → Sí (clientes con N cámaras)
  → Forecast justificado ✅

Ojo 2 (Arquitectura):
  "¿r1.0 bounded contexts permiten r2.0?"
  → Tests mentales pasan (scale horizontal)
  → Arquitectura correcta ✅

Ojo 3 (Código):
  "¿Implementamos r2.0 ahora?"
  → NO (YAGNI)
  → Solo r1.0 (simple) ✅

Resultado: PROPOSAL creado (forecast sin sobre-diseñar)
```

---

## PROPOSAL Lifecycle (Discovery → Forecast → ADR)

### Fases

```
┌─────────────────┐
│ Discovery       │  Emergen insights r2.0
│ Session         │  Tests mentales pasan
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│ PROPOSAL        │  P001-multi-stream.md
│ (Forecast)      │  Status: 🔮 Proposed
└────────┬────────┘  Version: 1.0
         │
         │ (tiempo pasa, r2.0 pedido)
         ▼
┌─────────────────┐
│ PROPOSAL        │  P001 validado/actualizado
│ (Validated)     │  Status: ✅ Validated
└────────┬────────┘  Version: 1.1
         │
         │ (r2.0 implementado)
         ▼
┌─────────────────┐
│ ADR             │  ADR-005-multi-stream.md
│ (Implemented)   │  Status: ✅ Implemented
└─────────────────┘  References: "Based on PROPOSAL/P001"

PROPOSAL/P001:
  Status: 🗄️ Archived (superseded by ADR-005)
  Link: See ADR/005-multi-stream.md
```

---

### PROPOSAL vs ADR

| Aspecto              | ADR (Architecture Decision Record)       | PROPOSAL (Architectural Forecast)                  |
| -------------------- | ---------------------------------------- | -------------------------------------------------- |
| **Status**           | ✅ Implementado (código existe)           | 🔮 Forecast (código NO existe)                     |
| **Cuándo se crea**   | Al implementar decisión                  | Durante discovery (insights emergen)               |
| **Propósito**        | Documentar decisión tomada               | Capturar conocimiento futuro                       |
| **Audiencia**        | Implementadores, mantenedores            | Arquitectos futuros, planificadores                |
| **Contenido**        | Context, Decision, Consequences (real)   | Context, Options, Evolution Paths (hipotético)     |
| **Cambia**           | Raramente (histórico)                    | Puede evolucionar (validaciones, opciones)         |
| **Versionado**       | Inmutable (ADR-001)                      | Evoluciona (P001 v1.0 → v1.1)                      |
| **Referencia código** | Sí (implementación existe)               | No (código futuro)                                 |

---

## Tests Mentales (Durante Discovery)

### Test 1: ¿Implementar o Documentar?

**Pregunta**: "¿Este insight debe convertirse en código o PROPOSAL?"

```
Checklist:
☐ ¿r2.0 pedido explícitamente? (business need)
  → SÍ: Implementar (ADR)
  → NO: Documentar (PROPOSAL)

☐ ¿Tests mentales pasan? (scale horizontal, movimientos futuros)
  → SÍ: PROPOSAL valioso (previene rey ahogado)
  → NO: Descartamos (especulación sin validación)

☐ ¿Múltiples opciones? (>1 movimiento posible)
  → SÍ: PROPOSAL útil (documenta tradeoffs)
  → NO: Descartamos (una sola opción obvia)
```

**Decisión**:
- Implementar: Si r2.0 pedido Y tests pasan
- PROPOSAL: Si r2.0 NO pedido Y tests pasan
- Descartar: Si tests NO pasan (especulación)

---

### Test 2: ¿Sobre-Diseño o Seniority?

**Pregunta**: "¿Estamos forecast con guardrails o sobre-diseñando?"

```
Guardrails presentes:
✅ Bounded contexts claros (certeza de dominio)
✅ Tests mentales pasan (validación objetiva)
✅ Implementamos SOLO r1.0 (YAGNI respetado)
✅ Entendemos value proposition (producto = norte)
✅ Entendemos cadena (proveedor → nosotros → cliente)

→ Seniority (forecast justificado) ✅

Guardrails ausentes:
❌ "Tal vez algún día necesitemos X" (especulación)
❌ Implementamos r2.0 "por si acaso" (YAGNI violado)
❌ Tests mentales fallan (rey ahogado)
❌ No entendemos business driver (código sin propósito)

→ Sobre-diseño (forecast injustificado) ❌
```

---

### Test 3: ¿Los Tres Ojos Balanceados?

**Pregunta**: "¿Estamos considerando producto, arquitectura Y código?"

```
Checklist:
☐ Ojo 1 (Producto): ¿r2.0 es business need validado?
☐ Ojo 2 (Arquitectura): ¿r1.0 bounded contexts permiten r2.0?
☐ Ojo 3 (Código): ¿Implementamos SOLO r1.0? (no r2.0 prematuro)

Si 3 respuestas = SÍ → Los tres ojos balanceados ✅
Si alguna = NO → Falta balance (revisar)
```

---

## Template PROPOSAL

### Estructura

```markdown
# P00X: [Título]

**Status**: 🔮 Proposed | ✅ Validated | 🗄️ Archived
**Version**: 1.0
**Created**: YYYY-MM-DD
**Last Updated**: YYYY-MM-DD
**Target Release**: r2.0 | r3.0 | TBD
**Superseded by**: (si archived) ADR-XXX

---

## Context

### Business Driver
¿Qué problema de negocio resuelve esta evolución?

### Current State (r1.0)
¿Cómo funciona hoy?

### Future Need (r2.0/r3.0)
¿Qué cambio se anticipa?

---

## Evolution Options

### Option A: [Nombre]
**Approach**: Descripción
**Pros**: ✅
**Cons**: ❌
**Complexity**: Low/Medium/High

### Option B: [Nombre]
**Approach**: Descripción
**Pros**: ✅
**Cons**: ❌
**Complexity**: Low/Medium/High

---

## Validation (Tests Mentales)

### Test 1: Scale Horizontal
¿Esta opción escala a r2.0 sin refactor r1.0?

### Test 2: Movimientos Futuros
¿Hay >1 opción para r3.0?

### Test 3: Bounded Contexts
¿Responsabilidades claras? ¿Qué módulos cambian?

---

## Impact Analysis

### Módulos Afectados
- module1: [cambios esperados]
- module2: [cambios esperados]

### Complejidad Agregada
- Low/Medium/High

---

## Decision Checkpoint

**Cuándo implementar**:
- [ ] r2.0 pedido explícitamente (business need)
- [ ] Tests mentales re-validados
- [ ] Opción elegida

**Cuándo NO implementar**:
- [ ] r1.0 aún no en producción (premature)
- [ ] Tests mentales fallan
- [ ] Complejidad > beneficio (YAGNI)

---

## References

- **Discovery Session**: [link]
- **Related ADRs**: ADR-XXX
- **Related Annexes**: ANNEX-001, ANNEX-002
```

---

## Caso de Estudio: FrameSupplier

### P001: Multi-Stream Scale Horizontal (r2.0)

**Context**: Clientes requieren N cámaras por instancia.

**Options**:
- **Option A**: N pipelines independientes (scale horizontal puro) ✅
- **Option B**: 1 pipeline global con stream_id routing ❌
- **Option C**: Hybrid (N pipelines + frame-buffer) ⚠️

**Validation**:
- Test 1 (Scale Horizontal): Option A pass ✅
- Test 2 (Movimientos Futuros): Option A y C preservan ✅
- Test 3 (Bounded Contexts): Option A limpios ✅

**Decision**: PROPOSAL created (Option A recomendado, r2.0 NO implementado aún)

**Resultado**:
- r1.0: Simple, funcional (YAGNI)
- Arquitectura: Permite r2.0 sin refactor (movilidad)
- Knowledge: PROPOSAL documenta opciones (forecast)

---

### P002: Frame-Buffer as Separate Module (r3.0)

**Context**: Heavy workers (VLM) requieren sharing (8GB VRAM).

**Options**:
- **Option A**: Frame-buffer facade (Unix tee philosophy) ✅
- **Option B**: Multiplexer in orchestrator (violates SRP) ❌
- **Option C**: Worker pool (over-engineering) ❌

**Validation**:
- Test 1 (Bounded Contexts): Option A limpio ✅
- Test 2 (Unix Philosophy): Option A es pipe + tee ✅
- Test 3 (Testability): Option A aislación ✅

**Decision**: PROPOSAL created (Option A recomendado, r3.0 NO implementado aún)

**Resultado**:
- r1.0: Sin frame-buffer (YAGNI)
- Arquitectura: framesupplier unchanged (bounded context correcto)
- Knowledge: PROPOSAL documenta módulo futuro (forecast)

---

## Anti-Patterns Comunes

### Anti-Pattern 1: Forecast sin Guardrails

```
❌ Durante discovery:
   "Tal vez algún día necesitemos multi-tenant"
   → Crea PROPOSAL/P003-multi-tenant.md (especulación)

Problema:
  - No hay business driver (nadie lo pidió)
  - Tests mentales NO aplicados (no validado)
  - Especulación pura (no seniority)

Síntoma: Proposals sin validación (ruido, no señal)
```

**Solución**: Solo crear PROPOSAL si tests mentales pasan. ✅

---

### Anti-Pattern 2: Implementar Forecast

```
❌ Discovery → PROPOSAL/P001 creado
              → "¡Es tan claro! Implementemos Option A"
              → Código r2.0 (abstracciones, interfaces)

Problema:
  - YAGNI violado (r2.0 no pedido)
  - Complejidad prematura (código no usado)
  - Testing overhead (mocks para r2.0)

Síntoma: Código "por si acaso" (forecast mal aplicado)
```

**Solución**: PROPOSAL = forecast, NO implementación. ✅

---

### Anti-Pattern 3: PROPOSAL como Wishlist

```
❌ PROPOSAL/P005-blockchain-integration.md
   PROPOSAL/P006-quantum-computing.md
   PROPOSAL/P007-AI-powered-orchestration.md

Problema:
  - No emergen de discovery (wishlist, no insights)
  - No validados con tests mentales (especulación)
  - No relacionados con value proposition (feature creep)

Síntoma: Proposals desconectados de negocio (ruido)
```

**Solución**: PROPOSAL solo si emerge de discovery Y tests pasan. ✅

---

### Anti-Pattern 4: No Documentar Forecast

```
❌ Discovery → Insights r2.0 emergen
              → "YAGNI, no implementemos"
              → (No documenta nada)

6 meses después:
  → Arquitecto nuevo reinventa (knowledge loss)

Problema:
  - Knowledge evaporated (se perdió el oro)
  - Reinvención (2 semanas explorando)
  - Puede violar bounded contexts (no conoce r1.0)

Síntoma: Reinvención, decisiones subóptimas
```

**Solución**: Documentar forecast en PROPOSAL (preserve knowledge). ✅

---

## Checklist (Durante Discovery)

```
☐ 1. Identificar insight futuro (r2.0, r3.0)
☐ 2. Validar business driver (¿es need real?)
☐ 3. Test 1: ¿Scale horizontal? (r1.0 permite r2.0 sin refactor)
☐ 4. Test 2: ¿Movimientos futuros? (>1 opción para r3.0)
☐ 5. Test 3: ¿Bounded contexts claros? (responsibilities)
☐ 6. Test 4: ¿Los tres ojos balanceados? (producto, arquitectura, código)
☐ 7. Decidir: ¿Implementar (ADR) o Documentar (PROPOSAL)?
☐ 8. Si PROPOSAL: Crear usando template
☐ 9. Si PROPOSAL: Documentar múltiples opciones (tradeoffs)
☐ 10. Implementar SOLO r1.0 (YAGNI respetado)
```

---

## Golden Rules

> **"Forecast arquitectónico captura el oro del discovery sin violar YAGNI."**

**Balance**:
- Implementamos SOLO r1.0 (código simple, funcional)
- Documentamos r2.0/r3.0 (conocimiento preservado)
- Arquitecto futuro lee PROPOSAL (no reinventa rueda)
- Decisiones r1.0 no cierran r2.0 (movilidad preservada)

---

> **"Los tres ojos: Producto = norte, Arquitectura = rieles, Código = transporte."**

**No perder vista**:
- Producto (value proposition, cómo crece)
- Arquitectura (bounded contexts, movilidad)
- Código (implementación simple, funcional)

---

> **"Blues con guardrails: Seniority permite forecast, guardrails previenen sobre-diseño."**

**Guardrails**:
- Bounded contexts claros
- Tests mentales pasan
- YAGNI respetado (solo r1.0 implementado)
- Value proposition entendido

---

## Referencias

- **Protocol**: [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md)
- **Related Annexes**:
  - [ANNEX-001: Thinking in Chains](./ANNEX-001_THINKING_IN_CHAINS.md) (tests mentales)
  - [ANNEX-002: Bounded Contexts](./ANNEX-002_BOUNDED_CONTEXTS.md) (responsibilities)
- **Module**: [modules/framesupplier/docs/PROPOSALS/README.md](../../modules/framesupplier/docs/PROPOSALS/README.md)
- **Examples**:
  - [P001: Multi-Stream](../../modules/framesupplier/docs/PROPOSALS/P001-multi-stream-scale-horizontal.md)
  - [P002: Frame-Buffer](../../modules/framesupplier/docs/PROPOSALS/P002-frame-buffer-as-separate-module.md)

---

**Versión**: 1.0
**Autor**: Pair-discovery session (Ernesto + Claude)
**Fecha**: 2025-11-06
**Status**: 🟢 Activo (patrón validado en FrameSupplier P001, P002)
