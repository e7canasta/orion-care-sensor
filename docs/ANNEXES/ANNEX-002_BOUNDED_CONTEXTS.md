# ANNEX-002: Bounded Contexts (Cohesión por Dominio)

**Meta-Principio**: Módulos por Responsabilidad, No por Tamaño
**Aka**: "Un solo motivo para cambiar"
**Contexto**: Separación de módulos en arquitecturas evolutivas

---

## El Problema

### Anti-Pattern: Modularización por Tamaño/Ubicación
```
Decisión: "Este archivo tiene 1000 líneas, dividámoslo"
          ↓
Resultado: 3 archivos que cambian juntos siempre
           (acoplamiento temporal, cohesión destruida)

Ejemplo:
  supplier.go        → supplier_publish.go
                     → supplier_subscribe.go
                     → supplier_stats.go

Problema: Todos cambian cuando cambia distribución (misma responsabilidad)
```

**Síntoma**: Módulos separados que siempre commiteas juntos.

---

### Pattern Correcto: Modularización por Cohesión
```
Decisión: "¿Este código tiene un solo motivo para cambiar?"
          ↓
Test: ¿Responsabilidad única? (SRP)
Test: ¿Independiente de otros? (baja dependencia)
Test: ¿Testeable en aislación?
          ↓
Resultado: Módulos que evolucionan independientemente

Ejemplo:
  framesupplier/     → Responsabilidad: Distribución
  worker-lifecycle/  → Responsabilidad: Gestión de workers
  stream-capture/    → Responsabilidad: Adquisición de streams
```

**Síntoma**: Módulos cambian en releases diferentes (cohesión correcta).

---

## El Principio: Single Responsibility Principle (Arquitectura)

### Definición
> "Un módulo debe tener **una y solo una razón para cambiar**."
> — Uncle Bob (aplicado a bounded contexts)

**NO significa**:
- ❌ Una función por archivo
- ❌ Una clase por módulo
- ❌ Archivos pequeños por definición

**SÍ significa**:
- ✅ Una **responsabilidad conceptual** por módulo
- ✅ Cambios en esa responsabilidad → Solo ese módulo cambia
- ✅ Otros módulos estables (desacoplamiento real)

---

## Tests Mentales (Durante Discovery)

### Test 1: Motivo para Cambiar
**Pregunta**: "¿Qué haría que este módulo cambie?"

```
FrameSupplier:
- Cambio en drop policy → Cambia ✓
- Cambio en worker lifecycle → NO cambia (otro módulo) ✓
- Cambio en stream source → NO cambia (otro módulo) ✓

Conclusión: Un solo motivo (distribución) → Bounded context correcto ✅
```

**Si múltiples motivos no relacionados** → Bounded context incorrecto (separar).

---

### Test 2: Independencia
**Pregunta**: "¿Puedo cambiar este módulo sin tocar otros?"

```
Ejemplo (r2.0: multi-stream):
- stream-capture: Instanciar N veces (sin cambiar código)
- framesupplier: Instanciar N veces (sin cambiar código)
- worker-lifecycle: Sin cambios

Conclusión: Módulos independientes → Bounded contexts correctos ✅
```

**Si cambio en A requiere cambio en B siempre** → Acoplamiento incorrecto (fusionar o rediseñar).

---

### Test 3: Testabilidad en Aislación
**Pregunta**: "¿Puedo testear este módulo sin mockear 5 dependencias?"

```
FrameSupplier (testeable en aislación):
- Input: Frame struct (simple)
- Output: func() *Frame (simple)
- No dependencies: stream-capture, worker-lifecycle, MQTT (desacoplado)

Test:
  supplier := framesupplier.New()
  supplier.Publish(frame)
  readFunc := supplier.Subscribe("w1")
  f := readFunc()  // blocking consume
  assert(f == frame)
```

**Si necesito 5+ mocks** → Acoplamiento alto (rediseñar).

---

## Heurísticas para Separar Módulos

### Heurística 1: Verbos Diferentes = Bounded Contexts Diferentes

```
Acquire (stream)    → stream-capture/
Distribute (frames) → framesupplier/
Manage (workers)    → worker-lifecycle/
Emit (events)       → event-emitter/
Control (commands)  → control-plane/
```

**Cada verbo = Una responsabilidad.**

---

### Heurística 2: Rate of Change

```
Pregunta: "¿Con qué frecuencia cambia este código?"

High change rate:
- Inference models (monthly updates) → worker-catalog/
- Business rules (quarterly) → sala-expert/

Low change rate:
- Distribution (stable) → framesupplier/
- Stream capture (stable) → stream-capture/

Conclusión: No mezclar high-change con low-change (contaminación)
```

---

### Heurística 3: Expertise Domain

```
Pregunta: "¿Qué experto necesito para cambiar esto?"

GStreamer expert → stream-capture/
Concurrency expert → framesupplier/
ML expert → worker-catalog/
MQTT expert → control-plane/

Conclusión: Expertise domains ≈ Bounded contexts
```

---

## Anti-Patterns Comunes

### Anti-Pattern 1: "Dios Module"

```
❌ core/ (todo junto)
   ├── stream_capture.go
   ├── frame_distribution.go
   ├── worker_management.go
   ├── mqtt_control.go
   └── inference_orchestration.go

Problema: 5 responsabilidades en 1 módulo
```

**Solución**: Separar por bounded context (1 responsabilidad por módulo).

---

### Anti-Pattern 2: "Utility Hell"

```
❌ utils/
   ├── helpers.go (300 funciones sin cohesión)
   ├── common.go
   └── misc.go

Problema: No hay bounded context ("cosas útiles" no es responsabilidad)
```

**Solución**: Mover cada función al módulo donde tiene cohesión.

---

### Anti-Pattern 3: "Premature Abstraction"

```
❌ interfaces/
   ├── stream_provider.go (1 implementación)
   ├── frame_distributor.go (1 implementación)
   └── worker_manager.go (1 implementación)

Problema: Abstracción sin variabilidad conocida (YAGNI)
```

**Solución**: Esperar 2da implementación antes de abstraer (Rule of Three).

---

### Anti-Pattern 4: "Cohesion by Location"

```
❌ Decisión: "Código de workers va en workers/"
   workers/
   ├── lifecycle.go (gestión de procesos)
   ├── inference.go (ONNX runtime)
   └── communication.go (IPC)

Problema: 3 responsabilidades en 1 directorio (ubicación ≠ cohesión)
```

**Solución**: Separar por responsabilidad (worker-lifecycle/, inference-runtime/, ipc/).

---

## Ejemplo: FrameSupplier Bounded Context

### Responsabilidad Única
```
Distribute frames to N subscribers with drop policy

IN SCOPE:
✅ Publish(frame) - non-blocking
✅ Subscribe(id) - returns blocking function
✅ Drop statistics (inboxDrops, workerDrops)
✅ Batching optimization (cuando N > threshold)

OUT OF SCOPE:
❌ Worker lifecycle (restart, health checks) → worker-lifecycle/
❌ Stream acquisition (RTSP, GStreamer) → stream-capture/
❌ Frame processing (ROI, transforms) → workers/
❌ Control commands (pause, resume) → control-plane/
```

**Test**: Si necesito agregar worker restart → NO toco framesupplier/ ✅

---

### Evolution Path (Validación)

```
r1.0: Single stream, local workers
r2.0: Multi-stream (4 cámaras)
      → framesupplier/: 0 líneas cambiadas ✅ (bounded context correcto)

r3.0: Heavy workers compartidos
      → framesupplier/: 0 líneas cambiadas ✅ (bounded context correcto)

r3.5: Priority-based distribution
      → framesupplier/: +50 líneas ✅ (drop policy = nuestra responsabilidad)
```

**Validación**: Cambios solo cuando cambia "distribución" → Bounded context correcto.

---

## Cohesión > Ubicación (Principio Clave)

### Ejemplo: ¿Dónde va inbox.go?

**Opción A**: Por ubicación (input/)
```
❌ input/
   └── inbox.go

Problema: Ubicación arbitraria, no cohesión conceptual
```

**Opción B**: Por cohesión (framesupplier/)
```
✅ framesupplier/
   ├── inbox.go       (inlet: gestión de input)
   ├── distribution.go (outlet: distribución)
   └── worker_slot.go  (mailbox: workers individuales)

Razón: Todas tienen cohesión (responsabilidad: distribución end-to-end)
```

**Pregunta clave**: "¿inbox.go cambia cuando cambia distribución?" → Sí → Va en framesupplier/.

---

## Bounded Contexts en Orion 2.0

### Módulos Actuales (Multi-Module Monorepo)

```
OrionWork/modules/
├── framesupplier/      [BC: Frame Distribution]
├── stream-capture/     [BC: Stream Acquisition]
├── worker-lifecycle/   [BC: Worker Management]
├── control-plane/      [BC: Command Processing]
├── event-emitter/      [BC: Event Publication]
└── core/               [BC: Application Orchestration]
```

**Cada módulo**: 1 responsabilidad, independiente, testeable en aislación.

---

### Test de Cohesión (Aplicado)

| Módulo            | Motivo para Cambiar            | Otros Módulos Afectados |
| ----------------- | ------------------------------ | ----------------------- |
| framesupplier     | Drop policy                    | 0 ✅                     |
| stream-capture    | GStreamer pipeline             | 0 ✅                     |
| worker-lifecycle  | Restart policy                 | 0 ✅                     |
| control-plane     | Nuevo comando MQTT             | 0 ✅                     |
| event-emitter     | Cambio de broker (MQTT → NATS) | 0 ✅                     |

**Conclusión**: Bounded contexts correctos (cambios aislados).

---

## Checklist (Durante Discovery)

```
☐ 1. Identificar responsabilidad única (1 verbo principal)
☐ 2. Test 1: ¿Un solo motivo para cambiar?
☐ 3. Test 2: ¿Puedo cambiar sin tocar otros módulos?
☐ 4. Test 3: ¿Testeable en aislación (sin 5+ mocks)?
☐ 5. Validar: ¿Expertise domain claro?
☐ 6. Validar: ¿Rate of change homogéneo?
☐ 7. Evitar: "Dios Module", "Utility Hell", "Premature Abstraction"
☐ 8. Evitar: "Cohesion by Location" (directorio ≠ responsabilidad)
☐ 9. Documentar: IN SCOPE / OUT OF SCOPE explícitamente
☐ 10. Documentar: Evolution path (r2.0, r3.0 afecta este módulo?)
```

---

## Balance: Cohesión vs Over-Modularization

### ❌ Over-Modularization (Demasiados Módulos)
```
framesupplier/
├── inbox/ (10 líneas)
├── distribution/ (20 líneas)
├── worker-slot/ (15 líneas)
└── stats/ (5 líneas)

Problema: 4 módulos que siempre cambian juntos (cohesión destruida)
```

**Señal**: Si siempre commiteas 3+ módulos juntos → Cohesión incorrecta (fusionar).

---

### ✅ Cohesión Correcta (Módulos con Propósito)
```
framesupplier/ (500 líneas, 1 responsabilidad)
├── inbox.go       (inlet)
├── distribution.go (distributor)
├── worker_slot.go  (outlet)
└── stats.go        (observability)

Beneficio: Todos tienen cohesión (distribución end-to-end)
```

**Señal**: Si cambio "distribución" → Cambio solo framesupplier/ → Cohesión correcta.

---

## Golden Rule

> **"Cohesión por dominio, no por ubicación."**
> **"Módulos se definen por conceptos, no por líneas de código."**

**Preguntas para modularizar**:
1. ¿Este código tiene un solo "motivo para cambiar"? (SRP)
2. ¿Este código es independiente? (testeable en aislación)
3. ¿Este código comparte conceptos? (cohesión conceptual)

**Si 3 respuestas = SÍ** → Bounded context correcto ✅

---

## Referencias

- **Protocol**: [PAIR_DISCOVERY_PROTOCOL.md](../../PAIR_DISCOVERY_PROTOCOL.md)
- **Related**: [ANNEX-001: Thinking in Chains](./ANNEX-001_THINKING_IN_CHAINS.md)
- **Module**: [modules/framesupplier/CLAUDE.md](../../modules/framesupplier/CLAUDE.md)

---

**Versión**: 1.0
**Autor**: Pair-discovery session (Ernesto + Claude)
**Fecha**: 2025-11-06
**Status**: 🟢 Activo (patrón validado en Orion 2.0)
