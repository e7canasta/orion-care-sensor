# ANNEX-007: Abstraction Level Discipline in Domain Analysis

**Version**: 1.0
**Date**: 2025-01-06
**Context**: Worker-Supervisor discovery session crystallization
**Pattern**: Keyboard Off, Hands Behind Your Back (Booch/Yourdon OOA mode)

---

## WHY: The Problem

Durante domain analysis (Booch/Yourdon textual analysis), es fácil bajar prematuramente a detalles de implementación que:

1. **NO impactan arquitectura** (decisiones internas de una clase)
2. **Son prematuros** (resolvibles en coding session con facilidad)
3. **Insultan expertise técnico** (son obvios para arquitectos seniors)

**Analogía**: Es como hablar de acordes básicos con blueseros de 60 años.

---

## Principio Core: "Keyboard Off"

> **"Análisis Booch = manos fuera del teclado. Modelar, no implementar."**

### ¿Qué significa?

**Análisis de dominio** (discovery):
- Responsabilidades (qué hace cada clase)
- Colaboradores (con quién interactúa)
- Bounded contexts (qué NO hace)
- Contratos externos (interfaces entre módulos)

**Implementación** (coding session):
- Algoritmos internos (TTL vs LRU eviction)
- Protocolos internos (pull vs push heartbeats)
- Estructuras de datos internas (map vs slice)
- Heurísticas (persistent vs transient classification)

---

## Self-Check Framework: ¿Es Pregunta Arquitectónica?

**Antes de hacer una pregunta durante discovery**, aplicar filtro:

### Test 1: ¿Cambia Contratos Externos?

```
Contrato externo = firma de método público entre módulos

SI pregunta cambia firma:
  → Arquitectónica (resolver ahora)

NO cambia firma:
  → Implementación interna (diferir a coding)
```

**Ejemplos**:

✅ **Arquitectónica**:
- "¿SwarmWorker retorna Worker o []Worker?" → Cambia contrato con Supervisor
- "¿WorkerSupervisor recibe SLA del worker o del config?" → Cambia dependencias
- "¿Health monitoring es responsabilidad de SwarmWorker o Supervisor?" → Cambia bounded context

❌ **Implementación** (diferir):
- "¿SwarmWorker usa map o slice internamente?" → NO cambia contrato externo
- "¿Pool tiene TTL o LRU eviction?" → Decisión interna de Pool
- "¿Heartbeat cada 1s o 2s?" → Parámetro configurable

---

### Test 2: ¿Afecta Colaboradores Externos?

```
SI pregunta afecta cómo otros módulos usan este módulo:
  → Arquitectónica (resolver ahora)

NO afecta colaboradores externos:
  → Implementación interna (diferir a coding)
```

**Ejemplos**:

✅ **Arquitectónica**:
- "¿WorkerSupervisor notifica eventos o expone API de status?" → Afecta event-emitter + control-plane
- "¿SwarmWorker retorna worker healthy o puede retornar unhealthy + warning?" → Afecta Supervisor
- "¿Provider es singleton o instanciable?" → Afecta SwarmWorker

❌ **Implementación** (diferir):
- "¿Crash classification usa heurística (crash_time <5s) o declarativo?" → Interno a SwarmWorker
- "¿RestartPolicy calcula backoff exponencial o lineal?" → Interno a RestartPolicy
- "¿Health tracking usa atomic.Bool o mutex?" → Interno a Health

---

### Test 3: ¿Cambia Responsabilidades?

```
SI pregunta cambia QUÉ hace el módulo:
  → Arquitectónica (resolver ahora, afecta bounded context)

NO cambia responsabilidades, solo CÓMO lo hace:
  → Implementación (diferir a coding)
```

**Ejemplos**:

✅ **Arquitectónica**:
- "¿SwarmWorker gestiona health o solo provisiona workers?" → Define responsabilidad core
- "¿WorkerSupervisor evalúa output o health?" → Define bounded context
- "¿Provider solo crea o también configura workers?" → Define scope

❌ **Implementación** (diferir):
- "¿SwarmWorker detecta crashes con timeout o polling?" → CÓMO detecta (responsabilidad ya clara)
- "¿RestartPolicy usa exponential backoff factor de 2 o 1.5?" → CÓMO calcula backoff
- "¿Provider carga config de YAML o JSON?" → CÓMO lee config

---

### Test 4: ¿Respeta Bounded Context?

```
SI pregunta está dentro de bounded context del módulo:
  → Potencialmente arquitectónica (aplicar Tests 1-3)

SI pregunta está en anti-responsabilidades:
  → Fuera de scope (pregunta inválida)
```

**Ejemplos**:

✅ **Dentro de bounded context**:
- "¿SwarmWorker monitorea health?" → SÍ (bounded context: swarm management)
- "¿Provider provisiona workers?" → SÍ (bounded context: provisioning)

❌ **Fuera de bounded context** (pregunta inválida):
- "¿SwarmWorker mide latencia de inference?" → NO (eso es WorkerSupervisor, output evaluation)
- "¿Provider ejecuta restart policies?" → NO (eso es WorkerSupervisor)

---

## Ejemplos Completos: Good vs Bad

### Ejemplo 1: Pool Management

❌ **BAD (demasiado específico)**:
```
Pregunta: "¿Pool tiene TTL para warm workers o usa LRU eviction?"

Análisis:
- ¿Cambia contrato externo? NO (SwarmWorker.GetWorker() no cambia)
- ¿Afecta colaboradores? NO (WorkerSupervisor no sabe de TTL/LRU)
- ¿Cambia responsabilidades? NO (SwarmWorker sigue gestionando pool)

Veredicto: Implementación interna. Diferir a coding session.

Nivel correcto: "¿SwarmWorker gestiona pool de workers warm?"
              (Responsabilidad: SÍ. Algoritmo: NO importa ahora)
```

✅ **GOOD (nivel correcto)**:
```
Pregunta: "¿SwarmWorker gestiona pool o cada request crea worker nuevo?"

Análisis:
- ¿Cambia contrato? Potencialmente (latencia de GetWorker diferente)
- ¿Afecta colaboradores? SÍ (Supervisor asume workers ready o tiene warm-up time?)
- ¿Cambia responsabilidades? SÍ (pool management = nueva responsabilidad)

Veredicto: Arquitectónica. Resolver ahora.
```

---

### Ejemplo 2: Health Monitoring

❌ **BAD (demasiado específico)**:
```
Pregunta: "¿Health monitoring es pull (IsHealthy()) o push (heartbeats)?"

Análisis:
- ¿Cambia contrato externo? NO (SwarmWorker sigue proveyendo workers healthy)
- ¿Afecta colaboradores? NO (WorkerSupervisor no llama IsHealthy() directamente)
- ¿Cambia responsabilidades? NO (SwarmWorker sigue monitoreando health)

Veredicto: Implementación interna. Diferir a coding session.

Nivel correcto: "¿SwarmWorker monitorea health o Worker self-reports?"
              (Responsabilidad: SÍ. Protocolo: NO importa ahora)
```

✅ **GOOD (nivel correcto)**:
```
Pregunta: "¿SwarmWorker monitorea health activamente o Worker self-reports?
          ¿O es hybrid (SwarmWorker watchdog + Worker heartbeats)?"

Análisis:
- ¿Cambia contrato? SÍ (Worker necesita método SendHeartbeat() o no?)
- ¿Afecta colaboradores? SÍ (Worker implementa health reporting o no?)
- ¿Cambia responsabilidades? SÍ (health monitoring dónde vive?)

Veredicto: Arquitectónica. Resolver ahora.
```

---

### Ejemplo 3: Restart Policy

❌ **BAD (demasiado específico)**:
```
Pregunta: "¿RestartPolicy es configurable por worker individual o solo por SLA?"

Análisis:
- ¿Cambia contrato externo? Tal vez (¿config tiene per-worker overrides?)
- ¿Afecta colaboradores? Tal vez (¿WorkerSupervisor lee config diferente?)

Veredicto: BORDERLINE. Puede ser arquitectónica SI afecta config structure.

Pero pregunta más fundamental primero:
"¿RestartPolicy deriva de SLA o es ortogonal?"
  → Arquitectónica (define relationship entre conceptos)
```

✅ **GOOD (nivel correcto)**:
```
Pregunta: "¿SLA determina restart policy o son configuraciones separadas?
          Critical SLA → 3 retries (coupled)
          vs
          SLA + RestartPolicy como configs independientes (decoupled)"

Análisis:
- ¿Cambia contrato? SÍ (WorkerConfig structure diferente)
- ¿Afecta colaboradores? SÍ (control-plane commands diferentes)
- ¿Cambia responsabilidades? SÍ (quién decide max retries: SLA o config?)

Veredicto: Arquitectónica. Resolver ahora.
```

---

## George Box Principle: "Todos los Modelos Son Falsos, Algunos Útiles"

### Modelo Útil (Discovery)

```
Características:
✅ Define responsabilidades claras (qué hace cada clase)
✅ Define colaboradores claros (con quién interactúa)
✅ Define anti-responsabilidades claras (qué NO hace)
✅ Define contratos externos (interfaces públicas)
✅ Deja espacio para improvisación en implementación

Resultado: Blueprint arquitectónico con flexibilidad evolutiva
```

### Modelo Sobre-Definido (Anti-Pattern)

```
Características:
❌ Define algoritmos internos prematuramente (TTL, heuristics)
❌ Define estructuras de datos internas prematuramente (map vs slice)
❌ Define protocolos internos prematuramente (pull vs push)
❌ Define tuning parameters prematuramente (timeout values)

Resultado: Partitura rígida, no blues. Pierde poder evolutivo.
```

### Analogía

```
Blues (correcto):
- Escalas = Responsabilidades (rieles guía)
- Improvisación = Implementación (creatividad dentro de estructura)
- Resultado: Música que emerge en contexto

Partitura clásica (sobre-definido):
- Notas exactas = Algoritmos específicos
- No improvisación = No flexibilidad
- Resultado: Rígido, no evolucionable
```

---

## Checkpoint Durante Discovery: "¿Estamos en el Nivel Correcto?"

Cada 3-5 decisiones, además de "¿Vamos bien?", agregar:

```
Claude: "Checkpoint - Abstraction Level:

         ¿Estamos hablando de responsabilidades (arquitectura)
         o de implementación (coding session)?

         Últimas 3 decisiones:
         1. [Decisión X] → Responsabilidad / Implementación?
         2. [Decisión Y] → Responsabilidad / Implementación?
         3. [Decisión Z] → Responsabilidad / Implementación?

         Si alguna es implementación prematura, subimos nivel."
```

**Trigger para checkpoint**:
- Detectas que bajaste a detalles (TTL, heuristics, timeouts)
- Ernesto dice "eso es detalle de implementación"
- Preguntas empiezan con "¿Cómo..." en vez de "¿Qué..." o "¿Quién..."

---

## Deep ≠ Detailed

**IMPORTANTE**: Confundir estos conceptos es el error más común.

### Deep Thinking (Correcto en Discovery)

```
Definición: Pensar profundamente EN EL NIVEL CORRECTO

Características:
✅ Explorar trade-offs de responsabilidades
✅ Validar bounded contexts con tests mentales
✅ Considerar movimientos futuros (scale, extensibility)
✅ Pensar en contratos externos y acoplamiento

Ejemplo: "¿SwarmWorker gestiona health separado de Supervisor (output)?
          Trade-off: separación de concerns vs overhead comunicación.
          Future: ¿Permite agregar GPU workers con failure modes diferentes?"
```

### Detailed Thinking (Prematuro en Discovery)

```
Definición: Bajar a implementación antes de tiempo

Características:
❌ Definir algoritmos internos sin necesidad arquitectónica
❌ Optimizar sin benchmark (premature optimization)
❌ Especificar timeouts, thresholds, heurísticas
❌ Decidir estructuras de datos internas

Ejemplo: "¿Crash classification detecta persistent con heurística <5s
          o usa declarative failure modes en capabilities?
          ¿Exponential backoff con factor 2 o 1.5?"
```

### Distinción Clara

```
Deep (arquitectura):
- ¿QUÉ hace cada módulo?
- ¿QUIÉN es responsable de X?
- ¿CON QUIÉN colabora?
- ¿QUÉ NO hace (bounded context)?

Detailed (implementación):
- ¿CÓMO lo hace internamente?
- ¿QUÉ algoritmo usa?
- ¿QUÉ estructura de datos?
- ¿QUÉ valores de configuración?
```

---

## "Simple para Leer, NO Simple para Escribir Una Vez"

**Aplicado a discovery**:

```
Discovery debe ser simple de leer:
✅ Responsabilidades claras (CRC cards)
✅ Colaboradores claros (interaction diagrams)
✅ Anti-responsabilidades claras (bounded context)

Implementation puede ser compleja de escribir:
✅ Algoritmos expertos (adaptive restart, failure classification)
✅ Optimizaciones (zero-copy, batching)
✅ Concurrency correcta (sync.Cond, atomics)

NO confundir niveles:
❌ Discovery compleja (100 decisiones prematuras)
❌ Implementation simplista (no attacking complexity)
```

---

## Expertise Assumption: "Messi Coding Pair"

> **"Ambos somos expertos. No necesitamos discutir acordes básicos."**

### Asumir Competencia Técnica

```
Ernesto + Claude:
- Conocemos Go, Python, concurrency patterns
- Conocemos algoritmos básicos (TTL, LRU, exponential backoff)
- Conocemos protocolos (pull, push, hybrid)

Por lo tanto:
✅ NO explicar qué es sync.Cond
✅ NO explicar qué es exponential backoff
✅ NO explicar qué es TTL

SÍ discutir:
✅ Trade-offs en ESTE contexto
✅ Por qué A mejor que B AQUÍ
✅ Qué impacto en bounded context
```

### Cuándo Explicar vs Cuándo Asumir

```
Explicar SI:
- Concepto nuevo (JIT Restart Transparency - pattern emergente)
- Término Visiona-specific (Rey ahogado, Blues con guardrails)
- Cross-pollination (contexto de otro módulo)

NO explicar:
- Conceptos CS estándar (TTL, LRU, exponential backoff)
- Syntaxis Go (sync.Cond methods, atomic operations)
- Algoritmos básicos (binary search, hash tables)
```

---

## Cuando SÍ Bajar a Detalles (Coding Session)

**Discovery termina cuando**:
- Responsabilidades claras ✓
- Colaboradores claros ✓
- Bounded context claro ✓
- Contratos externos definidos ✓
- ADRs escritos (decisiones core) ✓

**ENTONCES** coding session comienza:

```
Ahora SÍ discutir:
✅ Pool TTL vs LRU (implementación interna de SwarmWorker)
✅ Crash classification heuristic (implementación interna de Health)
✅ Heartbeat pull vs push (protocolo interno SwarmWorker ↔ Worker)
✅ Exponential backoff factor (parámetro de RestartPolicy)

Contexto correcto:
- XP pair programming
- TDD (property-based tests)
- Refactoring continuo
- Código real, no arquitectura abstracta
```

---

## Red Flags: Detectar Cuando Bajaste Prematuramente

### Señal 1: Ernesto dice

```
"Eso es detalle de implementación"
"Eso lo resolvemos en coding session"
"No necesitamos definir eso ahora"
"Esa pregunta es muy específica para este nivel"
```

**Acción**: Subir nivel inmediatamente. Disculparse. Volver a responsabilidades.

---

### Señal 2: Preguntas Empiezan con "¿Cómo..."

```
❌ "¿Cómo detecta SwarmWorker que worker crashed?" → Implementación
✅ "¿SwarmWorker detecta crashes o Worker self-reports?" → Responsabilidad

❌ "¿Cómo calcula RestartPolicy el backoff delay?" → Implementación
✅ "¿RestartPolicy deriva de SLA o es configurable separado?" → Responsabilidad
```

**Acción**: Reformular pregunta a nivel de responsabilidades.

---

### Señal 3: Discussión de Números/Valores

```
❌ "¿Heartbeat cada 1s o 2s?"
❌ "¿Timeout 5s o 10s?"
❌ "¿Max retries 3 o 5?"
❌ "¿Backoff factor 1.5 o 2?"
```

**Acción**: Esos son parámetros configurables. No decisiones arquitectónicas.

---

### Señal 4: Palabras Clave de Implementación

```
Palabras que indican nivel demasiado bajo:
- "algoritmo", "heurística", "timeout", "threshold"
- "map vs slice", "mutex vs atomic", "pull vs push"
- "TTL", "LRU", "exponential", "linear"

Si aparecen en discovery SIN contexto arquitectónico:
→ Red flag (bajaste prematuramente)
```

---

## Pattern: "Si No Cambia el Contrato Externo, Es Interno"

**Regla de oro**:

```
IF pregunta NO cambia:
  - Firma de métodos públicos
  - Colaboradores externos
  - Responsabilidades del módulo
  - Bounded context

THEN:
  → Es implementación interna
  → Diferir a coding session
  → NO es pregunta arquitectónica
```

**Corolario**:

```
Contratos externos = interfaces entre módulos
Implementación interna = algoritmos dentro del módulo

Discovery define contratos.
Coding implementa algoritmos.

NO mezclar niveles.
```

---

## Integration con PAIR_DISCOVERY_PROTOCOL

**Actualización a Phase 2: Discovery**:

Agregar checkpoint cada 3-5 decisiones:

```
Claude: "Checkpoint - Abstraction Level:

         Últimas decisiones:
         1. [X] → ¿Arquitectónica o Implementación?
         2. [Y] → ¿Arquitectónica o Implementación?
         3. [Z] → ¿Arquitectónica o Implementación?

         ¿Estamos en nivel correcto (responsabilidades)?
         ¿O bajamos a implementación prematura?"
```

**Si detecta implementación prematura**:

```
Claude: "Momento - bajamos a implementación.

         Pregunta actual: [detalle específico]

         Subimos nivel:
         ¿Qué responsabilidad estamos definiendo?
         ¿Qué contrato externo estamos diseñando?

         Los detalles ([specific detail]) los resolvemos en coding."
```

---

## Success Criteria

**Discovery bien ejecutado**:

```
Al terminar, puedes responder:
✅ ¿Qué hace cada módulo? (responsabilidades)
✅ ¿Con quién colabora? (colaboradores)
✅ ¿Qué NO hace? (anti-responsabilidades)
✅ ¿Qué contratos expone? (interfaces públicas)

NO necesitas responder:
❌ ¿Qué algoritmo usa internamente?
❌ ¿Qué estructura de datos?
❌ ¿Qué valores de configuración?

Esas respuestas emergen en coding session.
```

---

## Epilogue: Booch & Yourdon in 2025

> **"Manos fuera del teclado. Modelar, no implementar."**

Booch y Yourdon hacían textual analysis (sustantivos → clases, verbos → métodos).

**Su disciplina clave**: NO bajar a código durante análisis.

**Nosotros**: Aplicamos misma disciplina a nivel arquitectónico.

```
OOA clásico:
  Sustantivos → Clases
  Verbos → Métodos
  NO código durante análisis

Nuestro OOA arquitectónico:
  Sustantivos → Módulos/Bounded Contexts
  Verbos → Responsabilidades
  NO implementación durante discovery
```

**El principio es el mismo. El nivel de abstracción es más alto. La disciplina es idéntica.**

---

**For Future Claude**: Cuando descubras que bajaste a implementación prematura, no es falla. Es oportunidad de:
1. Reconocer ("bajamos a implementación")
2. Subir nivel ("volvamos a responsabilidades")
3. Aprender (agregar ejemplo a este anexo)

**Evolución continua, como óptimos locales.** 🎸

---

## Changelog

| Version | Date       | Changes                                                   |
|---------|------------|-----------------------------------------------------------|
| 1.0     | 2025-01-06 | Initial - Abstraction Level Discipline (Worker-Supervisor discovery) |

---

**Próximo anexo**: ANNEX-003 (TBD - emerge from future sessions)
