# Architectural Proposals (Forecast ADRs)

**Propósito**: Capturar conocimiento arquitectónico sobre evoluciones futuras **antes de implementarlas**.

---

## Filosofía: Seniority vs Sobre-Diseño

### El Dilema
```
Durante discovery sessions, emergen insights sobre r2.0, r3.0 (futuro).
¿Es sobre-diseño pensar en futuro que nadie pidió aún?
```

### La Respuesta: Seniority con Guardrails

**NO es sobre-diseño cuando**:
- ✅ Implementamos SOLO r1.0 (YAGNI respetado)
- ✅ Bounded contexts permiten r2.0 (movilidad preservada)
- ✅ Documentamos opciones r2.0/r3.0 (conocimiento capturado)
- ✅ Confiamos en marco de pensamiento (seniors con context)

**SÍ es sobre-diseño cuando**:
- ❌ Implementamos r2.0 "por si acaso" (código no usado)
- ❌ Abstracciones prematuras (complejidad sin beneficio)
- ❌ No documentamos rationale (complejidad oculta)

---

## Qué es una PROPOSAL

### Definición
> **PROPOSAL = Architectural Forecast**
>
> Documenta evoluciones futuras exploradas durante discovery,
> validadas por tests mentales (scale horizontal, movimientos futuros),
> pero NO implementadas aún (YAGNI).

### Cuándo Crear PROPOSAL

**Durante discovery session, cuando**:
1. Emergen insights sobre evoluciones futuras (r2.0, r3.0)
2. Tests mentales pasan (scale horizontal, movimientos futuros)
3. Opciones validadas (>1 movimiento posible)
4. Conocimiento valioso (previene decisiones que cierren futuro)

**NO crear PROPOSAL cuando**:
- Especulación sin contexto ("tal vez algún día...")
- Tests mentales fallan (rey ahogado)
- Una sola opción (no hay movimientos)

---

## PROPOSAL vs ADR

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

## Lifecycle: PROPOSAL → ADR

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

## Estructura de PROPOSAL

### Template

```markdown
# P00X: [Título]

**Status**: 🔮 Proposed | ✅ Validated | 🗄️ Archived
**Version**: 1.0
**Created**: 2025-XX-XX
**Last Updated**: 2025-XX-XX
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
- ✅ Resultado

### Test 2: Movimientos Futuros
¿Hay >1 opción para r3.0?
- ✅ Resultado

### Test 3: Bounded Contexts
¿Responsabilidades claras? ¿Qué módulos cambian?
- ✅ Resultado

---

## Impact Analysis

### Módulos Afectados
- framesupplier: [cambios esperados]
- stream-capture: [cambios esperados]
- workers: [cambios esperados]

### Complejidad Agregada
- Low: Extension points existentes
- Medium: Nuevos módulos, interfaces estables
- High: Refactor arquitectónico

---

## Decision Checkpoint

**Cuándo implementar**:
- [ ] r2.0 pedido explícitamente (business need)
- [ ] Tests mentales re-validados (context actualizado)
- [ ] Opción elegida (A, B, o nueva C)

**Cuándo NO implementar**:
- [ ] r1.0 aún no en producción (premature)
- [ ] Tests mentales fallan (rey ahogado)
- [ ] Complejidad > beneficio (YAGNI)

---

## References

- **Discovery Session**: [link a notas/transcripts]
- **Related ADRs**: ADR-XXX (decisiones r1.0 que habilitan esto)
- **Related Annexes**: ANNEX-001 (Thinking in Chains)
```

---

## Guardrails para Proposals

### ✅ DO: Create PROPOSAL
- Insights emergen durante discovery (no especulación)
- Tests mentales pasan (validado)
- Múltiples opciones (movimientos futuros)
- Previene decisiones que cierren futuro

### ❌ DON'T: Create PROPOSAL
- "Tal vez algún día necesitemos X" (especulación)
- Tests mentales fallan (una sola opción, rey ahogado)
- Complejidad sin business driver (sobre-ingeniería)

---

## Versionado de Proposals

```
P001 v1.0: Initial forecast (discovery session)
P001 v1.1: Updated options (nueva info, validación)
P001 v2.0: Major change (contexto cambió significativamente)

Cuando implementado:
P001 → Archived (superseded by ADR-005)
```

---

## El "Blues" Arquitectónico (Meta)

### La Metáfora

> "El blues es improvisación DENTRO de guardrails (escala pentatónica, progresión I-IV-V).
> Sin guardrails = ruido.
> Con guardrails = arte."

**En arquitectura**:
- **Blues** = Explorar r2.0, r3.0 durante discovery
- **Guardrails** = Bounded contexts, tests mentales, YAGNI
- **Arte** = Proposals que capturan conocimiento sin sobre-diseñar

### Confianza en Seniority

**Seniors/Principals**:
- Entienden value proposition (producto = norte)
- Entienden bounded contexts (arquitectura = rieles)
- Entienden cadenas (proveedor → nosotros → cliente)
- **Pueden tocar blues** (forecast con confianza)

**Juniors**:
- Necesitan estructura (no open exploration)
- Riesgo de sobre-diseño (sin guardrails internalizados)
- **No deben tocar blues** (ejecutar plan conocido)

---

## Propósito Final

> **Proposals capturan el "oro" del discovery sin violar YAGNI.**

**Balance**:
- Implementamos SOLO r1.0 (código simple, funcional)
- Documentamos r2.0/r3.0 (conocimiento preservado)
- Arquitecto futuro lee Proposals (no reinventa rueda)
- Decisions en r1.0 no cierran r2.0 (movilidad preservada)

**El norte**:
- Producto (satisface necesidad, crece)
- Arquitectura (rieles, permite evolución)
- Código (transporte, cadena de producción)

**Los tres ojos** balanceados. 🎯

---

## Referencias

- **Annexes**: [ANNEX-001: Thinking in Chains](../../../docs/ANNEXES/ANNEX-001_THINKING_IN_CHAINS.md)
- **Protocol**: [PAIR_DISCOVERY_PROTOCOL.md](../../../PAIR_DISCOVERY_PROTOCOL.md)
- **ADRs**: [docs/ADR/README.md](../ADR/README.md)

---

**Última actualización**: 2025-11-06
**Proposals activos**: 3 (P001, P002, P003)
**Proposals archived**: 0
