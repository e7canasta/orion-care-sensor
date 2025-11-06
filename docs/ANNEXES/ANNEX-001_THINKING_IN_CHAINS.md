# ANNEX-001: Thinking in Chains (Rey No Ahogado)

**Meta-Principio**: Óptimo Local vs Movilidad Futura
**Aka**: "El rey no se nos puede ahogar"
**Contexto**: Diseño de módulos en arquitecturas evolutivas

---

## El Problema

### Óptimo Local Incorrecto
```
Decisión: "Resolvamos el problema de HOY"
          ↓
r1.0: Feature implementado ✅
          ↓
r2.0: "Necesitamos refactor" ← Rey ahogado
          ↓
r3.0: "No hay movimientos posibles" ← Jaque mate
```

**Síntoma**: Jugada óptima hoy → Sin casilleros mañana.

---

### Óptimo Local Correcto
```
Decisión: "Diseñemos la CADENA, no solo nuestro módulo"
          ↓
r1.0: Bounded contexts claros ✅
          ↓
Test: "¿Multi-X es scale horizontal?" → Sí ✅
Test: "¿Hay movimientos para constraint Y?" → Sí ✅
          ↓
r2.0, r3.0: Otros módulos evolucionan, nosotros estables
```

**Síntoma**: Movilidad preservada → Opciones mañana.

---

## El Principio: Diseñar Cadenas, No Módulos

### Value Stream
```
[Proveedor] → [Nosotros] → [Cliente]
    ↑             ↑            ↑
 Upstream     Bounded      Downstream
             Context
```

**No preguntar solo**:
- ¿Cómo implemento esta feature?

**Preguntar**:
- ¿Qué necesita proveedor de mí?
- ¿Qué necesita cliente de mí?
- ¿Qué ofrezco que SIMPLIFICA la cadena?
- ¿Qué restricciones permiten EVOLUCIÓN?

---

## La Gracia del Óptimo Local

> "La gracia del óptimo local es ganar posición sin hipotecar movilidad."

**Óptimo Local Correcto**:
- ✅ Resuelve el problema de HOY (r1.0 funcional)
- ✅ Preserva movimientos para MAÑANA (r2.0, r3.0 factibles)
- ✅ Bounded contexts claros (certeza de dominio)
- ✅ Tests mentales pasan (scale horizontal, opciones futuras)

**Óptimo Local Incorrecto**:
- ✅ Resuelve el problema de HOY (r1.0 funcional)
- ❌ Cierra opciones para MAÑANA (r2.0 requiere refactor)
- ❌ Acoplamiento en la cadena (proveedor/cliente adivinan)
- ❌ Tests mentales fallan (no escala, rey ahogado)

---

## Tests Mentales (Durante Discovery)

### Test 1: Scale Horizontal
**Pregunta**: "¿r2.0 (multi-X) es instanciar N veces sin refactor?"

```go
// r1.0
module1 := New(config1)

// r2.0
module1 := New(config1)
module2 := New(config2)
module3 := New(config3)
// ¿Alguna dependencia entre instancias?
```

**Si NO** → Bounded context incorrecto (rediseñar).
**Si SÍ** → Movimiento 1 preservado ✅

---

### Test 2: Movimientos Futuros
**Pregunta**: "Si aparece constraint X, ¿hay >1 solución sin refactor nuestro?"

Ejemplo (FrameSupplier + Heavy Workers):
```
Constraint: Heavy worker compartido (8GB VRAM, 1 instancia)

Movimientos posibles:
- Opción A: Multiplexer (orchestrator hace scheduling)
- Opción B: Frame-buffer (facade entre suppliers y worker)
- Opción C: Worker pool (workers se auto-asignan)

¿Alguna requiere refactor de FrameSupplier?
```

**Si NO hay opciones** → Rey ahogado (rediseñar).
**Si SÍ (>1 opción)** → Movilidad preservada ✅

---

### Test 3: Estabilidad de Módulo
**Pregunta**: "De patineta a avión, ¿cuántas líneas cambian?"

```
r1.0 → r2.0 → r3.0 → r4.0
  ↓      ↓      ↓      ↓
Cambios en nuestro módulo:
- 0 líneas: ✅✅✅ (cadena perfecta)
- <50 líneas: ✅✅ (extension point)
- <200 líneas: ✅ (refactor menor)
- >200 líneas: ❌ (cadena mal diseñada)
```

**Si >200 o refactor arquitectónico** → Cadena mal diseñada (rediseñar).

---

## ADRs como Contratos de Cadena

### Estructura Tradicional (Módulo-Céntrica)
```markdown
## ADR-XXX: [Decisión]

Context: Problema que resolvemos
Decision: Qué hacemos
Consequences: Pros/cons
```

**Útil**, pero no diseña la cadena.

---

### Estructura v2 (Cadena-Céntrica)
```markdown
## ADR-XXX: [Decisión]

### Context
Value Stream: [Proveedor] → [Nosotros] → [Cliente]
Problema en la cadena: ...

### Decision
Qué hacemos nosotros (implementación)

### Consequences

#### Para Proveedor (Upstream)
- **Compromisos**: Qué DEBE hacer proveedor
- **Libertades**: Qué PUEDE asumir (simplificaciones)
- **Evolución r2.0**: Cómo puede escalar sin tocarnos

#### Para Nosotros (Bounded Context)
- **Responsabilidades**: Qué garantizamos
- **No-Responsabilidades**: Qué NO hacemos
- **Invariantes**: Qué NUNCA cambiará

#### Para Cliente (Downstream)
- **Compromisos**: Qué DEBE hacer cliente
- **Libertades**: Qué PUEDE asumir
- **Evolución r2.0**: Cómo puede escalar sin tocarnos

### Future Evolution Paths
- r2.0: Multi-X → Cambios en [módulo Y], nosotros estables
- r3.0: Constraint Z → Opciones: [A, B, C]
```

---

## Ejemplo Completo: FrameSupplier Evolution

### Evolución Multi-Release

```
r1.0: Single stream, local workers
FrameSupplier: 500 líneas

r2.0: Multi-stream (4 cámaras)
Cambios:
- stream-capture: Instanciar N veces
- FrameSupplier: Instanciar N veces (0 líneas código)
- workers: Suscribirse a supplier correcto
- orquestador: Gestionar N pipelines

r3.0: Heavy workers compartidos (VLM, SAM, YOLO-XL)
Cambios:
- stream-capture: Sin cambios
- FrameSupplier: Sin cambios (0 líneas)
- workers: Introducir frame-buffer (facade)
- orquestador: Sin cambios

r3.5: Priority-based distribution
Cambios:
- stream-capture: Sin cambios
- FrameSupplier: Drop policy pluggable (+50 líneas)
- workers: Declarar SLA en Subscribe
- orquestador: Configurar SLAs

r4.0: Distributed workers (networked)
Cambios:
- stream-capture: Sin cambios
- FrameSupplier: Sin cambios (0 líneas)
- workers: RPC client wrapper
- orquestador: Service discovery
```

**De patineta a avión: FrameSupplier cambió <10%.**

**¿Por qué?** Cadena bien diseñada (ADRs pensaron en proveedor/cliente desde r1.0).

---

## Cadena de Pensamiento (Meta-Pensamiento)

### Caso: ¿Cómo pensar multi-stream en r1.0?

```
r1.0 Design:
"¿Frame-supplier tiene bounded context claro?"
  ↓
Test 1: "¿Multi-stream es scale horizontal?"
  ↓ (pensamiento)
  PS = {PS_1, PS_2, ..., PS_N}
  PS_i = stream-capture(si) + frame-supplier(si) + worker-orchestrator(si)
  ¿Hay dependencias entre PS_i y PS_j?
  ↓
  NO → r2.0 factible sin refactor ✅
  ↓
Test 2: "¿Heavy workers compartidos tienen movimientos posibles?"
  ↓ (pensamiento)
  VLM (8GB) compartido entre 4 streams
  Opciones:
    A) Multiplexer (orchestrator scheduling)
    B) Frame-buffer (facade N→1)
    C) Worker pool (auto-asignación)
  ¿Alguna requiere refactor de frame-supplier?
  ↓
  NO (todas usan interfaz existente) → r3.0 tiene opciones ✅
  ↓
Conclusión: Bounded contexts correctos → Rey no ahogado
```

**NO necesitamos**:
- ❌ Predecir que r3.0 será frame-buffer exacto
- ❌ Implementar multiplexer en r1.0 "por si acaso"

**SÍ necesitamos**:
- ✅ Bounded contexts claros (frame-supplier = distribution, punto)
- ✅ Validar movimientos posibles (test mental, no implementación)
- ✅ Documentar opciones en ADRs (conocimiento futuro)

---

## La Analogía Ajedrez/Patineta

### Rey Ahogado (Ajedrez)
```
Movida óptima → Gana pieza
              → Pero rey sin casilleros
              → Ahogado (empate o derrota)
```

**En software**:
```
Feature óptima hoy → Funciona en r1.0
                   → Pero sin movimientos para r2.0
                   → Refactor (empate) o rewrite (derrota)
```

---

### Patineta → Avión (MVP Correcto)

**MVP Incorrecto** (rueda → volante → motor):
```
User: "Quiero moverme"
Dev: "Ok, te doy una rueda" (no se puede mover)
     "Ahora un volante" (sigue sin moverse)
     "Ahora un motor" (sigue sin moverse)

Problema: MVPs no son funcionales hasta el final
```

**MVP Correcto** (patineta → bici → auto):
```
User: "Quiero moverme"
Dev: "Ok, te doy patineta" (se mueve, funcional)
     "Ahora bici" (se mueve mejor)
     "Ahora auto" (se mueve mejor aún)

Beneficio: Cada release es funcional Y preserva movilidad
```

**En arquitectura**:
- Patineta = r1.0 (funcional, simple)
- Bici = r2.0 (funcional, más features)
- Auto = r3.0 (funcional, optimizado)

**Clave**: Cada release funciona Y permite siguiente release sin refactor.

---

## Conocimiento Futuro (Beneficio Clave)

### ADRs Generan Previsión Arquitectónica

```
ADR-004 (hoy) documenta:
- Cómo funciona r1.0 (presente)
- Cómo escala r2.0 (multi-stream)
- Opciones para r3.0 (heavy workers)
- Marco de foco (qué módulo evoluciona)
```

**Resultado**:
- ✅ Arquitecto futuro: Lee ADR, sabe dónde tocar (no adivina)
- ✅ Claude futuro: Lee ADR, propone según evolución prevista
- ✅ Team: Certeza de roadmap técnico (no sorpresas)

### Dos Reglas y Media

#### Regla 1: Pensar la Cadena Completa
> "Aunque estés en locks y punteros, pensá value proposition del sistema"

- Diseñás módulo → Pensás proveedor/cliente
- Elegís sync.Cond → Simplifica stream-capture (no backpressure)
- Elegís blocking consume → Simplifica workers (no polling)
- → Complejidad distribuida óptimamente

#### Regla 2: ADRs Diseñan la Cadena
> "ADR = Contrato de cadena + Directrices para vecinos"

- Proveedor: "Te entrego ownership, vos tolerás drops"
- Nosotros: "Te garantizo non-blocking, no gestiono backpressure"
- Cliente: "Consumo JIT, tolero blocking"
- → Cada uno sabe su rol

#### Regla 2.5: Genera Conocimiento Futuro
> "ADR bien escrito → Previsión arquitectónica"

- Documenta presente (cómo integrarse)
- Documenta futuro (cómo evolucionar)
- Documenta foco (qué módulo cambia en cada release)
- → Roadmap técnico sin sorpresas

---

## Balance: YAGNI vs Movilidad

### ❌ NO Hacer (Sobre-diseño)
```
"Implementemos multi-stream en r1.0 por si acaso"
"Implementemos frame-buffer ahora (no lo necesitamos)"
"Abstraigamos TODO con interfaces (might need flexibility)"
```
→ YAGNI violado, complejidad prematura

### ✅ SÍ Hacer (Movilidad preservada)
```
"Diseñemos bounded contexts para que multi-stream sea scale horizontal"
"Documentemos opciones para heavy workers (frame-buffer, multiplexer)"
"Implementemos SOLO r1.0, validemos tests mentales"
```
→ YAGNI respetado, movilidad garantizada

---

## Checklist (Durante Discovery)

```
☐ 1. Identificar value stream: [Proveedor] → [Nosotros] → [Cliente]
☐ 2. Test 1: ¿Multi-X es scale horizontal?
☐ 3. Test 2: ¿Hay >1 movimiento para constraints futuros?
☐ 4. Test 3: ¿De patineta a avión, cambios <10%?
☐ 5. ADR documenta: Compromisos/Libertades para proveedor
☐ 6. ADR documenta: Compromisos/Libertades para cliente
☐ 7. ADR documenta: Invariantes (qué NUNCA cambiará)
☐ 8. ADR documenta: Evoluciones futuras (r2.0, r3.0, opciones)
☐ 9. Validar: ¿Implementamos solo r1.0? (YAGNI)
☐ 10. Validar: ¿Documentamos r2.0, r3.0? (Movilidad)
```

---

## Aplicación: FrameSupplier ADR-004

### Antes (Módulo-Céntrico)
```markdown
ADR-004: Symmetric JIT
- Inbox mailbox con overwrite
- Non-blocking Publish
- Blocking Subscribe
```

### Después (Cadena-Céntrica)
```markdown
ADR-004: Symmetric JIT Architecture

Value Stream:
[stream-capture] → [FrameSupplier] → [workers]

Para stream-capture:
- Compromisos: Entregar ownership, tolerar drops
- Libertades: Fire-and-forget, no backpressure logic
- r2.0: Instanciar N veces (sin refactor)

Para FrameSupplier:
- Responsabilidades: Non-blocking Publish, blocking Subscribe
- No-Responsabilidades: Backpressure a proveedor, retry a cliente
- Invariantes: Drop > Queue (NUNCA cambiará)

Para workers:
- Compromisos: Consumir JIT, tolerar blocking
- Libertades: Asumir orden, interfaz simple
- r3.0: frame-buffer facade (sin refactor nuestro)
```

**Diferencia**: Cadena completa documentada, evoluciones previstas.

---

## Referencias

- **Lean/Toyota**: JIT, conocer clientes y proveedores
- **Protocol**: [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md)
- **Context**: [CLAUDE_CONTEXT.md](../../CLAUDE_CONTEXT.md)
- **Module**: [modules/framesupplier/CLAUDE.md](../../modules/framesupplier/CLAUDE.md)

---

**Versión**: 1.0
**Autor**: Pair-discovery session (Ernesto + Claude)
**Fecha**: 2025-11-06
**Status**: 🟢 Activo (patrón validado en FrameSupplier)
