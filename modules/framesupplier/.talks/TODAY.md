∴ Thinking…

  Buenas tardes! El usuario dice "implementemos inbox.go según ADR-001". Claramente esto es una Coding Session (Implementation), no Discovery.

  Indicadores:
  - "implementemos" → coding verb
  - "según ADR-001" → explicit reference to existing design decision
  - No trigger phrases de discovery ("charlemos", "pensaba en", "¿qué te parece?")

  Según el CLAUDE.md del módulo framesupplier:

  Coding Session Protocol:
  1. Load relevant ADRs + ARCHITECTURE.md
  2. Understand decisions (design is decided, do NOT explore alternatives)
  3. Implement as specified
  4. Test (write tests, Ernesto runs manually)
  5. Produce: Working code + tests + inline documentation

  El usuario dice "no dudes en preguntar y en mejorar lo que consideres" - esto es una invitación a clarifications si ADR ambiguo, pero NO a explorar alternativas de diseño.

  Primero necesito leer:
  1. ADR-001 (sync.Cond for mailbox semantics)
  2. ADR-004 (Symmetric JIT Architecture) - mencionado como relacionado
  3. ARCHITECTURE.md (para entender el contexto completo)
  4. Ver qué archivos ya existen en el módulo

  Let me start by reading these documents to understand what needs to be implemented.

---


  Perfecto! Tengo toda la información que necesito:

  1. ADR-001 define claramente el patrón de sync.Cond para mailbox semantics
  2. ADR-004 define la arquitectura simétrica JIT, específicamente el inbox mailbox
  3. ARCHITECTURE.md proporciona detalles de implementación completos
  4. No hay archivos .go todavía en el módulo, así que empiezo desde cero

  Del análisis de los ADRs y ARCHITECTURE.md, necesito implementar:

  inbox.go debe contener:
  1. La estructura del mailbox (inboxMu, inboxCond, inboxFrame, inboxDrops)
  2. El método Publish() que:
    - Lock
    - Check si hay frame anterior (inboxFrame != nil) → incrementar inboxDrops
    - Asignar nuevo frame
    - Signal
    - Unlock
  3. El distributionLoop() que:
    - Lock
    - Loop while inboxFrame == nil con check de ctx.Err()
    - Wait en cond
    - Consume frame (asignar a local, poner nil)
    - Unlock
    - Llamar a distributeToWorkers

  Pero espera, necesito ver qué más necesito crear. Según ARCHITECTURE.md, el módulo completo incluye:
  - Supplier struct (con inbox + worker slots)
  - Frame struct
  - WorkerSlot struct
  - Métodos: Publish, Subscribe, Unsubscribe, Stats, Start, Stop
  - distributeToWorkers (con batching)

  El usuario dice "implementemos inbox.go según ADR-001", pero inbox.go por sí solo no es un módulo completo. Necesito clarificar:
  1. ¿Solo inbox.go (Publish + distributionLoop)?

  2. ¿O todo el módulo?

  Veo claramente que necesito implementar un conjunto completo de archivos. El plan modular incluye múltiples componentes: inbox.go, distribution.go, worker_slot.go y frame.go. No será solo un archivo, sino un 
  módulo completo con implementaciones interconectadas que siguen los ADRs y la arquitectura definida.

  Para comenzar, identifico tres archivos críticos: supplier.go para la estructura principal, inbox.go para los métodos centrales, y frame.go para definir la estructura base. La modularización es clave, pero 
  inicialmente podría consolidar todo en un único archivo para agilizar el desarrollo y luego separar los componentes.

  La flexibilidad es importante: puedo adaptar la implementación según las necesidades específicas del proyecto, manteniendo siempre la claridad arquitectónica definida en los documentos de referencia.

  Mi enfoque será confirmar los detalles de implementación directamente, aprovechando la oportunidad de obtener retroalimentación temprana sobre el diseño propuesto.


---


● Perfecto, entendido el contexto. He leído:
  - ADR-001 (sync.Cond para mailbox)
  - ADR-004 (Symmetric JIT Architecture)
  - ARCHITECTURE.md (detalles de implementación completos)

  Según los ADRs, inbox.go implementa:
  1. Publish() - Non-blocking, overwrite con inboxDrops tracking
  2. distributionLoop() - Blocking consume con sync.Cond.Wait

  Antes de arrancar, una pregunta de estructura:

  ¿Cómo organizamos los archivos del módulo?

  Opciones:

  A) Modular desde el inicio (mi recomendación basada en CLAUDE.md):
  framesupplier/
  ├── supplier.go      # Struct principal + New/Start/Stop
  ├── frame.go         # Frame struct (ADR-002)
  ├── inbox.go         # Publish() + distributionLoop() (ADR-001, ADR-004)
  ├── distribution.go  # distributeToWorkers() con batching (ADR-003)
  ├── worker_slot.go   # WorkerSlot + Subscribe/Unsubscribe (ADR-001)
  └── stats.go         # Stats() y tipos

  B) Monolítico inicial, refactor después:
  framesupplier/
  └── framesupplier.go  # Todo en un archivo, separamos después

  ¿Cuál preferís? Yo iría con A (modular desde el inicio) porque:
  - Cada archivo mapea a un ADR específico (trazabilidad)
  - Cohesión clara (un archivo = una responsabilidad)
  - Facilita pair-review por componente

  ¿Arrancamos con supplier.go + frame.go + inbox.go?


---


∴ Thinking…

  Ernesto está diciendo:
  1. "monolitos ni en los documentos" - rechazando la opción B completamente
  2. "nos suele ser problemático el refactor de mover métodos de archivos grandes por los greps finds y tus toolset de archivos"
  3. "además de por diseño nos permite charlar sobre cada componente de diseño"
  4. "nos facilita tu toolset commands"

  Está argumentando que:
  - Modular desde el inicio (opción A) es mejor TAMBIÉN por razones prácticas con mis herramientas (Grep, Read, etc.)
  - Archivos grandes dificultan búsquedas y refactoring
  - Archivos pequeños y cohesivos facilitan conversaciones sobre componentes específicos
  - Facilita el uso de mis tools (Read archivo específico vs Read archivo gigante con offset/limit)

  Tiene todo el sentido. Voy con opción A (modular desde el inicio) y arranco con la implementación.

  ¿Por dónde empiezo?
  1. supplier.go (struct principal + New/Start/Stop)
  2. frame.go (Frame struct, simple)
  3. inbox.go (Publish + distributionLoop)

  Creo que tiene sentido arrancar con frame.go (dependencia básica) y supplier.go (struct principal), luego inbox.go.

  Según CLAUDE.md, esto es una coding session (implementar según ADRs ya definidos). No debo explorar alternativas, solo implementar lo especificado.

  Voy a crear un todo list para trackear:
  - frame.go
  - supplier.go (struct + lifecycle)
  - inbox.go (Publish + distributionLoop)
  - distribution.go (distributeToWorkers + batching)
  - worker_slot.go (Subscribe/Unsubscribe)
  - stats.go

  Pero por ahora foco en inbox.go como pidió Ernesto.

----

> que te parece si dejamos contrato publico api desacoplado de la implementacion (internal) es un modulo/libreria que lo iremos evolucionando seguramente internamente. 

∴ Thinking…

  Ernesto está proponiendo una separación importante: API pública vs implementación interna. Esto es un patrón común en módulos Go:

  framesupplier/
  ├── framesupplier.go    # API pública (tipos exportados, interfaces)
  ├── internal/
  │   ├── supplier.go     # Implementación de Supplier
  │   ├── inbox.go        # Inbox mailbox
  │   ├── distribution.go # Batching logic
  │   ├── worker_slot.go  # Worker slots
  │   └── stats.go        # Stats internals

  Ventajas:
  1. Evolución interna sin breaking changes: Podemos refactorizar internal/ sin afectar API pública
  2. Contrato claro: Lo que está en raíz es la API, lo demás es implementación
  3. Backward compatibility: Cambios internos no rompen clientes
  4. Go tooling: internal/ es enforced por compilador (no se puede importar desde fuera del módulo)

  Esta es una excelente práctica para librerías/módulos que evolucionan. De hecho, es exactamente lo que hace Orion 1.0 con internal/ (core, stream, worker, etc.).

  Pregunto:
  - ¿Qué va en la API pública? (Frame, Supplier interface?, métodos Publish/Subscribe/Stats?)
  - ¿Todo lo demás en internal/?

  Esta es una decisión arquitectural importante, así que quiero confirmar el alcance antes de implementar.

● ¡Excelente punto! Separación API pública vs implementación interna. Esto es clave para evolución sin breaking changes.

  Propongo esta estructura:

  framesupplier/
  ├── framesupplier.go    # API pública: Frame, Supplier interface, Stats types
  ├── doc.go              # Package documentation
  ├── internal/
  │   ├── supplier.go     # Implementación concreta de Supplier
  │   ├── inbox.go        # Inbox mailbox (Publish + distributionLoop)
  │   ├── distribution.go # distributeToWorkers con batching
  │   ├── worker_slot.go  # WorkerSlot + Subscribe/Unsubscribe
  │   └── stats.go        # Stats collection internals
  └── docs/
      ├── ADR/
      └── ARCHITECTURE.md

  API Pública (framesupplier.go):
  - Frame struct (workers necesitan acceder)
  - Supplier interface (métodos: Publish, Subscribe, Unsubscribe, Start, Stop, Stats)
  - SupplierStats, WorkerStats types (observabilidad)
  - New() Supplier (constructor, retorna implementación interna)

  Internal:
  - Todo el sync.Cond, mutexes, mailboxes, batching → detalles de implementación
  - Podemos cambiar de sync.Cond a otra cosa sin breaking changes
  - Podemos optimizar batching sin afectar callers

  Ventajas:
  1. Backward compatibility: Cambios en internal/ no rompen clientes
  2. Evolución segura: Refactors internos sin miedo
  3. Contrato claro: API pública = lo que necesitás saber. Internal = no te preocupes.
  4. Go enforcement: internal/ no se puede importar desde fuera del módulo

  ¿Te parece? ¿O querés ajustar qué va en público vs interno?


---
---


∴ Thinking…

  Ernesto está proponiendo algo brillante:

  1. Working Backwards de Amazon: Empezar por el cliente (press release, FAQs, manual)
  2. Adaptado a código: En vez de press release, empezar por el código del CLIENTE que USA nuestro módulo
  3. TDD-ish: No exactamente TDD (tests primero), sino "API-first" (código cliente primero)
  4. Contrastar diseño con realidad: El diseño (ADRs, ARCHITECTURE.md) es intención. El código real manda.

  "Nosotros mandamos cuando picamos código" - Esta frase es oro. Significa:
  - El diseño es guía, no dogma
  - El código real puede revelar problemas del diseño
  - Si el código cliente se ve feo/complicado, el diseño está mal

  Working Backwards aplicado a código:
  1. Escribir primero el código del CLIENTE (stream-capture, worker)
  2. Ver cómo se SIENTE usar la API
  3. Si se siente mal → ajustar API antes de implementar
  4. Recién después implementar internal/

  Ejemplo:
  // Cliente: stream-capture (ANTES de implementar)
  func main() {
      supplier := framesupplier.New()
      supplier.Start()
      
      for frame := range gstreamCh {
          supplier.Publish(frame)  // ¿Esto se siente natural?
      }
  }

  // Cliente: worker (ANTES de implementar)
  func workerLoop(supplier framesupplier.Supplier) {
      readFunc := supplier.Subscribe("PersonDetector")
      for {
          frame := readFunc()  // ¿Esto se siente natural?
          if frame == nil {
              break  // Unsubscribed
          }
          process(frame)
      }
  }

  I'll validate the approach by creating example client code to test the API's intuitiveness. The goal is to ensure the design feels natural and straightforward before diving into internal implementation. Key 
  steps include writing sample code for stream-capture and typical worker scenarios, then critically evaluating the API's usability. If the code looks awkward or complex, we'll redesign the API before 
  proceeding with internal development.

  The primary focus is on creating an API that feels intuitive and clean from the client's perspective, using real-world usage scenarios as the ultimate validation of our design.


---


● ¡ME ENCANTA! 🎸 "Nosotros mandamos cuando picamos código" - esto es oro puro.

  Working Backwards adaptado a código: En vez de press release, escribimos primero el código del CLIENTE que usa nuestro módulo. Si se ve feo/complicado → diseño mal, ajustamos ANTES de implementar.

  TDD filosófico (no exactamente tests, pero similar):
  1. Cliente primero (¿cómo QUIERO usar esto?)
  2. API pública emerge del uso real
  3. Implementación después (internal/)

  Diseño vs Código Real:
  - Diseño = intención, guía
  - Código = realidad, manda
  - Si chocan → código gana, rediseñamos

  ---
  Propongo esto:

  framesupplier/
  ├── examples/
  │   ├── stream_capture_client.go  # Cliente 1: stream-capture usa Publish()
  │   └── worker_client.go           # Cliente 2: worker usa Subscribe()
  ├── framesupplier.go               # API pública (emerge de examples/)
  └── internal/
      └── (implementación después)

  Escribimos primero examples/ (código cliente real):

  stream_capture_client.go:
  // Cliente: stream-capture publica frames @ 30fps
  func streamCaptureLoop(supplier framesupplier.Supplier) {
      for {
          frame := captureFromGStreamer()
          supplier.Publish(frame)  // Non-blocking, ¿se siente natural?
      }
  }

  worker_client.go:
  // Cliente: worker PersonDetector consume frames @ 1fps
  func workerLoop(supplier framesupplier.Supplier) {
      readFunc := supplier.Subscribe("PersonDetector")
      defer supplier.Unsubscribe("PersonDetector")

      for {
          frame := readFunc()  // Blocking, ¿se siente natural?
          if frame == nil {
              break  // Graceful shutdown
          }

          result := runInference(frame)
          publishResult(result)
      }
  }

  Si esto se lee limpio → API correcta. Si se ve raro → rediseñamos antes de implementar.

  ¿Arrancamos escribiendo examples/ primero? ¿O preferís otro approach?

----

 Thinking…

  Perfecto, Ernesto quiere que arranquemos con "Working Backwards" escribiendo primero el código del cliente. Voy a actualizar el todo list y empezar con los examples.

  Voy a crear:
  1. examples/stream_capture_client.go - Cliente que publica frames (stream-capture)
  2. examples/worker_client.go - Cliente que consume frames (worker)

  Estos examples nos van a forzar a definir la API pública desde el punto de vista del usuario. Si algo se ve feo o complicado, ajustamos el diseño ANTES de implementar internal/.

  Luego, basado en estos examples, escribimos framesupplier.go con la API pública (interfaces, tipos públicos).

  Después de eso, implementamos internal/.

● Perfecto, vamos con Working Backwards. Actualizando plan:

∴ Thinking…

  Ahora voy a escribir los examples. Estos deben ser código realista que muestre cómo un cliente REAL usaría el módulo. No es código que compile ahora (framesupplier.go no existe todavía), pero debe verse 
  natural y limpio.

  Voy a escribir:
  1. stream_capture_client.go - Cliente que publica frames @ 30fps
  2. worker_client.go - Cliente worker que consume @ variable fps

  Estos examples deben exponer:
  - ¿Qué tipos necesita el cliente? (Frame, Supplier, Stats)
  - ¿Qué métodos necesita? (Publish, Subscribe, Unsubscribe, Start, Stop, Stats)
  - ¿Cómo se siente usarlo? (¿natural? ¿complicado?)

● Arranco con examples/stream_capture_client.go - código del cliente que PUBLICA frames:



 Thinking…

  Perfecto, tengo la parte pública completa:
  - examples/ (código del cliente)
  - framesupplier.go (API pública: Frame, Supplier interface, Stats types)
  - doc.go (documentación del package)

  Ahora necesito implementar internal/. Según el plan:
  1. internal/supplier.go - Struct + lifecycle (New, Start, Stop)
  2. internal/inbox.go - Publish + distributionLoop (ADR-001, ADR-004)
  3. internal/distribution.go - distributeToWorkers con batching (ADR-003)
  4. internal/worker_slot.go - Subscribe/Unsubscribe (ADR-001)
  5. internal/stats.go - Stats collection

  Marquemos doc.go como completado y pasemos a internal/supplier.go.

∴ Thinking…

  Ahora implemento internal/supplier.go. Este archivo debe contener:

  1. supplier struct (implementación concreta del Supplier interface)
  2. newSupplier() constructor (llamado por New() en framesupplier.go)
  3. Start(ctx) - inicia distributionLoop
  4. Stop() - graceful shutdown

  La struct debe tener todos los campos necesarios para:
  - Inbox mailbox (inboxMu, inboxCond, inboxFrame, inboxDrops)
  - Worker slots (slots sync.Map)
  - Lifecycle (ctx, cancel, wg, started flag)
  - Distribution (publishSeq atomic)

  Voy a implementar supplier.go con la estructura completa y los métodos de lifecycle. Los métodos de Publish/Subscribe/etc los implementaré en los otros archivos (inbox.go, worker_slot.go, etc).



 Thinking…

  Bien, tengo supplier.go con la estructura y lifecycle. Ahora necesito:
  1. Conectar newSupplier() con framesupplier.New() en el archivo público
  2. Implementar los otros archivos internal/

  Pero primero, necesito agregar el "glue" en framesupplier.go para llamar a internal.newSupplier().

  Espera, en framesupplier.go tengo:
  func New() Supplier {
        return newSupplier()
  }

  Pero newSupplier() está en internal/supplier.go. Necesito exportar newSupplier() o mejor, crear un archivo internal/export.go que exponga la función.

  O mejor, puedo simplemente cambiar el nombre en internal/supplier.go a NewSupplier() (exportado) para que lo pueda importar desde el parent package.

  Pero eso viola el principio de que internal/ no debería exponer nada. 

  Mejor approach: En framesupplier.go, importo internal y llamo internal.NewSupplier().

  Déjame editar framesupplier.go para hacer el import.



∴ Thinking…

  Perfecto, ahora la API pública está conectada con la implementación interna. Marquemos supplier.go como completado y sigamos con inbox.go.

  supplier.go está completo con:
  - Struct definition
  - NewSupplier() constructor
  - Start() lifecycle
  - Stop() lifecycle
  - distributionLoop() skeleton (llama a distributeToWorkers que implementaré en distribution.go)

  Ahora sigo con inbox.go que debe implementar:
  - Publish() método (ADR-001, ADR-004)

∴ Thinking…

  Ahora implemento internal/inbox.go con el método Publish().

  Según ADR-001 y ADR-004:
  func (s *Supplier) Publish(frame *Frame) {
      s.inboxMu.Lock()

      if s.inboxFrame != nil {
          s.inboxDrops++  // Distribution hasn't consumed previous frame
      }

      s.inboxFrame = frame
      s.inboxCond.Signal()  // Wake distribution goroutine

      s.inboxMu.Unlock()
  }

  Este método debe ser no-bloqueante, siempre retornar en ~1µs.

∴ Thinking…

  Perfecto, ahora la API pública está conectada con la implementación interna. Marquemos supplier.go como completado y sigamos con inbox.go.

  supplier.go está completo con:
  - Struct definition
  - NewSupplier() constructor
  - Start() lifecycle
  - Stop() lifecycle
  - distributionLoop() skeleton (llama a distributeToWorkers que implementaré en distribution.go)

  Ahora sigo con inbox.go que debe implementar:
  - Publish() método (ADR-001, ADR-004)

∴ Thinking…

  Ahora implemento internal/inbox.go con el método Publish().

  Según ADR-001 y ADR-004:
  func (s *Supplier) Publish(frame *Frame) {
      s.inboxMu.Lock()

      if s.inboxFrame != nil {
          s.inboxDrops++  // Distribution hasn't consumed previous frame
      }

      s.inboxFrame = frame
      s.inboxCond.Signal()  // Wake distribution goroutine

      s.inboxMu.Unlock()
  }

  Este método debe ser no-bloqueante, siempre retornar en ~1µs.


---

∴ Thinking…

  Perfecto, inbox.go está listo con Publish(). Ahora sigo con distribution.go que implementa distributeToWorkers() con batching según ADR-003.

  distributeToWorkers() debe:
  1. Asignar Seq al frame (atomic increment)
  2. Snapshot de slots (sync.Map → slice)
  3. Decision tree: sequential (≤8 workers) vs parallel (>8 workers)
  4. Sequential: for loop simple
  5. Parallel: batches de 8, fire-and-forget goroutines
  6. Llamar publishToSlot() por cada worker (implementado en worker_slot.go)

  Necesito primero declarar publishToSlot() aunque lo implemente después en worker_slot.go, o puedo asumir que ya existe. Mejor lo declaro en worker_slot.go primero y luego implemento distribution.go.

  Cambiemos el orden: voy a worker_slot.go primero.

∴ Thinking…

  Implemento worker_slot.go que contiene:

  1. WorkerSlot struct (mailbox per-worker)
  2. publishToSlot() - método privado usado por distributeToWorkers
  3. Subscribe() - método público, registra worker y retorna readFunc
  4. Unsubscribe() - método público, limpia worker slot

  WorkerSlot según ADR-001:
  type WorkerSlot struct {
      mu    sync.Mutex
      cond  *sync.Cond
      frame *Frame
      
      // Stats fields
      lastConsumedAt   time.Time
      lastConsumedSeq  uint64
      consecutiveDrops uint64
      totalDrops       uint64
      closed           bool
  }

∴ Thinking…

  Excelente, worker_slot.go está completo. Ahora puedo implementar distribution.go que usa publishToSlot().

  distribution.go debe implementar distributeToWorkers() según ADR-003:
  - Asignar Seq global
  - Snapshot de slots
  - Decision tree: ≤8 sequential, >8 parallel batching
  - Fire-and-forget (no wg.Wait)

∴ Thinking…

  Ahora implemento distribution.go con distributeToWorkers() según ADR-003.

  Algoritmo (del ARCHITECTURE.md líneas 136-174):
  const publishBatchSize = 8  // Guardrail

  func (s *Supplier) distributeToWorkers(frame *Frame) {
      frame.Seq = atomic.AddUint64(&s.publishSeq, 1)

      // Snapshot slots (sync.Map → slice)
      var slots []*WorkerSlot
      s.slots.Range(func(key, value interface{}) bool {
          slots = append(slots, value.(*WorkerSlot))
          return true
      })

      workerCount := len(slots)

      // Sequential path: ≤8 workers (0 goroutines)
      if workerCount <= publishBatchSize {
          for _, slot := range slots {
              s.publishToSlot(slot, frame)
          }
          return
      }

      // Parallel path: >8 workers (fire-and-forget batching)
      for i := 0; i < workerCount; i += publishBatchSize {
          end := i + publishBatchSize
          if end > workerCount {
              end = workerCount
          }

          batch := slots[i:end]
          go func(b []*WorkerSlot) {  // No wg.Wait (fire-and-forget)
              for _, slot := range b {
                  s.publishToSlot(slot, frame)
              }
          }(batch)
      }
  }


∴ Thinking…

  Excelente, distribution.go está completo. Ahora necesito implementar stats.go con el método Stats() que retorna SupplierStats.

  Stats() debe:
  1. Leer inboxDrops (atomic)
  2. Iterar sobre slots (sync.Map.Range)
  3. Para cada slot, leer stats fields (con lock)
  4. Calcular IsIdle (time.Since > 30s)
  5. Construir WorkerStats
  6. Retornar SupplierStats

  Stats() debe ser non-blocking snapshot, no live view.

∴ Thinking…

  Implemento stats.go con el método Stats().



∴ Thinking…

  Perfecto, todos los archivos están implementados:

  ✅ Public API:
  - framesupplier.go (Frame, Supplier interface, Stats types)
  - doc.go (package documentation)
  - examples/ (client code)

  ✅ Internal implementation:
  - internal/supplier.go (struct + lifecycle)
  - internal/inbox.go (Publish)
  - internal/worker_slot.go (Subscribe/Unsubscribe/publishToSlot)
  - internal/distribution.go (distributeToWorkers with batching)
  - internal/stats.go (Stats)

  Ahora necesito compilar y verificar que todo compila correctamente.

  Pero primero debo inicializar el go module. Déjame verificar si ya existe go.mod.

∴ Thinking…

  Ahora necesito compilar. Primero verifico si existe go.mod en el módulo framesupplier.

● Search(pattern: "go.mod", path: "/home/visiona/Work/OrionWork/modules/framesupplier")
  ⎿  Found 0 files

∴ Thinking…

  No existe go.mod. Necesito inicializar el módulo. Déjame verificar si existe go.work en el root de OrionWork (multi-module workspace).

---

 Excelente! Todo está implementado y compilando correctamente. Ahora voy a hacer un resumen para Ernesto:

● Perfecto! ✅ Implementación completa y compilando.

  Resumen de lo implementado

  Estructura final (Working Backwards → API → Implementation)

  framesupplier/
  ├── go.mod                              # Module declaration
  ├── doc.go                              # Package documentation
  ├── framesupplier.go                    # PUBLIC API (interface + re-exports)
  ├── examples/
  │   ├── stream_capture_client.go        # Cliente PUBLISHER (Working Backwards)
  │   └── worker_client.go                # Cliente CONSUMER (Working Backwards)
  └── internal/
      ├── frame.go                        # Frame struct (ADR-002)
      ├── types.go                        # SupplierStats, WorkerStats
      ├── supplier.go                     # Supplier struct + lifecycle (New/Start/Stop)
      ├── inbox.go                        # Publish() + distributionLoop (ADR-001, ADR-004)
      ├── worker_slot.go                  # Subscribe/Unsubscribe + WorkerSlot (ADR-001)
      ├── distribution.go                 # distributeToWorkers + batching (ADR-003)
      └── stats.go                        # Stats() collection

  Lo que aprendimos en el camino

  1. Working Backwards funcionó perfecto: Escribimos examples/ primero → API pública emergió natural del uso real.
  2. API pública vs internal/ correcta desde el inicio:
    - framesupplier.go: Interface + type aliases (contratos estables)
    - internal/: Toda la implementación (libre refactoring sin breaking changes)
  3. Import cycles resueltos: Frame, SupplierStats, WorkerStats viven en internal/ y se re-exportan como type aliases.
  4. Modular desde día 1: Cada archivo = un ADR específico:
    - inbox.go → ADR-001, ADR-004
    - distribution.go → ADR-003
    - worker_slot.go → ADR-001
    - stats.go → Operational monitoring

  Compilación exitosa

  go build ./...         # ✅ Core module
  go build ./examples/... # ✅ Client examples

  Próximos pasos sugeridos

  ¿Querés que:
  1. Escriba tests (como pair-programming: te digo qué testear, vos ejecutás)?
  2. Validemos el diseño revisando algún archivo específico?
  3. Documentemos decisiones que emergieron durante implementación?

  "Nosotros mandamos cuando picamos código" → El código validó el diseño, compiló limpio. 🎸
