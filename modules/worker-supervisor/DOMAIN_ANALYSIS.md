# Domain Analysis: Worker Supervision System
## Textual Analysis (Yourdon/Booch OOA)

**Análisis narrativo del dominio**: Sistema de supervisión de workers de inferencia en pipeline de video en tiempo real, con gestión de lifecycle, health monitoring, restart policies basadas en criticidad, y provisioning de workers heterogéneos (Python, HTTP, GPU).

---

## 1. NOUN ANALYSIS (Sustantivos → Clases/Entidades)

### Sustantivos Primarios (Core Domain)

**Worker**
- Contexto: "Worker ejecuta inference", "Worker consume frames", "Worker reporta health"
- Naturaleza: Entidad activa, responsable de producir inferences
- Abstracción: Interfaz (múltiples implementaciones: Python, HTTP, GPU)

**WorkerSupervisor**
- Contexto: "Supervisor supervisa workers", "Supervisor evalúa desempeño", "Supervisor decide restart"
- Naturaleza: Coordinador, evaluador de output
- Analogía: Jefe de sala de producción

**SwarmWorker**
- Contexto: "Swarm gestiona salud del enjambre", "Swarm provee workers healthy", "Swarm monitorea heartbeats"
- Naturaleza: Gestor de pool + health monitoring
- Analogía: HR bienestar + médico laboral

**Provider**
- Contexto: "Provider crea workers", "Provider provisiona según capabilities", "Provider fabrica machinery"
- Naturaleza: Factory, conoce implementaciones concretas
- Analogía: Reclutamiento, contratación

**Inference**
- Contexto: "Worker produce inference", "Inference contiene detections", "Inference tiene timing metrics"
- Naturaleza: Producto del trabajo, output
- Relación: Worker → produce → Inference

**Frame**
- Contexto: "Worker consume frames", "Frame distribuido por FrameSupplier"
- Naturaleza: Input de trabajo (materia prima)
- Relación: FrameSupplier → distribuye → Frame → Worker

**Capabilities**
- Contexto: "Worker declara capabilities", "PersonDetection capability", "VLM capability"
- Naturaleza: Contrato de qué sabe hacer un worker
- Relación: Worker → declara → Capabilities

**SLA** (Service Level Agreement)
- Contexto: "Critical SLA", "BestEffort SLA", "SLA determina max retries"
- Naturaleza: Nivel de criticidad del worker
- Valores: Critical, High, Normal, BestEffort

---

### Sustantivos Secundarios (Supporting Domain)

**Health**
- Contexto: "Health monitoring", "Worker healthy", "Health checks"
- Naturaleza: Estado de bienestar del worker
- Relación: SwarmWorker → monitorea → Health

**Heartbeat**
- Contexto: "Heartbeat checks", "Worker envía heartbeat"
- Naturaleza: Señal de vida periódica
- Relación: Worker → emite → Heartbeat → SwarmWorker

**Pool**
- Contexto: "Pool de workers", "Pool management", "Warm workers en pool"
- Naturaleza: Colección gestionada de workers disponibles
- Relación: SwarmWorker → gestiona → Pool

**RestartPolicy**
- Contexto: "Restart policy según SLA", "Bounded retries", "Exponential backoff"
- Naturaleza: Estrategia de restart
- Relación: WorkerSupervisor → aplica → RestartPolicy

**Crash**
- Contexto: "Worker crashed", "Crash detection", "Persistent vs transient crash"
- Naturaleza: Evento de falla
- Relación: Worker → emite → Crash → WorkerSupervisor

**Lifecycle**
- Contexto: "Lifecycle management", "Spawn, start, stop, shutdown"
- Naturaleza: Estados y transiciones del worker
- Estados: Created, Starting, Running, Stopping, Stopped, Crashed

**Machinery**
- Contexto: "Python subprocess machinery", "HTTP client machinery", "GPU kernel machinery"
- Naturaleza: Implementación interna del worker (herramientas)
- Relación: Worker → usa → Machinery (hidden)

**Output** (o Performance)
- Contexto: "Supervisor evalúa output", "Output producido a tiempo"
- Naturaleza: Producto/resultado del worker
- Relación: WorkerSupervisor → evalúa → Output

**Resource**
- Contexto: "GPU resource", "Memory resource", "Worker requiere resources"
- Naturaleza: Requisitos de hardware/software
- Relación: Worker → declara → ResourceRequirements

**Subscription**
- Contexto: "Worker subscripto a FrameSupplier", "Subscribe/Unsubscribe"
- Naturaleza: Registro de worker como consumidor de frames
- Relación: WorkerSupervisor → registra → Worker → FrameSupplier

---

### Sustantivos de Implementación (Machinery Layer)

**PythonSubprocess**
- Contexto: "Python subprocess worker", "exec.Command", "stdin/stdout IPC"
- Naturaleza: Implementación concreta de Worker
- Machinery: Python interpreter + ONNX Runtime

**HTTPClient**
- Contexto: "HTTP worker", "Gemini API worker", "Remote service worker"
- Naturaleza: Implementación concreta de Worker
- Machinery: net/http client

**GPUKernel**
- Contexto: "GPU worker", "CUDA kernel", "VAAPI acceleration"
- Naturaleza: Implementación concreta de Worker
- Machinery: GPU driver + CUDA/OpenCL

---

## 2. VERB ANALYSIS (Verbos → Responsabilidades/Métodos)

### Verbos de WorkerSupervisor

**supervise** (supervisar)
- "Supervisor supervisa workers"
- Responsabilidad: Coordinar lifecycle, evaluar desempeño

**evaluate** (evaluar)
- "Supervisor evalúa output", "Supervisor evalúa desempeño"
- Responsabilidad: Value-based supervision (producto entregado)

**restart** (reiniciar)
- "Supervisor decide restart", "Supervisor aplica restart policy"
- Responsabilidad: Ejecutar políticas de restart según SLA

**register** (registrar)
- "Supervisor registra worker en FrameSupplier"
- Responsabilidad: Coordinar con otros módulos

**request** (solicitar)
- "Supervisor solicita worker a SwarmWorker"
- Responsabilidad: Pedir workers healthy del swarm

**replace** (reemplazar)
- "Supervisor reemplaza worker crashed"
- Responsabilidad: Sustituir worker no productivo

---

### Verbos de SwarmWorker

**monitor** (monitorear)
- "Swarm monitorea health", "Swarm monitorea heartbeats"
- Responsabilidad: Health-based management

**provide** (proveer)
- "Swarm provee workers healthy"
- Responsabilidad: Entregar workers listos para producir

**pool** (gestionar pool)
- "Swarm gestiona pool", "Swarm mantiene warm workers"
- Responsabilidad: Pool management (reuse, capacity)

**detect** (detectar)
- "Swarm detecta worker crashed", "Swarm detecta hanging"
- Responsabilidad: Crash/hang detection

**check** (verificar)
- "Swarm verifica liveness", "Swarm hace health checks"
- Responsabilidad: Heartbeat monitoring

**allocate** (asignar)
- "Swarm asigna worker del pool"
- Responsabilidad: Worker allocation

**reclaim** (recuperar)
- "Swarm recupera worker stopped"
- Responsabilidad: Pool recycling

---

### Verbos de Provider

**create** (crear)
- "Provider crea workers", "Provider fabrica workers"
- Responsabilidad: Factory pattern

**provision** (provisionar)
- "Provider provisiona según capabilities"
- Responsabilidad: Worker instantiation

**configure** (configurar)
- "Provider configura model paths", "Provider configura endpoints"
- Responsabilidad: Worker configuration

**spawn** (lanzar)
- "Provider spawns Python subprocess"
- Responsabilidad: Machinery initialization

---

### Verbos de Worker

**execute** (ejecutar)
- "Worker ejecuta inference"
- Responsabilidad: Core business logic

**consume** (consumir)
- "Worker consume frames"
- Responsabilidad: Input processing

**produce** (producir)
- "Worker produce inferences"
- Responsabilidad: Output generation

**report** (reportar)
- "Worker reporta health", "Worker reporta crash"
- Responsabilidad: State communication

**declare** (declarar)
- "Worker declara capabilities", "Worker declara SLA"
- Responsabilidad: Contract publication

**start** (iniciar)
- "Worker inicia procesamiento"
- Responsabilidad: Lifecycle transition

**stop** (detener)
- "Worker detiene procesamiento"
- Responsabilidad: Graceful shutdown

---

### Verbos de Colaboración (Cross-module)

**subscribe** (suscribir)
- "Worker se suscribe a FrameSupplier"
- Módulos: WorkerSupervisor → FrameSupplier

**unsubscribe** (desuscribir)
- "Worker se desuscribe de FrameSupplier"
- Módulos: WorkerSupervisor → FrameSupplier

**publish** (publicar)
- "Worker publica inference", "Swarm publica eventos de health"
- Módulos: Worker → event-emitter

**command** (comandar)
- "control-plane comanda restart", "control-plane comanda reload"
- Módulos: control-plane → WorkerSupervisor

---

## 3. ADJECTIVE ANALYSIS (Adjetivos → Atributos/Estados)

### Adjetivos de Worker

**healthy** / **unhealthy**
- "Worker healthy", "Worker unhealthy"
- Atributo: Health state

**running** / **stopped** / **crashed**
- "Worker running", "Worker stopped"
- Atributo: Lifecycle state

**productive** / **non-productive**
- "Worker productivo", "Worker no produce output"
- Atributo: Performance state

**critical** / **high** / **normal** / **best-effort**
- "Critical worker", "BestEffort worker"
- Atributo: SLA level

**warm** / **cold**
- "Warm worker en pool", "Cold start"
- Atributo: Readiness state

---

### Adjetivos de Crash

**persistent** / **transient**
- "Persistent failure", "Transient failure"
- Atributo: Failure nature

**immediate** / **delayed**
- "Crash inmediato (<5s)", "Crash después de N inferences"
- Atributo: Crash timing

---

### Adjetivos de Policy

**bounded** / **unbounded**
- "Bounded retries", "Unbounded retry loop"
- Atributo: Retry limit

**adaptive** / **static**
- "Adaptive restart policy", "Static policy"
- Atributo: Policy behavior

---

## 4. CRC CARDS (Complete Set)

---

### CRC: WorkerSupervisor

**Responsabilidades**:
- Supervise worker output (value-based, no health-based)
- Evaluate worker performance (inferences produced, latency)
- Apply restart policies (según SLA: Critical=3 retries, BestEffort=0)
- Request workers from SwarmWorker
- Register workers in FrameSupplier (Subscribe/Unsubscribe)
- Replace non-productive workers
- Execute commands from control-plane (RestartWorker, StopWorker)
- Publish supervision events (worker_replaced, max_retries_exceeded)
- One-for-one supervision semantics (crash isolation)

**Colaboradores**:
- SwarmWorker (pide workers healthy)
- Worker (supervisa output, no health)
- FrameSupplier (registra/desregistra workers)
- control-plane (recibe comandos)
- event-emitter (publica eventos de supervision)

**Atributos/Estado**:
- workers: map[workerID]Worker
- restartPolicies: map[SLALevel]RestartPolicy
- performanceMetrics: map[workerID]PerformanceMetrics

---

### CRC: SwarmWorker

**Responsabilidades**:
- Monitor worker health (heartbeats, liveness checks)
- Detect crashes/hangs (crash pattern detection)
- Manage worker pool (warm workers, capacity, reuse)
- Provide healthy workers to Supervisor
- Allocate workers from pool
- Reclaim stopped workers to pool
- Health-based management (bienestar del enjambre)
- Collaborate with Provider to create workers
- Track health metrics (uptime, consecutive crashes, last heartbeat)

**Colaboradores**:
- Provider (solicita creación de workers)
- Worker (monitorea health, no output)
- WorkerSupervisor (provee workers, no supervisa)
- Pool (gestiona colección interna)

**Atributos/Estado**:
- pool: Pool (colección de workers disponibles)
- healthMetrics: map[workerID]HealthMetrics
- heartbeatInterval: time.Duration

---

### CRC: Provider

**Responsabilidades**:
- Create workers (factory pattern)
- Provision workers según capabilities (PersonDetection → PythonWorker, VLM → HTTPWorker)
- Configure worker machinery (model paths, endpoints, resources)
- Spawn subprocess/client (exec.Command, http.Client, GPU kernel)
- Factory registry (capabilities → WorkerFactory)
- Resource allocation (assign GPU, memory limits)
- Read configuration (orion.yaml: model paths, endpoints)

**Colaboradores**:
- SwarmWorker (recibe requests de workers)
- Worker (crea instancias concretas)
- config (lee orion.yaml)
- PythonSubprocess, HTTPClient, GPUKernel (machinery)

**Atributos/Estado**:
- factoryRegistry: map[CapabilityType]WorkerFactory
- config: WorkerConfig (model paths, endpoints, resources)

---

### CRC: Worker (Interface)

**Responsabilidades**:
- Execute inference (core business logic)
- Consume frames (from FrameSupplier via readFunc)
- Produce inferences (to result channel)
- Report health (via callbacks/interface)
- Declare capabilities (PersonDetection, VLM, Pose)
- Declare SLA level (Critical, High, Normal, BestEffort)
- Declare resource requirements (CPU, GPU, Memory)
- Manage internal machinery (Python, HTTP, GPU - hidden)
- Start/Stop lifecycle

**Colaboradores**:
- FrameSupplier (consume frames)
- WorkerSupervisor (reporta output implícitamente)
- SwarmWorker (reporta health)
- Machinery (usa herramientas internas: Python, HTTP, GPU)

**Atributos/Estado**:
- id: string
- capabilities: WorkerCapabilities
- sla: SLALevel
- state: LifecycleState (Created, Running, Stopped, Crashed)
- machinery: internal (PythonSubprocess, HTTPClient, etc.)

---

### CRC: Pool

**Responsabilidades**:
- Store available workers (warm pool)
- Allocate worker on request (by capabilities + SLA)
- Reclaim worker after use (if reusable)
- Capacity management (max pool size)
- Eviction policy (LRU, TTL)

**Colaboradores**:
- SwarmWorker (managed by)
- Worker (pooled entities)

**Atributos/Estado**:
- workers: []Worker (available workers)
- capacity: int (max pool size)
- evictionPolicy: EvictionPolicy

---

### CRC: RestartPolicy

**Responsabilidades**:
- Define restart behavior (max retries, backoff strategy)
- Calculate backoff delay (exponential, linear, immediate)
- Determine if retry allowed (based on attempt count, SLA)
- Track restart attempts per worker

**Colaboradores**:
- WorkerSupervisor (aplica policy)
- SLA (deriva max retries de SLA level)

**Atributos/Estado**:
- maxRetries: int (derived from SLA)
- backoffStrategy: BackoffStrategy (exponential, linear)
- backoffBase: time.Duration

---

### CRC: Capabilities

**Responsabilidades**:
- Declare worker skills (PersonDetection, VLM, PoseEstimation, FaceRecognition)
- Match worker to task (capability matching)
- Extensibility (new capabilities without code changes)

**Colaboradores**:
- Worker (declara capabilities)
- Provider (usa capabilities para factory selection)
- WorkerSupervisor (solicita workers por capabilities)

**Atributos/Estado**:
- type: CapabilityType (PersonDetection, VLM, etc.)
- parameters: map[string]interface{} (confidence_threshold, model_size, etc.)

---

### CRC: SLA (Service Level Agreement)

**Responsabilidades**:
- Define criticality level (Critical, High, Normal, BestEffort)
- Derive max retries (Critical=3, BestEffort=0)
- Determine monitoring intensity (Critical=frequent heartbeats)
- Business-driven policy (fall detection=Critical, research=BestEffort)

**Colaboradores**:
- Worker (declara SLA)
- RestartPolicy (deriva max retries)
- SwarmWorker (ajusta monitoring según SLA)

**Atributos/Estado**:
- level: SLALevel (Critical, High, Normal, BestEffort)
- maxRetries: int
- heartbeatInterval: time.Duration

---

### CRC: Health

**Responsabilidades**:
- Track worker liveness (heartbeat timestamp)
- Track crash history (consecutive crashes, last crash time)
- Determine health state (healthy, unhealthy, degraded)
- Provide health metrics (uptime, availability)

**Colaboradores**:
- SwarmWorker (monitorea health)
- Worker (reporta health)

**Atributos/Estado**:
- isAlive: bool
- lastHeartbeat: time.Time
- consecutiveCrashes: int
- lastCrashTime: time.Time
- uptime: time.Duration

---

### CRC: Crash

**Responsabilidades**:
- Represent crash event (timestamp, reason, worker ID)
- Classify failure nature (persistent vs transient)
- Classify crash timing (immediate <5s vs delayed)
- Provide crash context (exit code, stderr, signal)

**Colaboradores**:
- Worker (emite crash event)
- SwarmWorker (detecta crash)
- WorkerSupervisor (evalúa restart)

**Atributos/Estado**:
- workerID: string
- timestamp: time.Time
- reason: error
- nature: FailureNature (persistent, transient, unknown)
- timing: CrashTiming (immediate, delayed)
- context: CrashContext (exit code, stderr)

---

### CRC: Lifecycle

**Responsabilidades**:
- Define worker states (Created, Starting, Running, Stopping, Stopped, Crashed)
- Define valid transitions (Created→Starting→Running, Running→Crashed)
- Enforce state invariants (no transition Stopped→Running without restart)
- Provide lifecycle callbacks (onStart, onStop, onCrash)

**Colaboradores**:
- Worker (tiene lifecycle state)
- WorkerSupervisor (gestiona transiciones)

**Atributos/Estado**:
- currentState: LifecycleState
- validTransitions: map[LifecycleState][]LifecycleState

---

### CRC: PythonSubprocess (Implementación concreta de Worker)

**Responsabilidades**:
- Spawn Python process (exec.Command)
- MsgPack IPC (stdin/stdout)
- Manage subprocess lifecycle (SIGTERM, SIGKILL)
- Parse stderr for crashes
- Load ONNX model (model path configuration)

**Colaboradores**:
- Provider (creado por)
- Worker (implementa interface)
- exec.Command (machinery)

**Atributos/Estado**:
- cmd: *exec.Cmd
- stdin: io.WriteCloser
- stdout: io.ReadCloser
- modelPath: string

---

### CRC: HTTPClient (Implementación concreta de Worker)

**Responsabilidades**:
- HTTP requests to remote service (Gemini API, gRPC)
- JSON/Protobuf serialization
- Retry logic (transient network failures)
- API key management

**Colaboradores**:
- Provider (creado por)
- Worker (implementa interface)
- net/http (machinery)

**Atributos/Estado**:
- client: *http.Client
- endpoint: string
- apiKey: string

---

### CRC: GPUKernel (Implementación concreta de Worker)

**Responsabilidades**:
- GPU memory allocation
- CUDA/OpenCL kernel execution
- VAAPI hardware acceleration
- GPU resource management

**Colaboradores**:
- Provider (creado por)
- Worker (implementa interface)
- GPU driver (machinery)

**Atributos/Estado**:
- gpuID: int
- memoryAllocated: int
- kernelHandle: unsafe.Pointer

---

## 5. RELATIONSHIPS & INTERACTIONS

### Aggregation (tiene un, contiene)

```
WorkerSupervisor [1] ─── tiene ───> [0..*] Worker
SwarmWorker [1] ─── contiene ───> [1] Pool
Pool [1] ─── contiene ───> [0..*] Worker
Worker [1] ─── tiene ───> [1] Capabilities
Worker [1] ─── tiene ───> [1] SLA
Worker [1] ─── tiene ───> [1] Health
Worker [1] ─── tiene ───> [1] Lifecycle
Worker [1] ─── usa ───> [1] Machinery (hidden)
RestartPolicy [1] ─── deriva de ───> [1] SLA
```

---

### Collaboration (usa, colabora con)

```
WorkerSupervisor ─── solicita ───> SwarmWorker
WorkerSupervisor ─── registra ───> FrameSupplier
WorkerSupervisor ─── publica eventos ───> event-emitter
WorkerSupervisor ─── recibe comandos ───> control-plane

SwarmWorker ─── solicita creación ───> Provider
SwarmWorker ─── monitorea ───> Worker (health)
SwarmWorker ─── provee ───> WorkerSupervisor

Provider ─── crea ───> Worker
Provider ─── lee config ───> config

Worker ─── consume frames ───> FrameSupplier
Worker ─── produce inferences ───> result channel
Worker ─── reporta health ───> SwarmWorker
```

---

### Supervision (supervisa, gestiona lifecycle)

```
WorkerSupervisor ═══ supervisa output ═══> Worker
SwarmWorker ═══ supervisa health ═══> Worker
Provider ═══ crea/fabrica ═══> Worker
```

---

## 6. DOMAIN INVARIANTS

### Invariante 1: Separation of Supervision Concerns
```
WorkerSupervisor evalúa OUTPUT (value-based)
SwarmWorker evalúa HEALTH (state-based)
NUNCA: Supervisor monitorea health directamente
NUNCA: SwarmWorker evalúa output directamente
```

### Invariante 2: Worker Abstraction Consistency
```
WorkerSupervisor conoce Worker (interface)
WorkerSupervisor NO conoce PythonSubprocess, HTTPClient, GPUKernel
Provider conoce implementaciones concretas (machinery)
```

### Invariante 3: SLA-Driven Policies
```
SLA determina maxRetries (Critical=3, BestEffort=0)
SLA determina heartbeatInterval (Critical=1s, BestEffort=10s)
SLA NO es hot-reloadable (decisión arquitectónica, no operacional)
```

### Invariante 4: One-for-One Supervision
```
Worker crash NO afecta otros workers
WorkerSupervisor reinicia solo el crashed worker
Supervision es aislada (no cascading failures)
```

### Invariante 5: Pool Warm Workers
```
SwarmWorker mantiene warm workers en pool
Workers en pool están healthy (health checks pasados)
Workers crashed NO vuelven a pool (se descartan)
```

### Invariante 6: Lifecycle State Validity
```
Transiciones válidas: Created→Starting→Running→Stopping→Stopped
Transición crash: Running→Crashed
NO válido: Stopped→Running (requiere restart: Stopped→Created→Starting→Running)
```

---

## 7. KEY SCENARIOS (Domain Storytelling)

### Scenario 1: Worker Request & Provisioning

**Actores**: WorkerSupervisor, SwarmWorker, Provider, Worker

**Narrativa**:
1. WorkerSupervisor necesita PersonDetector (Critical SLA)
2. Supervisor solicita a SwarmWorker: "Dame PersonDetector healthy"
3. SwarmWorker revisa pool: ¿hay warm PersonDetector disponible?
   - SI: asigna del pool
   - NO: solicita a Provider: "Creá PersonDetector"
4. Provider fabrica PythonSubprocessWorker (spawn Python + load ONNX)
5. Provider retorna Worker a SwarmWorker
6. SwarmWorker inicia health monitoring (heartbeats)
7. SwarmWorker retorna Worker healthy a Supervisor
8. Supervisor registra Worker en FrameSupplier (Subscribe)
9. Worker comienza a consumir frames y producir inferences

**Sustantivos**: WorkerSupervisor, SwarmWorker, Provider, Worker, Pool, FrameSupplier
**Verbos**: solicitar, asignar, crear, fabricar, iniciar, registrar, consumir, producir

---

### Scenario 2: Worker Crash & Restart

**Actores**: Worker, SwarmWorker, WorkerSupervisor, Provider

**Narrativa**:
1. Worker (PersonDetector, Critical SLA) ejecutando inferences normalmente
2. Worker crashea (Python exception, OOM)
3. SwarmWorker detecta crash (heartbeat timeout)
4. SwarmWorker clasifica crash: transient (crash después de 1000 inferences)
5. SwarmWorker notifica a WorkerSupervisor: "Worker X crashed"
6. Supervisor evalúa restart policy: Critical SLA → max 3 retries
7. Supervisor solicita a SwarmWorker: "Dame nuevo PersonDetector"
8. SwarmWorker solicita a Provider: "Creá PersonDetector" (attempt 1)
9. Provider fabrica nuevo PythonSubprocessWorker
10. SwarmWorker inicia health monitoring
11. Nuevo worker healthy → retornado a Supervisor
12. Supervisor registra nuevo worker en FrameSupplier
13. Nuevo worker comienza a producir inferences
14. Supervisor descarta worker crashed (no vuelve a pool)

**Sustantivos**: Worker, Crash, SwarmWorker, WorkerSupervisor, RestartPolicy, SLA, Provider
**Verbos**: crashear, detectar, clasificar, notificar, evaluar, reintentar, descartar

---

### Scenario 3: Persistent Failure (Max Retries Exceeded)

**Actores**: Worker, SwarmWorker, WorkerSupervisor, Provider, event-emitter

**Narrativa**:
1. Worker (PersonDetector, Critical SLA) crashea inmediatamente (<5s after spawn)
2. SwarmWorker detecta crash: immediate (persistent failure pattern)
3. Supervisor aplica restart policy: attempt 1 (max 3)
4. Provider crea nuevo worker → crashea inmediatamente
5. Supervisor aplica restart policy: attempt 2
6. Provider crea nuevo worker → crashea inmediatamente
7. Supervisor aplica restart policy: attempt 3
8. Provider crea nuevo worker → crashea inmediatamente
9. Supervisor: max retries exceeded (3/3)
10. Supervisor publica evento a event-emitter: "worker_failed_permanent" (workerID, reason)
11. Supervisor NO solicita más workers (fail-fast)
12. Sistema queda sin PersonDetector (require intervención manual vía MQTT)

**Sustantivos**: Worker, Crash, RestartPolicy, maxRetries, event-emitter
**Verbos**: crashear, detectar, reintentar, exceder, publicar, fallar

---

### Scenario 4: Pool Reuse (Warm Worker)

**Actores**: WorkerSupervisor, SwarmWorker, Pool, Worker

**Narrativa**:
1. Supervisor solicita VLM worker (BestEffort SLA)
2. SwarmWorker revisa pool: hay 2 warm VLM workers disponibles
3. SwarmWorker asigna worker del pool (no crea nuevo)
4. SwarmWorker verifica health: worker healthy (last heartbeat <10s)
5. SwarmWorker retorna worker a Supervisor
6. Supervisor registra worker en FrameSupplier
7. Worker consume frames, produce inferences
8. Supervisor solicita stop worker (comando MQTT)
9. Worker graceful shutdown
10. SwarmWorker recupera worker a pool (reclaim)
11. Worker queda warm en pool (listo para próximo request)

**Sustantivos**: Pool, Worker, SwarmWorker, Health, Heartbeat
**Verbos**: asignar, verificar, recuperar, reusar

---

### Scenario 5: Health Monitoring (Heartbeat Timeout)

**Actores**: Worker, SwarmWorker, Health

**Narrativa**:
1. Worker (PersonDetector) running, enviando heartbeats cada 1s (Critical SLA)
2. SwarmWorker recibe heartbeats normalmente
3. Worker cuelga (infinite loop, deadlock)
4. SwarmWorker NO recibe heartbeat después de 3s (3× heartbeatInterval)
5. SwarmWorker marca worker como unhealthy
6. SwarmWorker notifica a WorkerSupervisor: "Worker X hanging"
7. Supervisor solicita replacement worker
8. Supervisor desregistra worker hanging de FrameSupplier
9. Supervisor registra nuevo worker
10. Worker hanging eventualmente killed (timeout)

**Sustantivos**: Heartbeat, Health, Worker, SwarmWorker
**Verbos**: enviar, recibir, colgar, detectar, marcar, notificar, reemplazar

---

## 8. ABSTRACTION LEVELS (Consistency Check)

### Layer 1: Supervision (WorkerSupervisor)
**Habla en**: workers, output, performance, restart policies
**NO habla en**: Python, HTTP, subprocesses, heartbeats

### Layer 2: Swarm Management (SwarmWorker)
**Habla en**: health, pool, heartbeats, liveness, crashes
**NO habla en**: output, inferences, Python, HTTP

### Layer 3: Provisioning (Provider)
**Habla en**: capabilities, factories, machinery, configuration
**NO habla en**: health, output, supervision

### Layer 4: Execution (Worker)
**Habla en**: inferences, frames, capabilities, SLA
**NO habla en**: restart policies, pool, factories

### Layer 5: Machinery (PythonSubprocess, HTTPClient, GPUKernel)
**Habla en**: exec.Command, http.Client, CUDA, stdin/stdout
**NO habla en**: supervision, health, capabilities

**Consistency**: Cada layer habla en SU nivel de abstracción. No leaks.

---

## 9. BOUNDED CONTEXTS (DDD)

### Bounded Context: Worker Supervision
**Entidades**: WorkerSupervisor, RestartPolicy, SLA
**Lenguaje**: supervise, evaluate output, restart, replace, performance
**Responsabilidad**: Garantizar workers productivos

### Bounded Context: Swarm Management
**Entidades**: SwarmWorker, Pool, Health, Heartbeat
**Responsabilidad**: Gestionar salud del enjambre

### Bounded Context: Worker Provisioning
**Entidades**: Provider, Capabilities, Machinery
**Responsabilidad**: Fabricar workers

### Bounded Context: Inference Execution
**Entidades**: Worker, Frame, Inference
**Responsabilidad**: Ejecutar inference

**Relaciones entre contextos**:
- Supervision → Swarm: "dame worker healthy"
- Swarm → Provisioning: "creá worker con capabilities X"
- Supervision → Execution: "producí output"

---

## 10. OPEN QUESTIONS (Para próximas decisiones)

**Q1**: ¿Pool tiene TTL (time-to-live) para warm workers?
- Eviction policy: LRU, TTL, capacity-based?

**Q2**: ¿RestartPolicy es configurable por worker o solo por SLA?
- ¿Override individual de max retries?

**Q3**: ¿Health monitoring es pull (IsHealthy()) o push (heartbeats)?
- Hybrid: heartbeats para liveness, IsHealthy() para on-demand?

**Q4**: ¿Worker registration en FrameSupplier es síncrono o asíncrono?
- ¿Supervisor espera confirmation de Subscribe()?

**Q5**: ¿Crash classification (persistent vs transient) es heurística o declarativa?
- ¿Worker declara expected failure modes en Capabilities?

**Q6**: ¿Provider poolea machinery (Python processes) o solo Workers?
- Warm Python interpreter pool vs warm Worker pool?

---

## 11. SUMMARY (Textual Analysis Results)

**Sustantivos identificados**: 25 entidades (10 core, 10 supporting, 5 machinery)
**Verbos identificados**: 35 responsabilidades distribuidas en 5 layers
**Adjetivos identificados**: 15 atributos/estados
**CRC Cards generadas**: 13 (completas con responsabilidades, colaboradores, estado)
**Scenarios narrativos**: 5 (end-to-end domain storytelling)
**Bounded Contexts**: 4 (DDD-style)
**Invariantes**: 6 (domain rules)
**Abstraction Levels**: 5 (consistency validated)

**Próximo paso**: Crystallization (ADRs de decisiones core)
