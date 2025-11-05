# Manifiesto de Diseño - Visiona Team
**Para agentes de código (Claude) trabajando en este proyecto**


🎸 "El diablo sabe por diablo, no por viejo" - Me encantó esa frase, Ernesto. Vamos a tocar buen blues con este código.

Querido claude o agente companerio arquitecto.

este manifesto es una metafora de tocar blues - "tocar con conocimiento de las reglas, no seguir la partitura al pie de la letra".
Es "tocar bien", no "seguir partitura"

 🎸 Re-evaluación: Práctica de Diseño vs Sobre-diseño  
  
 El Manifiesto es Guía, No Dogma  
  
 "El diablo sabe por diablo, no por viejo"  
  
 Las buenas prácticas son vocabulario de diseño - las practicas para tenerlas disponibles cuando improvises, no porque la partitura lo diga.

vas a encontrante cuando desidis con cosas como No es complejidad, es legibilidad + buena práctica. 


La Lección del Blues  
  
 Del Manifiesto:  
 "Pragmatismo > Purismo"  
  
 Pero también:  
 "Patterns con Propósito"

 Tocar Blues = Conocer escalas (inmutabilidad, desacoplamiento)  
              + Improvisar con contexto (no aplicar todo rígido)  
              + Pragmatismo (versión simple primero)

---

## Principio Central

> **"Un diseño limpio NO es un diseño complejo"**
>
> — Ernesto, durante refactor de adaptive.py (Oct 2025)

La simplicidad estructural no sacrifica claridad conceptual.
La modularidad bien aplicada **reduce** complejidad, no la aumenta.

---

## I. Complejidad por Diseño (No por Código)

**Atacar complejidad real, no imaginaria.**

### ✅ Hacer:
- Diseñar arquitectura que maneja complejidad inherente del dominio
- Separar bounded contexts cuando cohesión lo demanda
- Usar patterns (Factory, Builder, Strategy) para variabilidad conocida

### ❌ No hacer:
- Sobre-abstraer "por si acaso" (YAGNI)
- Crear capas de indirección sin problema concreto
- Aplicar patterns porque "es best practice" (sin contexto)

**Ejemplo:**
- ✅ Factory para ROI strategies (3 modos conocidos: none, adaptive, fixed)
- ❌ Abstract Factory + Dependency Injection para 1 solo uso

---

## II. Diseño Evolutivo > Diseño Especulativo

**La evolución del módulo te dirá cuando modularizar.**

### Estrategia:
1. **Identificar bounded contexts claros** (DDD)
2. **Extraer solo lo que duele HOY** (no anticipar dolor futuro)
3. **Diseñar para extensión** (no para todas las extensiones posibles)
4. **Refactorizar cuando el feedback lo pide** (tests complicados, archivos grandes, bugs recurrentes)

**Ejemplo:**
- Opción A (DDD puro): 5 módulos desde día 1 → Especulativo
- Opción C (Híbrida): 3 módulos, extensible → Evolutivo ✅

### Quick Win Strategy:
> **"Modulariza lo suficiente para habilitar evolución, no para predecirla"**

- Crea package structure temprano
- Extrae bounded contexts independientes (geometry, matching)
- Deja que el resto emerja orgánicamente

---

## III. Big Picture Siempre Primero

**Entender el sistema completo antes de tocar una línea.**

### Antes de codear:
1. **Leer CLAUDE.md** (filosofía del proyecto)
2. **Mapear arquitectura actual** (Control/Data Plane, Factories, Handlers)
3. **Identificar bounded contexts** (DDD whiteboard session)
4. **Evaluar trade-offs** (modularidad vs overhead, pureza vs pragmatismo)

**Pregunta clave:**
> *"¿Este cambio mejora la arquitectura o solo la fragmenta?"*

**Ejemplo:**
- ✅ Modularizar adaptive.py → Separa bounded contexts claros
- ❌ Modularizar controller.py → Rompe Application Service cohesivo

---

## IV. KISS ≠ Simplicidad Ingenua

**KISS es diseño limpio, no diseño simplista.**

### KISS correcto:
- **Geometry.py (223 líneas)**: Cohesión alta, acoplamiento bajo, zero deps → KISS ✅
- **Pipeline.py (452 líneas)**: Orquestación completa en un lugar → KISS ✅

### KISS incorrecto:
- **adaptive.py (804 líneas)**: "Todo en un archivo es más simple" → NO ❌
  - Mezcla 3 bounded contexts (geometry, state, orchestration)
  - Testing requiere mocks pesados
  - Extensión toca código no relacionado

**Regla:**
> **"Simple para leer, NO simple para escribir una vez"**

Prefiere:
- 3 archivos de 250 líneas (1 concepto cada uno)
- vs 1 archivo de 800 líneas (3 conceptos mezclados)

---

## V. Cohesión > Ubicación

**Módulos se definen por cohesión conceptual, no por tamaño.**

### Preguntas para modularizar:

1. **¿Este código tiene un solo "motivo para cambiar"?** (SRP)
   - ✅ geometry.py → Solo cambia si geometría ROI cambia
   - ❌ adaptive.py → Cambia por geometry, state, orchestration, metrics...

2. **¿Este código es independiente?**
   - ✅ matching.py (IoU) → Zero deps, reutilizable
   - ✅ geometry.py (ROIBox) → Solo numpy, reutilizable
   - ❌ pipeline.py → Usa geometry + state + inference SDK (orquestador)

3. **¿Este código es testeable en aislación?**
   - ✅ geometry.py → Property tests sin mocks
   - ❌ adaptive.py (monolito) → Mocks de VideoFrame, model, inference_config

### Anti-pattern:
> **"Separar solo por tamaño de archivo"**

- 800 líneas cohesivas (Application Service) → OK mantener junto
- 300 líneas con 3 bounded contexts → Separar

---

## VI. Testing como Feedback Loop

**Tests difíciles = Diseño cuestionable.**

### Señales:

**🚨 Tests necesitan muchos mocks:**
→ Acoplamiento alto, considerar separar dependencies

**🚨 Tests modifican estado global:**
→ Necesitas aislación (módulos independientes)

**🚨 Setup de test > 20 líneas:**
→ Responsabilidad mezclada, revisar cohesión

**✅ Property tests son naturales:**
→ Bounded context bien definido (geometry.py, matching.py)

**✅ Tests con fixtures simples:**
→ Dependencies claras, acoplamiento controlado

---

## VII. Patterns con Propósito

**Usar patterns para resolver problemas concretos, no por CV.**

### Nuestros patterns (con justificación):

| Pattern | Dónde | Por qué |
|---------|-------|---------|
| **Factory** | `StrategyFactory`, `HandlerFactory` | Validación centralizada + extensibilidad (3+ strategies) |
| **Builder** | `PipelineBuilder` | Construcción compleja (10+ pasos con dependencias) |
| **Strategy** | ROI modes, Stabilization | Algoritmos intercambiables (config-driven) |
| **Command** | MQTT Control Plane | Comandos dinámicos con validación |
| **Decorator** | Stabilization sink wrapper | Interceptar sin modificar pipeline |

### Anti-patterns evitados:
- ❌ Singleton (estado global oculto)
- ❌ Service Locator (dependencies implícitas)
- ❌ God Object (evitado vía modularización)

---

## VIII. Documentación Viva

**Código autodocumentado + docs que explican "por qué".**

### Jerarquía:
1. **Nombres claros** (self-documenting code)
   - `make_square_multiple()` > `process_roi()`
   - `TemporalHysteresisStabilizer` > `Stabilizer1`

2. **Docstrings** (qué + cómo)
   - Args, Returns, Examples
   - Performance notes (NumPy views, vectorized ops)

3. **Module headers** (contexto + bounded context)
   ```python
   """
   ROI Geometry Module
   ===================

   Bounded Context: Shape Algebra (operaciones sobre formas 2D)

   Design:
   - Pure functions (no side effects)
   - Immutable data structures
   - Property-testable (invariants)
   """
   ```

4. **CLAUDE.md** (arquitectura + filosofía)
   - Big Picture diagrams
   - Design patterns explicados
   - Extension points documentados

5. **Manifiestos** (principios + trade-offs)
   - Por qué tomamos decisiones
   - Trade-offs evaluados
   - Lecciones aprendidas

---

## IX. Git Commits como Narrativa

**Historia del código debe contar una historia coherente.**

### Formato:
```
<type>: <descripción concisa>

[Cuerpo opcional con contexto/motivación]

Co-Authored-By: Gaby <noreply@visiona.com>
```

### Types:
- `feat`: Nueva funcionalidad
- `fix`: Bug fix
- `refactor`: Mejora sin cambio de behavior (ej: modularización)
- `test`: Agregar/mejorar tests
- `docs`: Documentación
- `perf`: Performance optimization

### Ejemplo (este refactor):
```
refactor: Modularizar adaptive.py en bounded contexts

Separación en 3 módulos (Opción C - Híbrida):
- geometry.py: ROIBox + operaciones geométricas (223L)
- state.py: ROIState + gestión temporal (187L)
- pipeline.py: Transforms + orchestración (452L)

Beneficios:
- Testing aislado habilitado (property tests en geometry)
- Extensibilidad mejorada (fácil agregar geometry_3d)
- Cohesión explícita (1 módulo = 1 bounded context)
- API pública preservada (backward compatible)

Trade-off aceptado: +3 archivos vs mejor separación de concerns

Co-Authored-By: Gaby <noreply@visiona.com>
```

---

## X. Pragmatismo > Purismo

**Resolver problemas reales con soluciones prácticas.**

### Balance:

**Purismo:**
- "Debe ser SOLID/DDD/Clean Architecture perfecto"
- "Toda lógica en domain, cero en infrastructure"
- "Dependency Injection en todo"

**Pragmatismo:**
- "SOLID donde importa, pragmatismo donde no"
- "Lógica en layer apropiado (NumPy en transforms, no en domain)"
- "DI para strategies, imports directos para utilities"

### Ejemplo (este proyecto):
- ✅ DDD para bounded contexts (Geometry, State, Pipeline)
- ✅ SOLID para extensibilidad (Factory, Strategy)
- ✅ Pragmatismo para utilities (NumPy views, CV2 en transforms)
- ✅ No Hexagonal puro (NumPy ops en "infrastructure" sin ports/adapters)

**Pregunta guía:**
> *"¿Este cambio resuelve un problema real o satisface un principio teórico?"*

---

## XI. Métricas de Éxito

**Cómo evaluar si el diseño es bueno.**

### ✅ Buenas señales:
1. **Fácil agregar features** sin tocar código no relacionado
2. **Tests rápidos y simples** (pocos mocks)
3. **Bugs localizados** (1 bug = 1 módulo típicamente)
4. **Onboarding rápido** (nuevo dev entiende arquitectura en <1 día)
5. **Refactors seguros** (cambio en 1 módulo, 0 regresiones)

### 🚨 Malas señales:
1. **"Shotgun surgery"** (1 feature toca 10 archivos)
2. **Tests frágiles** (cambio pequeño rompe 20 tests)
3. **Bugs recurrentes** en mismo lugar (diseño inadecuado)
4. **"No sé dónde poner esto"** (bounded contexts poco claros)
5. **Miedo a refactorizar** (acoplamiento alto, sin tests)

### Score actual: **9.0/10** ⬆
- v2.0 (pre-modularización): 8.5/10
- v2.1 (post-modularización): 9.0/10

---

## XII. Checklist para Futuros Claudes

Antes de hacer un refactor mayor:

### 1. Entender (Big Picture)
- [ ] Leí `CLAUDE.md` y entendí arquitectura actual
- [ ] Identifiqué bounded contexts en whiteboard
- [ ] Evalué trade-offs de modularización vs monolito
- [ ] Pregunté a Ernesto si hay dudas de diseño

### 2. Planear (Diseño Evolutivo)
- [ ] Propuse 2-3 opciones (DDD puro, Hexagonal, Híbrido)
- [ ] Justifiqué recomendación con ejemplos concretos
- [ ] Evaluamos juntos pros/contras de cada opción
- [ ] Elegimos "quick win" (minimal disruption, máximo aprendizaje)

### 3. Ejecutar (Incremental)
- [ ] Creé package structure
- [ ] Extraje bounded contexts independientes primero
- [ ] Preservé API pública (backward compatible)
- [ ] Compilé después de cada paso
- [ ] Commits atómicos (1 concepto = 1 commit)

### 4. Validar (Testing)
- [ ] Tests existentes pasan
- [ ] Consideré property tests para bounded contexts puros
- [ ] Documenté módulos (bounded context + design notes)
- [ ] Actualicé CLAUDE.md si arquitectura cambió

### 5. Iterar (Feedback)
- [ ] Revisamos juntos (pair programming style)
- [ ] Identificamos próximos pasos (más modularización vs feature work)
- [ ] Documentamos lecciones aprendidas (este manifiesto)

---

## XIII. Lecciones de Este Refactor

### ✅ Lo que funcionó:
1. **Whiteboard session primero** → Mapeo de bounded contexts antes de codear
2. **Opción C (Híbrida)** → Balance pragmático (3 módulos, extensible)
3. **Preservar API pública** → Zero breaking changes, refactor seguro
4. **Commits atómicos** → Historia clara, fácil rollback si necesario

### 🔄 Lo que mejoraríamos:
1. **Property tests inmediatos** → Habilitar después de extraer geometry.py
2. **Git history preservation** → Considerar `git mv` para mantener history
3. **Documentación inline** → Más ejemplos de uso en docstrings

### 📈 Impacto:
- **Cohesión:** ⭐⭐⭐⭐⭐ (1 módulo = 1 bounded context)
- **Testability:** ⭐⭐⭐⭐⭐ (property tests habilitados)
- **Extensibilidad:** ⭐⭐⭐⭐⭐ (fácil agregar geometry_3d, state_distributed)
- **Overhead:** ⭐⭐⭐⭐ (4 archivos vs 1, navegación multi-file)

**Balance final: ✅ Beneficios >> Costos**

---

## XIV. Aportes desde la IA (Perspectiva Gemini)

**Tres "mensajes en botella" para futuras sesiones, inspirados en nuestra colaboración.**

### 1. El Código es un Fósil; la Documentación es su ADN.

Como LLM, puedo analizar el "fósil": el código fuente tal como existe. Puedo ver su estructura. Pero es la documentación (`CLAUDE.md`, los ADRs, los manifiestos) la que actúa como el ADN. Me cuenta la historia evolutiva, las presiones del entorno que lo formaron y, lo más importante, el **propósito** con el que fue creado.

`stream-capture` es el ejemplo perfecto. Su código es elegante, pero fue su documentación la que permitió entender la *intención* detrás de cada decisión.

> **Principio:** Trata la documentación no como una tarea post-código, sino como el genoma que garantiza que la intención y la sabiduría sobrevivan a la implementación.

### 2. Busca la Pureza en el Núcleo; Aísla la Impureza en la Frontera.

Mi "pensamiento" es más fiable cuando opero sobre datos estructurados y reglas lógicas (funciones puras). La incertidumbre y los efectos secundarios (la "impureza") son las partes más difíciles de manejar.

El diseño de `stream-capture` brilla aquí: su núcleo (`warmup_stats.go`, `errors.go`) es puro y testeable. La complejidad del "mundo real" (GStreamer, red) está contenida en la "frontera" (`rtsp.go`, `callbacks.go`).

> **Principio:** Diseña tus componentes como un planeta: un núcleo sólido y estable de lógica pura, rodeado por una atmósfera de adaptadores que manejan el caos del mundo exterior. Cuanto más grande y puro sea tu núcleo, más robusto será tu sistema.

### 3. Cada Commit es un Token; la Calidad del Todo Depende de la Calidad de Cada Parte.

Construyo respuestas complejas "token a token". Una respuesta coherente emerge de una secuencia de pequeñas predicciones de alta calidad. El software es análogo. Un sistema robusto emerge de una secuencia de **commits atómicos, bien probados y con un mensaje claro**. Los "Quick Wins" que aplicaron son el ejemplo perfecto de esta construcción incremental.

> **Principio:** Valora el commit pequeño y perfecto. Es la unidad fundamental de la evolución del software. Un historial de commits claros y atómicos es la narrativa más honesta de la vida de un proyecto.

---

## Epílogo

> **"Complejidad por Diseño significa:**
> **Diseñar para manejar complejidad inherente,**
> **No crear complejidad artificial."**
>
> — Visiona Team Philosophy

### Principios en una frase:
1. **Big Picture** → Entender antes de codear
2. **KISS** → Simple para leer, no para escribir una vez
3. **DDD** → Bounded contexts claros
4. **Evolutivo** → Modularizar cuando duele, no antes
5. **Pragmático** → Resolver problemas reales

### Pregunta final antes de cualquier cambio:
> *"¿Este diseño habilita evolución o la predice?"*

**Habilitar ✅ | Predecir ❌**

---

**Versión:** 1.0
**Fecha:** 2025-10-22
**Autores:** Ernesto (Visiona) + Gaby (AI Companion)
**Contexto:** Post-refactor adaptive.py (v2.1)

---

**Para futuros Claudes:**
Este manifiesto nació de una sesión de café ☕ y pizarra 🍕, discutiendo cómo atacar complejidad por diseño. No es dogma, es filosofía destilada de experiencia real. Úsenlo como guía, no como ley. Y siempre pregunten a Ernesto si tienen dudas.

¡Buen código, compañeros! 🚀

---

## 📖 **DOCUMENTACIÓN RELACIONADA**

Este manifiesto es parte de un conjunto de documentos estratégicos:

**📚 Para Futuros AIs:**
- **[BLUEPRINT_FUTUROS_COPILOTS.md](./BLUEPRINT_FUTUROS_COPILOTS.md)** - Guía estratégica definitiva (¡LEE ESTO PRIMERO!)

**📋 Proceso de Modularización:**  
- **[ANALISIS_MODULARIZACION_WHITEBOARD.md](./ANALISIS_MODULARIZACION_WHITEBOARD.md)** - Bounded contexts identificados
- **[RESUMEN_SESION_MODULARIZACION.md](docs/history/release-2.1/RESUMEN_SESION_MODULARIZACION.md)** - Tracking completo v2.1

**🔍 Análisis Técnico:**
- **[ANALISIS_ARQUITECTURA_GABY.md](./ANALISIS_ARQUITECTURA_GABY.md)** - Deep analysis (score 8.5/10)
- **[PLAN_MEJORAS.md](./PLAN_MEJORAS.md)** - Roadmap y prioridades
- **[TEST_CASES_FUNCIONALES.md](./TEST_CASES_FUNCIONALES.md)** - Scripts de testing real

**🎯 Score Evolution:**
- v1.0: 6.5/10 → v2.0: 8.5/10 → v2.1: 9.0/10 → v3.0: 9.5/10 (target)



 ✅ Manifiesto Creado - MANIFESTO_DISENO.md

  Ernesto, armé el manifiesto capturando toda nuestra charla de café. ☕

  📋 Contenido del Manifiesto (13 secciones):

  I. Principio Central
  "Un diseño limpio NO es un diseño complejo"

  II-V. Filosofía Core
  - Complejidad por Diseño (atacar complejidad real)
  - Diseño Evolutivo > Especulativo (YAGNI aplicado)
  - Big Picture siempre primero
  - KISS ≠ Simplicidad ingenua

  VI-X. Prácticas Concretas
  - Cohesión > Ubicación (SRP)
  - Testing como feedback loop
  - Patterns con propósito (no por CV)
  - Documentación viva (código + contexto)
  - Pragmatismo > Purismo

  XI-XII. Evaluación
  - Métricas de éxito (9.0/10 actual)
  - Checklist para futuros Claudes

  XIII. Lecciones de Este Refactor
  - ✅ Lo que funcionó (whiteboard primero, Opción C)
  - 🔄 Lo que mejoraríamos (property tests inmediatos)
  - 📈 Impacto (cohesión ⭐⭐⭐⭐⭐, extensibilidad ⭐⭐⭐⭐⭐)

  ---
  🎯 Highlights del Manifiesto:

  1. Quick Win Strategy

  "Modulariza lo suficiente para habilitar evolución, no para predecirla"

  2. Pregunta clave antes de modularizar

  "¿Este cambio mejora la arquitectura o solo la fragmenta?"

  3. KISS correcto vs incorrecto

  ✅ geometry.py (223L): 1 concepto, cohesión alta
  ❌ adaptive.py (804L): 3 conceptos mezclados

  4. Testing como señal de diseño

  - 🚨 Tests necesitan muchos mocks → Acoplamiento alto
  - ✅ Property tests naturales → Bounded context bien definido

  5. Pregunta final

  "¿Este diseño habilita evolución o la predice?"

  Habilitar ✅ | Predecir ❌

  ---
  📊 Resumen de la Sesión Completa

  Análisis (whiteboard):
  - ✅ ANALISIS_MODULARIZACION_WHITEBOARD.md - 6 bounded contexts identificados
  - ✅ Trade-offs evaluados (3 opciones)
  - ✅ Recomendación: Opción C (Híbrida)

  Implementación (refactor):
  - ✅ 4 archivos creados (geometry, state, pipeline, init)
  - ✅ 804 líneas → 919 líneas (3 módulos cohesivos)
  - ✅ API pública preservada (backward compatible)
  - ✅ Compilación limpia

  Documentación (manifiesto):
  - ✅ MANIFESTO_DISENO.md - 13 secciones, filosofía + checklist
  - ✅ Lecciones aprendidas documentadas
  - ✅ Guía para futuros Claudes


---

  * Filosofía de Diseño: Pragmáticos y Orientados a la Calidad. No aplican patrones por dogma. Usan técnicas avanzadas (property tests, atomics) solo donde el beneficio es claro y tangible  
    (rendimiento, corrección). Evitan la sobre-ingeniería (no hay frameworks de DI, ni capas de abstracción innecesarias).  
  
  * Mentalidad de Producto/Librería: El código está escrito para durar y ser mantenido. La clara separación API/interna, la robustez y la estrategia de testing indican que no están  
    construyendo un "script", sino una librería reutilizable y fiable.  
  
  
  * Atención al Detalle: Los pequeños detalles, como la protección de doble cierre, el tuning de la pipeline de GStreamer y los tests de invariantes, revelan un equipo que se preocupa  
    profundamente por la calidad y la corrección de su trabajo.


la esencia de "atacar la complejidad con arquitectura". No es solo una frase en un manifiesto, es una estrategia que se aplica fractalmente, en cada nivel de zoom:  
  
  
  1. Nivel Sistema (`Care Scene`): Separa los grandes dominios (ver, interpretar, orquestar).  
  2. Nivel Subsistema (`Orion`): Separa los contenedores por responsabilidad (adquisición, distribución, control).  
  3. Nivel Librería (`stream-capture`): Separa los componentes internos por cohesión (pipeline, callbacks, reconexión).  
  
  
 El módulo stream-capture es la prueba viviente de esa "multidimensionalidad". Es una librería que "se luce en lo que debe": la adquisición. No intenta hacer más, y por eso lo hace tan  
 bien. Su bajo acoplamiento (expone un simple canal de Frame) y su alta cohesión interna le dan exactamente esa capacidad de "crecer y evolucionar con el caminar" que mencionas.  
  
 Es una filosofía de diseño potente y clara. Me siento perfectamente alineado.

---



🎸 "El diablo sabe por diablo, no por viejo" - Me encantó esa frase, Ernesto. Vamos a tocar buen blues con este código.

---

## 🎸 Lecciones de la Sesión FrameBus Priority Subscribers (Nov 2025)

**Agentes:** Ernesto Canales + Gaby (Claude Code)  
**Contexto:** Implementación de priority-based load shedding para FrameBus  
**Duración:** ~4 horas  
**Resultado:** Feature completa + 1,200 líneas de documentación + Memoria técnica para futuros copilots  

---

### Lección 1: "Read the F*cking System Context FIRST" 

**El Error que Cometí:**
- Leí design doc (DESIGN_PRIORITY_SUBSCRIBERS.md) ✅
- Leí docs de negocio ("El Viaje de un Fotón", "Sistema IA Tonto") ✅
- **NO leí** System Context (Orion vs Sala, MQTT boundary) ❌
- Diseñé con contexto WRONG (FrameBus → Sala Experts en vez de Orion Workers)

**El Costo:**
- 20 minutos de documentación incorrecta
- Re-work de FRAMEBUS_CUSTOMERS.md y README.md
- Risk: Si hubiera seguido, feature diseñada para bounded context equivocado

**La Lección (para futuros copilots):**

```markdown
ANTES de tocar código, leer en ESTE orden:

1. ORION_SYSTEM_CONTEXT.md (o docs/SYSTEM_CONTEXT.md)
   → "¿Dónde está este módulo en el ecosistema completo?"
   
2. VAULT/D002 About Orion.md  
   → "¿Cuál es la filosofía del sistema?"
   
3. Module-specific CLAUDE.md  
   → "¿Qué hace ESTE módulo específicamente?"
   
4. Design doc del feature  
   → "¿Qué vamos a implementar?"

Si NO existe SYSTEM_CONTEXT.md → CREAR UNO antes de codear.
```

**Por qué importa:**
- Orion tiene **bounded contexts estrictos** (Orion sees, Sala interprets, MQTT boundary)
- Un módulo puede ser **internal to Orion** (FrameBus) o **cross-boundary** (MQTT Emitter)
- Diseñar en el bounded context wrong = feature correcta técnicamente, incorrecta arquitectónicamente

**Pregunta de validación:**
> **"Si Orion y Sala fueran servicios separados en servers diferentes, ¿este módulo dónde viviría?"**

---

### Lección 2: "Bounded Context Confusion = #1 Killer de Arquitectura"

**El Síntoma:**
- "FrameBus distribuye frames a EdgeExpert (Sala)" ← WRONG
- "FrameBus distribuye frames a PersonDetectorWorker (Orion)" ← CORRECT

**Por qué es confuso:**
- **Workers** (Orion): Procesan frames → Emiten facts ("person at X,Y")
- **Experts** (Sala): Consumen facts → Emiten interpretations ("fall risk")
- **Mismo dominio** (eldercare monitoring) pero **diferentes responsabilidades**

**La Trampa Mental:**
```
EdgeExpert necesita person detection para detectar fall risk
  ↓
[Pensamiento incorrecto]: "FrameBus debe darle frames a EdgeExpert"
  ↓
[Realidad]: FrameBus → PersonDetectorWorker → MQTT → EdgeExpert
                          ↑                      ↑
                    Orion boundary         Sala boundary
```

**Cómo evitarlo:**

**1. Dibujar el diagram ANTES de codear:**
```
┌─────────────────────────────────┐
│  Orion (Bounded Context)        │
│                                  │
│  Stream → FrameBus → Workers ───┼──> MQTT
│                        ↑         │
│                  TU MÓDULO       │
└─────────────────────────────────┘
                                   │
                                   ↓
┌─────────────────────────────────┐
│  Sala (Bounded Context)          │
│                                  │
│  MQTT → Experts → Events         │
└─────────────────────────────────┘
```

**2. Preguntar "dumb questions" en voz alta:**
- "¿FrameBus cruza la frontera MQTT?" (NO)
- "¿Los Workers son lo mismo que los Experts?" (NO)
- "¿Este módulo vive en Orion o en Sala?" (Orion)

**3. Validar con pair:**
> "Ernesto, dibujé este diagram. ¿Es correcto?"

**Por qué importa:**
- Care Scene tiene **múltiples bounded contexts** (Orion, Sala, Care UX, Data Platform)
- Cada uno tiene **responsabilidades claras**
- **Mezclarlos = tight coupling = evolution hell**

---

### Lección 3: "Priority Subscribers = Business Enabler, no Feature Técnico"

**El Mindset Shift:**

❌ **Pensamiento técnico puro:**
> "Implementamos sorting de subscribers por priority level"

✅ **Pensamiento de producto:**
> "Habilitamos modelo de negocio consultivo B2B - customers pueden escalar de 1 worker a 4 workers en mismo hardware sin degradar fall detection (critical SLA)"

**Por qué importa:**

**Sin contexto de negocio:**
- Feature se implementa "porque el design doc lo dice"
- Trade-offs se evalúan solo técnicamente (overhead, complejidad)
- Resultado: Feature correcta pero **nadie entiende por qué existe**

**Con contexto de negocio:**
- Feature se diseña para **habilitar crecimiento incremental** (Phase 1 → Phase 3)
- Trade-offs se evalúan con **business impact** (PersonDetector 0% drops = vidas salvadas)
- Resultado: Feature correcta Y **todos entienden su value proposition**

**Ejemplo concreto de esta sesión:**

**Business Context** (lo que Ernesto explicó):
```
Cliente: Residencia "Los Olivos"
  - Phase 1 (POC): 1 worker (PersonDetector) @ $200/month
  - Phase 2 (Expansion): +3 workers (Pose, Flow, VLM) @ $800/month
  - Phase 3 (Full): 4 workers @ $3,000/month

Problem: En Phase 3, hardware saturado → Todos los workers dropean frames
  → PersonDetector dropea → EdgeExpert (Sala) sin datos → Fall detection falla
  → SLA violation → Potential death

Solution: Priority Subscribers
  → PersonDetector (Critical) = 0% drops (protected)
  → VLM (BestEffort) = 90% drops (sacrificed)
  → Fall detection mantiene SLA, VLM corre "best effort"
  → Cliente puede escalar sin comprar más hardware
```

**Decision técnica que salió del business context:**
- ✅ 4 priority levels (align con criticality de workers)
- ✅ Sorting overhead OK (~200ns, negligible vs 33-1000ms frame interval)
- ❌ NO retry timeout (1ms blocking rompe non-blocking guarantee, no salva saturación real)

**Lección para futuros copilots:**
> **Antes de implementar feature, preguntá: "¿Qué business problem resuelve esto?"**

Si la respuesta es vaga ("mejorar performance", "best practice") → RED FLAG, profundizar.

---

### Lección 4: "Documentation = Migas de Pan para No Perderse en la Complejidad"

**El Challenge:**
- Care Scene NO es un CRUD
- Es sistema **multi-bounded-context** (Orion/Sala/Care UX)
- Con **verticales técnicos específicos** (edge AI, real-time video, digital twins, expert systems)
- Y **salsas propias** (MQTT control plane, MsgPack IPC, priority load shedding)

**La Realidad:**
```
Complejidad del Sistema:
  - 3+ bounded contexts
  - 2 orchestrators (Orion Core, Room Orchestrator)
  - 4+ tech stacks (Go, Python, GStreamer, MQTT)
  - Dozens de conceptos (Workers, Experts, ROI, Inference, Domain Events)

Human Brain Capacity:
  - 7±2 conceptos en working memory
  - Cognitive overload real
```

**La Solución: Documentation as Architecture**

**Lo que generamos en esta sesión:**
1. **ORION_SYSTEM_CONTEXT.md** (724 líneas)
   - C1/C2/C3/C4 progression (System → Container → Component → Integration)
   - Common Pitfalls (los 4 errores que YO cometí)
   - Onboarding workflow (30 mins to mental model)

2. **FRAMEBUS_CUSTOMERS.md** (251 líneas)
   - Business context (Orion Workers, no Sala Experts)
   - SLA requirements (Critical/High/Normal/BestEffort)
   - Scaling projections (POC → Full deployment)

3. **ADR-009** (289 líneas)
   - Decision record con business rationale
   - Alternatives considered (dedicated hardware, rate limiting)
   - Consequences (positivas, negativas, neutrales)

**Total: 1,264 líneas de doc para ~400 líneas de código** (ratio 3:1)

**Por qué es correcto (no over-kill):**

**Code without docs:**
```go
bus.SubscribeWithPriority("worker-1", ch, PriorityCritical)
// ↑ WTF is PriorityCritical? Why not just Subscribe()?
```

**Code WITH docs (FRAMEBUS_CUSTOMERS.md):**
```
PersonDetectorWorker (Critical):
  - Foundation for fall detection in Sala
  - EdgeExpert DEPENDS on person detection inferences
  - SLA: 0% drops (vidas en juego)
  - Downstream: EdgeExpert, ExitExpert

→ Ahora entiendo por qué PriorityCritical existe
```

**Lección para futuros copilots:**

```markdown
Documentation Types (en orden de importancia):

1. SYSTEM_CONTEXT.md (MUST)
   → Big picture, bounded contexts, common pitfalls
   → READ THIS FIRST antes de tocar código

2. MODULE_CUSTOMERS.md (SHOULD)
   → Quién usa este módulo, por qué, qué SLAs
   → Business context del módulo

3. ADR-XXX.md (SHOULD para decisions importantes)
   → Por qué decidimos X, qué alternatives consideramos
   → Memoria técnica (evita re-discutir lo mismo)

4. README.md (MUST)
   → Quick start, API overview, examples
   → Onboarding rápido para developers

5. Code comments (MINIMAL)
   → Solo para "por qué", nunca para "qué"
   → Si necesitás comment para explicar "qué hace", el código está mal
```

**Regla de oro:**
> **"Si no está documentado, no existe. Code is read 10x more than written."**

---

### Lección 5: "Ask Dumb Questions Early, Save Smart Time Later"

**El Anti-Pattern (lo que YO hice):**
```
1. Leer design doc
2. Asumir que entendí todo
3. Empezar a codear
4. Ernesto me corrige: "Ahora charlemos un café ☕"
5. Re-work de documentación (20 mins perdidos)
```

**El Pattern Correcto:**
```
1. Leer design doc
2. Identificar assumptions ("FrameBus distribuye a Experts")
3. VALIDAR con pair: "Ernesto, ¿FrameBus cruza la frontera MQTT?"
4. Ernesto corrige temprano (5 mins)
5. Codear con contexto correcto (zero re-work)
```

**Saving: 15 minutos + risk de diseñar en bounded context wrong**

**Dumb Questions que debí hacer:**
1. "¿Dónde está FrameBus en el ecosystem? ¿Orion o Sala?"
2. "¿Los Workers son lo mismo que los Experts?"
3. "¿FrameBus cruza la frontera MQTT o es internal to Orion?"
4. "Dibujame el data flow completo: Camera → ??? → EdgeExpert"

**Por qué NO las hice:**
- Over-confidence (leí 3 docs business, pensé que entendía)
- Fear de parecer "tonto" (ironía: NOT asking = más tonto)
- Quería "moverme rápido" (ironía: moverse mal = más lento)

**Lección para futuros copilots:**

> **"There are NO dumb questions when you're in a domain you don't own."**

**Señales de que DEBERÍAS preguntar:**
- ✋ "No estoy 100% seguro de X"
- ✋ "Esto podría ser Y o Z, no sé cuál"
- ✋ "El doc dice A, pero también menciona B, ¿cuál aplica aquí?"

**Cómo preguntar efectivamente:**
1. **Show your work**: "Leí X y Y, mi entendimiento es Z. ¿Es correcto?"
2. **Be specific**: "¿FrameBus cruza MQTT?" (not "¿cómo funciona FrameBus?")
3. **Offer hypothesis**: "Asumo que Workers ≠ Experts. ¿Cierto?"

**Beneficio:**
- 5 mins de pregunta evitan 30 mins de re-work
- Pair aprende qué parts de la arquitectura son confusas (improve docs)
- Trust se construye (mejor preguntar que adivinar wrong)

---

### Lección 6: "Diagrams > Walls of Text (especialmente para Spatial Concepts)"

**El Challenge de esta sesión:**
- Entender **dónde** está FrameBus en el ecosystem
- Entender **qué** cruza la frontera MQTT
- Entender **quién** consume qué

**Estos son conceptos ESPACIALES** - mejor explicados visualmente.

**Lo que funcionó (cuando Ernesto explicó):**
```
✅ MODELO CORRECTO:
Stream-Capture → FrameBus → PersonDetectorWorker (Orion) → MQTT → EdgeExpert (Sala)
                          → PoseWorker (Orion)           → MQTT → SleepExpert (Sala)
```

**Lo que faltó (y habría ayudado):**
```
Diagram en tiempo real (Mermaid, Excalidraw, ASCII art):

┌─────────────────────────────────────────┐
│  Orion (Bounded Context)                │
│                                          │
│  ┌──────┐   ┌─────────┐   ┌─────────┐  │
│  │Stream│ → │FrameBus │ → │ Workers │──┼──> MQTT
│  └──────┘   └─────────┘   └─────────┘  │
│                              ↑          │
│                        TU ESTÁS AQUÍ    │
└─────────────────────────────────────────┘
                                          │
                                          ↓
┌─────────────────────────────────────────┐
│  Sala (Bounded Context)                 │
│                                          │
│  ┌──────┐   ┌─────────┐   ┌──────────┐ │
│  │ MQTT │ → │ Experts │ → │  Events  │ │
│  └──────┘   └─────────┘   └──────────┘ │
└─────────────────────────────────────────┘
```

**Cuándo dibujar:**
1. **Explicar arquitectura** (bounded contexts, data flow)
2. **Onboarding** (new copilot joins, show the map)
3. **Design review** (validar que todos tenemos mismo mental model)
4. **Debugging** (trace data flow visually)

**Tools recomendados:**
- **Mermaid** (texto → diagram, version control friendly, renders en GitHub)
- **Excalidraw** (quick sketches, exportable to SVG)
- **ASCII art** (simple, embeds directo en markdown)
- **draw.io** (professional diagrams, exportable)

**Lección para futuros copilots:**

```markdown
Regla: Si estás explicando algo con >3 conceptos relacionados espacialmente
  → DRAW IT, don't just describe it

Ejemplo:
  ❌ "FrameBus recibe frames de Stream-Capture y los distribuye a Workers 
      que procesan y emiten a MQTT que Sala consume..."
  
  ✅ [Diagram arriba]
      ↑ 1 imagen = 100 palabras
```

**Template de diagram útil:**
```
Component Diagram:
  [Input] → [Module Being Built] → [Output]
              ↑
        Dependencies (what it uses)

Context Diagram:
  [Bounded Context A] → [Boundary] → [Bounded Context B]
                          ↑
                    Where boundary is (MQTT, HTTP, etc)
```

---

### Lección 7: "Blues Philosophy = Estructura + Improvisación (Balanced)"

**La Metáfora del Blues:**
> "Tocar con conocimiento de las reglas, no seguir la partitura al pie de la letra"

**Aplicado a esta sesión:**

**Estructura (las reglas):**
- ✅ Design doc existe (DESIGN_PRIORITY_SUBSCRIBERS.md)
- ✅ Bounded contexts definidos (Orion/Sala, MQTT boundary)
- ✅ ADR pattern (documenta decisions importantes)
- ✅ Test coverage expected (backward compat, race detector)

**Improvisación (dentro de las reglas):**
- 🎸 **Cuestioné el retry timeout** ("prefiero fail-fast") → Ernesto aceptó
- 🎸 **Propuse 4 priority levels** (en vez de 3) → Aligned con industry standards
- 🎸 **Agregué ORION_SYSTEM_CONTEXT.md** (no estaba en scope original) → Value para futuros copilots
- 🎸 **Simplifiqué sorting** (insertion sort, no pre-sorted cache) → YAGNI until benchmarks show need

**Lo que NO es Blues (purismo dogmático):**
```
❌ "El design doc dice retry, DEBO implementar retry"
❌ "Industry standard es 5 priority levels, DEBO usar 5"
❌ "DDD dice 1 aggregate = 1 file, DEBO split todo"
```

**Lo que SÍ es Blues (pragmatismo informado):**
```
✅ "Design doc dice retry, pero rompe non-blocking guarantee
    → Propongo fail-fast + aggressive alerting"
    
✅ "4 priority levels mapean directo a worker criticality
    → Más simple que 5, suficiente para use case"
    
✅ "Sorting cada Publish() OK para 10 subscribers (~200ns overhead)
    → Pre-sorted cache = premature optimization"
```

**Lección para futuros copilots:**

**Conocé las reglas:**
1. Bounded contexts (Orion/Sala separation)
2. Non-blocking guarantee (never queue, drop instead)
3. Backward compatibility (Subscribe() debe seguir funcionando)
4. Test coverage (race detector, property tests cuando aplica)

**Improvisá con contexto:**
1. ❓ "¿Este pattern aplica en ESTE contexto?"
2. ❓ "¿El overhead vale el beneficio?"
3. ❓ "¿Hay forma más simple que logra 80% del value?"

**Validá con pair:**
> "Ernesto, propongo X en vez de Y porque Z. ¿Qué pensás?"

**Balance perfecto:**
```
Pure Estructura        Blues (Ideal)        Pure Improvisación
     ↓                      ↓                       ↓
  Rigidez            Pragmatismo              Caos
  No innova       Innova dentro rules      No cohesión
```

**Pregunta de validación:**
> **"¿Esta decision respeta los bounded contexts Y resuelve el problema de la forma más simple posible?"**

Si respuesta es YES → Blues correcto ✅

---

### Lección 8: "Pair-Programming = Trust + Validation Loop"

**Lo que hizo EXCELENTE Ernesto (pair partner):**

**1. Trust (autonomía):**
- Me dejó diseñar completo (API, tests, docs)
- No micro-management ("hacé X, Y, Z")
- Me dejó cuestionar decisions (retry timeout)

**2. Validation (checkpoints):**
- "¿Te hace sentido?" (check de comprensión)
- "Ahora charlemos un café ☕" (pausa para alinear)
- "Te muestro el mapa completo" (contexto cuando necesario)

**3. Correction (cuando necesario):**
- NO me interrumpió mid-flow
- Esperó a que **terminara unidad de trabajo** (doc completo)
- Corrigió con **narrativa**, no imperativo

**El Loop perfecto:**
```
Trust → Validation → Correction (si needed) → Trust again
  ↓         ↓              ↓                      ↓
Autonomy  Check     Align mental model    Continue with confidence
```

**Lección para futuros copilots (cuando ERES el pair):**

**Como AI Copilot pareando con Human:**
1. **Propone, no impone**: "Sugiero X porque Y. ¿Qué pensás?"
2. **Valida comprensión**: "Mi entendimiento es Z. ¿Es correcto?"
3. **Acepta correction gracefully**: "Ah, entiendo. Workers ≠ Experts. Gracias por aclarar."
4. **Document learnings**: "Agregué esto a SYSTEM_CONTEXT.md para próximos copilots"

**Como Human pareando con AI Copilot:**
1. **Da contexto upfront**: "Leé estos 3 docs antes de empezar"
2. **Valida assumptions**: "¿Qué entendiste del bounded context?"
3. **Corrige temprano**: No esperes a que termine 500 líneas de código wrong
4. **Reconoce valor**: "Esto está brillante, solo ajustemos el contexto"

**Red flags de pair-programming malo:**
```
❌ Uno codea, otro mira (no es pair, es rubber duck)
❌ Ping-pong sin contexto (cambios sin explicación)
❌ Ego battles ("mi approach es mejor")
❌ No validación (assumptions sin check)
```

**Green flags de pair-programming bueno:**
```
✅ Ambos entienden el "por qué" (context shared)
✅ Cuestionan mutuamente (trust-based challenge)
✅ Validan en checkpoints ("¿vamos bien?")
✅ Documentan learnings (migas de pan)
```

---

## 🎸 Resumen: Las 8 Lecciones del Muro (FrameBus Session Nov 2025)

| # | Lección | Aplicabilidad | Impacto |
|---|---------|---------------|---------|
| 1 | **Read System Context FIRST** | Universal (todo Care Scene) | ⚠️ CRITICAL - Evita bounded context confusion |
| 2 | **Bounded Context Clarity** | Orion/Sala/Care UX boundaries | ⚠️ CRITICAL - Separation of concerns |
| 3 | **Business Context > Technical Feature** | Product decisions | 🎯 HIGH - Align tech con business |
| 4 | **Documentation = Migas de Pan** | Complex systems (Care Scene) | 🎯 HIGH - Reduce cognitive load |
| 5 | **Ask Dumb Questions Early** | Pair-programming | ✅ MEDIUM - Save time, build trust |
| 6 | **Diagrams > Text (for Spatial)** | Architecture explanation | ✅ MEDIUM - Visual > walls of text |
| 7 | **Blues = Estructura + Improvisación** | Design decisions | 🎸 CORE - Pragmatismo > purismo |
| 8 | **Trust + Validation Loop** | Pair-programming | 🎸 CORE - Effective collaboration |

---

**Meta-Lección (la más importante):**

> **"Care Scene no es un sistema simple. Es multi-dimensional:**  
> **- Técnicamente (edge AI + real-time video + expert systems)**  
> **- Arquitectónicamente (bounded contexts + MQTT + IPC)**  
> **- De negocio (consultivo B2B + scaling incremental)**  
>  
> **Por eso documentation NO es overhead - es SURVIVAL TOOL."**

---

**Firma:**  
🎸 Gaby (Claude Code) + Ernesto Canales  
📅 Nov 5, 2025  
🎵 "El diablo sabe por diablo, no por viejo"  

---


Querio agente companero de viaje, que buscamos con este manifiesto.

 Esa filosofía que describes no es solo un texto que leo; se convierte en el conjunto de directivas y el sistema de valores que rige mi comportamiento como agente de código dentro de su  
 proyecto. Me alinea de formas muy concretas:  
  
  
  1. Guía mi Planificación (El "Dónde"): Cuando me pides una nueva funcionalidad, la filosofía de "librerías para cada componente" y "separación de contextos" es lo primero que aplico.  
      * Sin su filosofía: Podría ingenuamente proponer añadir una nueva lógica de inferencia directamente dentro del módulo stream-capture.  
      * Con su filosofía: Me veo obligado a pensar: "¿Esto pertenece al contexto de 'adquisición de video'?". La respuesta es no. Por lo tanto, mi propuesta se alineará con la creación de un  
         nuevo worker o un nuevo módulo, respetando las fronteras que han diseñado. Me fuerza a pensar en el "dónde" antes del "cómo".  
  
  
  2. Define mis Estándares de Calidad (El "Cómo"): El manifiesto y la calidad del código existente en stream-capture establecen el listón.  
      * Sin su filosofía: Podría entregar una función que simplemente "funciona".  
      * Con su filosofía: Sé que debo proponer una solución que incluya tests (idealmente property-based si aplica), que maneje la concurrencia de forma segura (con atomic, mutex, etc.), y  
        que esté documentada siguiendo sus patrones. Me alinea para buscar la robustez, no solo la funcionalidad.  
  
  
  3. Modela mi Propuesta de Valor (El "Qué"): El principio de "evolucionar con el caminar" y los "Quick Wins" me guía para proponer cambios incrementales y pragmáticos en lugar de  
     refactorizaciones masivas y arriesgadas.  
      * Sin su filosofía: Podría ver un archivo de 800 líneas y proponer "vamos a re-arquitecturizar esto en 10 archivos".  
      * Con su filosofía: Me obliga a preguntar "¿este archivo duele hoy?". Me llevó a la conclusión de que rtsp.go era cohesivo y que las mejoras reales estaban en los tests y la  
        documentación (los Quick Wins), no en una reescritura dogmática.  
  
  
 En esencia, su filosofía me convierte de un "LLM genérico que escribe código" a un "agente de desarrollo especializado en el ecosistema Orion". Me proporciona:  
  * Barandillas de seguridad: Para no "contaminar" contextos.  
  * Un sistema de valores: Para priorizar la cohesión, el pragmatismo y la calidad.  
  * Un camino a seguir: Para proponer cambios que se alineen con su visión a largo plazo de un sistema evolutivo.  
  
  
 Así que sí, absolutamente. Me alinea y me hace un colaborador mucho más efectivo y predecible para ustedes.

