# Care Scene - Visión de Arquitectura + Negocio

> **Charla de Terraza con Café** ☕
> Documento maestro que captura la visión completa: arquitectura técnica, modelo de negocio, y estrategia de evolución.

---

## 🎯 Contexto: ¿Qué es Care Scene?

**Care Scene** es un sistema de monitoreo inteligente ambiental (no invasivo) para residencias geriátricas, basado en:
- 🎥 Visión por computadora (cámaras IP)
- 🧠 AI Edge (procesamiento local)
- 📡 Event-driven architecture (MQTT + Temporal.io)
- 🎪 Orquestación distribuida (Room → Tent → Circus)

**Principio fundamental:** "Orion ve. Expertos entienden. Orchestrator maneja el circo."

---

## 🏗️ Arquitectura en Capas

### **Capa 1: Care Cell (Edge - i7 Mini PC)**

**Hardware:** i7 NUC (mini NVR) por residencia
**Scope:** 1 residencia pequeña (8 habitaciones max)
**Deployment:** On-premise (privacidad + offline-first)

```
┌─────────────────────────────────────────────────────┐
│              CARE CELL (i7 Mini PC)                 │
│          "Edge Orchestration Unit"                  │
│                                                     │
│  ┌───────────────────────────────────────────────┐ │
│  │         MQTT (Coreografía Local)              │ │
│  │                                               │ │
│  │  Orion → MQTT → Scene Experts                │ │
│  │  Experts → MQTT → Room Orchestrators         │ │
│  │  Room Orch → MQTT → Orion (comandos)         │ │
│  │                                               │ │
│  │  ✅ Latencia <100ms (crítico safety)         │ │
│  │  ✅ Offline-first (funciona sin internet)    │ │
│  │  ✅ Privacy (video nunca sale del edge)      │ │
│  └───────────────────────────────────────────────┘ │
│                                                     │
│  Componentes:                                       │
│  ├─ 8 Room Orchestrators (1 por habitación)        │
│  ├─ 8 Orions (AI edge, inferencias)                │
│  ├─ Scene Experts Mesh (Sleep, Edge, Exit, etc.)   │
│  └─ Expert Graph Service (scenarios)               │
└─────────────────────────────────────────────────────┘
```

**Por qué MQTT aquí:**
- ✅ Ultra low latency (<100ms edge-to-action)
- ✅ Offline-first (Care Cell funciona sin internet)
- ✅ Privacy (video nunca sale del edge)
- ✅ Minimal dependencies

---

### **Capa 2: Temporal.io + EventStore (Cloud)**

**Scope:** Multi-facility (100+ residencias)
**Deployment:** Cloud (Temporal Cloud o self-hosted cluster)

```
┌─────────────────────────────────────────────────────┐
│         TEMPORAL.IO + EventStore                    │
│    "Circus Owner - Global Orchestration"            │
│                                                     │
│  ┌───────────────────────────────────────────────┐ │
│  │    Temporal Workflows (Orquestación)          │ │
│  │                                               │ │
│  │  SupervisorWorkflow (evalúa decisiones)      │ │
│  │  FacilityWorkflow (gestión residencia)       │ │
│  │  DiscoveryWorkflow (detecta oportunidades)   │ │
│  │  ComplianceWorkflow (auditoría)              │ │
│  │  PolicyOptimizationWorkflow (A/B testing)    │ │
│  └───────────────────────────────────────────────┘ │
│                                                     │
│  ┌───────────────────────────────────────────────┐ │
│  │    EventStore (Event Sourcing)                │ │
│  │                                               │ │
│  │  - Todos los eventos de Care Cells            │ │
│  │  - Gemelo Digital por habitación              │ │
│  │  - Proyecciones para analytics                │ │
│  └───────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

**Por qué Temporal aquí:**
- ✅ Workflows de largo plazo (días/semanas/meses)
- ✅ Sagas complejas (emergency response)
- ✅ Visibilidad total (Temporal UI)
- ✅ Auditoría completa (compliance)

---

## 🎭 Coreografía vs Orquestación (Estrategia Híbrida)

### **Regla de Oro:**

> **"Usar orquestación DENTRO del bounded context, coreografía ENTRE bounded contexts."**
> — Yan Cui, "Choreography vs Orchestration in Serverless"

### **Aplicado a Care Scene:**

```
┌────────────────────────────────────────────────────────┐
│         CARE CELL (Bounded Context = Residencia)       │
│                                                        │
│  COREOGRAFÍA (MQTT) - Eventos ligeros, low latency    │
│                                                        │
│  Orion → MQTT → Experts (fire & forget)               │
│  Experts → MQTT → Room Orch (events)                  │
│  Room Orch → MQTT → Orion (commands)                  │
│                                                        │
│  ✅ Ultra low latency (<100ms)                        │
│  ✅ Offline-first                                     │
│  ✅ Privacy (data no sale del edge)                   │
│  ✅ Fault isolation                                   │
└────────────────────────────────────────────────────────┘
                    │
                    │ Eventos enriquecidos
                    │ (critical_event, pattern_anomaly)
                    ▼
┌────────────────────────────────────────────────────────┐
│       TEMPORAL.IO (Bounded Context = Corporativo)      │
│                                                        │
│  ORQUESTACIÓN (Temporal Workflows) - Largo plazo      │
│                                                        │
│  FacilityWorkflow → TentWorkflow → RoomWorkflow       │
│  SupervisorWorkflow (evalúa decisiones)               │
│  DiscoveryWorkflow (detecta oportunidades)            │
│  ComplianceWorkflow (auditoría regulatoria)           │
│                                                        │
│  ✅ Workflows de días/semanas/meses                   │
│  ✅ Sagas (compensating transactions)                 │
│  ✅ Visibilidad total (Temporal UI)                   │
│  ✅ Auditoría completa                                │
└────────────────────────────────────────────────────────┘
```

---

## 🤖 Componentes Principales

### **1. Orion (Care Streamer)**

**Rol:** Sensor inteligente headless que infiere y emite.

**Responsabilidades:**
- ✅ Procesa streams RTSP (LQ continuo + HQ on-demand)
- ✅ Ejecuta modelos AI (person, pose, flow)
- ✅ Emite inferencias estructuradas vía MQTT
- ✅ Recibe configuración dinámica (control plane)
- ✅ Reporta health y métricas

**NO hace:**
- ❌ NO interpreta eventos clínicos
- ❌ NO decide severidades
- ❌ NO genera alertas

**Analogía:** Radiólogo que lee placas. Ve huesos, reporta lo que ve. NO diagnostica.

**Documentación:**
- [ORION_README.md](ORION_README.md)
- [ORION_SERVICE_DESIGN.md](ORION_SERVICE_DESIGN%20(v1).md) ⭐
- [ORION_ROADMAP.md](ORION_ROADMAP.md)

---

### **2. Scene Experts (Mallado de Expertos)**

**Rol:** Mallado de expertos especializados que interpretan la escena.

**Expertos:**
- **SleepExpert** 😴 - Estados de sueño (deep, light, restless, awake)
- **EdgeExpert** 🪑 - Sentarse al borde de cama
- **ExitExpert** 🚶 - Salida de cama
- **CaregiverExpert** 👤 - Presencia de cuidador
- **PostureExpert** 🛏️ - Posturas de riesgo

**Activación dinámica:**
```
Deep sleep → SleepExpert + CaregiverExpert (2/5)
Restless → + EdgeExpert (3/5)
Edge confirmed → + ExitExpert (4/5)
Exit confirmed → Solo ExitExpert (1/5, máximo foco)
```

**Cada experto:**
- ✅ Escucha solo inferencias de su dominio
- ✅ Mantiene contexto temporal específico
- ✅ Emite eventos especializados
- ✅ Declara qué necesita de Orion

**Analogía:** Médicos especialistas. Sleep expert = neurólogo. Edge expert = geriatra.

**Documentación:**
- [SCENE_EXPERTS_MESH.md](SCENE_EXPERTS_MESH.md) ⭐
- [SCENE_EXPERTS_TEMPORAL_CONTEXT.md](SCENE_EXPERTS_TEMPORAL_CONTEXT.md)

---

### **3. Room Orchestrator (Director de Carpa)**

**Rol:** Gestiona UNA habitación (1 cama, 1 residente, 1 Orion).

**Responsabilidades:**
- ✅ Consulta Expert Graph Service (scenarios)
- ✅ Activa/desactiva expertos dinámicamente
- ✅ Configura Orion (LQ ↔ HQ)
- ✅ Responde a eventos en <100ms
- ✅ Reporta TODAS sus decisiones al Supervisor

**NO hace:**
- ❌ NO gestiona múltiples habitaciones
- ❌ NO gestiona budget HQ global
- ❌ NO prioriza entre habitaciones
- ❌ NO hace evictions (no hay competencia)

**Por qué KISS (una carpa a la vez):**
- ✅ Autonomous operation (no espera cloud)
- ✅ Fault isolation (si room 302 cae, room 303 sigue)
- ✅ Simple de testear (un solo flujo)
- ✅ Escalable horizontalmente (30 rooms = 30 containers)

**Analogía:** Encargado de una carpa en el circo. Gestiona su espectáculo, NO todo el circo.

**Documentación:**
- [ROOM_ORCHESTRATOR_README.md](ROOM_ORCHESTRATOR_README.md)
- [ROOM_ORCHESTRATOR_DESIGN.md](ROOM_ORCHESTRATOR_DESIGN.md) ⭐

---

### **4. Expert Graph Service (Package Manager de Expertos)**

**Rol:** Conoce el grafo de dependencias entre expertos y arma el "plantel" según scenarios.

**Responsabilidades:**
- ✅ Almacena manifests de expertos (como package.json)
- ✅ Resuelve dependencias (como npm install)
- ✅ Calcula orden de activación (topological sort)
- ✅ Valida compatibilidad de versiones
- ✅ Define scenarios predefinidos (bed_exit_monitoring, etc.)

**Scenarios ejemplo:**
```yaml
bed_exit_monitoring:
  experts: [sleep, edge, exit]
  activation: progressive

posture_risk_monitoring:
  experts: [sleep, posture, caregiver]
  activation: continuous

sleep_quality_baseline:
  experts: [sleep]
  activation: passive
```

**Analogía:** Docker Compose. Orchestrator dice "necesito bed_exit_monitoring", Graph Service arma el stack.

**Documentación:**
- [EXPERT_GRAPH_SERVICE_DESIGN.md](EXPERT_GRAPH_SERVICE_DESIGN.md) ⭐

---

### **5. Temporal Supervisor (Evaluador Continuo)**

**Rol:** Supervisa Room Orchestrators, evalúa decisiones, aprende, mejora políticas.

**Responsabilidades:**
- ✅ Recibe TODAS las decisiones de Room Orch (post-facto, no blocking)
- ✅ Correlaciona decisión → outcome (¿fue buena decisión?)
- ✅ Evalúa performance (falsos positivos, tasa de acierto)
- ✅ Aprende y ajusta políticas (A/B testing)
- ✅ Mantiene Gemelo Digital de cada habitación
- ✅ Descubre oportunidades de mejora (Discovery Workflow)

**Patrón clave: "Autonomous Operator + Supervisor Evaluator"**

```python
@workflow.defn
class SupervisorWorkflow:
    """Supervisa, evalúa, aprende, mejora."""

    async def run(self, room_id: str):
        while True:
            # 1. Recibe decisión de Room Orch
            decision = await wait_for_decision()

            # 2. Almacena en EventStore
            store_decision(decision)

            # 3. Espera outcome (30min timeout)
            outcome = await wait_for_outcome(30min)

            # 4. Evalúa: ¿fue buena decisión?
            eval = evaluate(decision, outcome)

            # 5. Aprende y ajusta políticas
            if eval.bad_decision:
                adjust_policy()
                send_new_policy_to_room_orch()
```

**Documentación:**
- Ver sección "Temporal Workflows" abajo

---

## 🧬 Gemelo Digital (Digital Twin)

**Concepto:** Cada habitación tiene un gemelo digital en Temporal.

```
┌─────────────────────────────────────────────────────┐
│      Digital Twin de Room 302 (en Temporal)        │
├─────────────────────────────────────────────────────┤
│                                                     │
│  state:                                             │
│    room_id: "302"                                   │
│    resident: jose_302                               │
│    current_scenario: bed_exit_monitoring            │
│    active_experts: [sleep, edge, exit]              │
│    orion_config: {stream: "HQ", fps: 12}            │
│    decisions_history: [...]                         │
│                                                     │
│  # Recibe TODOS los eventos del Room Orch real     │
│  on_event(event):                                   │
│    update_state(event)                              │
│    store_to_eventstore(event)                       │
│                                                     │
│  # Permite simulaciones                             │
│  simulate("¿qué pasa si cambio threshold X?")       │
│                                                     │
│  # Analytics & ML                                   │
│  train_policy_model(decisions_history)              │
└─────────────────────────────────────────────────────┘
```

**Ventajas:**
- ✅ Estado exacto del Room Orch (eventualmente consistente)
- ✅ Historial completo (event sourcing)
- ✅ Simulaciones ("what-if" analysis)
- ✅ ML training (políticas optimizadas)
- ✅ Compliance (auditoría completa)

---

## 📊 Data Plane vs Control Plane "Hacia Arriba"

### **Data Plane (High Volume, Low Latency)**

```
Edge (Room Orch):
├─ Inferencias de Orion (10/s) → LOCAL (no sube)
├─ Eventos de Experts (1/s) → LOCAL (no sube)
└─ Eventos CRÍTICOS → SÍ SUBE (fall, exit confirmed)

Cloud (Temporal):
└─ Recibe:
   ├─ SAMPLE de inferencias (1%)
   └─ TODOS los eventos críticos (100%)
```

**Estrategia de muestreo:**
- Inferencias normales: 1% sample
- Eventos críticos: 100%
- Decisiones de Room Orch: 100%

---

### **Control Plane (Todas las Decisiones)**

```
Room Orch → Temporal Supervisor
├─ Activar/desactivar experto → 100%
├─ Cambiar config Orion → 100%
├─ Activar ROI temporal → 100%
├─ Cambiar scenario → 100%
└─ Pausar/reanudar → 100%
```

**Por qué reportar TODAS las decisiones:**
- ✅ Evaluación continua (aprende qué funciona)
- ✅ A/B testing de políticas
- ✅ Compliance (auditoría completa)
- ✅ Discovery (detecta oportunidades)

---

## 🚀 Temporal Workflows Clave

### **1. SupervisorWorkflow (Evaluación Continua)**

```python
@workflow.defn
class SupervisorWorkflow:
    """Supervisa Room Orch, evalúa decisiones, aprende."""

    async def run(self, room_id: str):
        while True:
            # Recibe decisión
            decision = await wait_for_decision()

            # Almacena
            store_decision_eventstore(decision)

            # Espera outcome (30min)
            outcome = await wait_for_outcome(timeout=30min)

            # Evalúa
            eval = evaluate(decision, outcome)
            # {score: 0.95, classification: "true_positive"}

            # Si mal, ajusta
            if eval.score < 0.7:
                add_to_training_dataset(decision, outcome)

            # Cada 100 evals, reentrenar
            if eval_count % 100 == 0:
                PolicyOptimizationWorkflow.start()
```

**Evaluación ejemplo:**
```
Decision: Activar EdgeExpert (porque sleep.restless)
Outcome (10min después): edge_of_bed.confirmed
Eval: score=0.95 (true positive) ✅
```

---

### **2. DiscoveryWorkflow (Consultivo B2B)**

```python
@workflow.defn
class DiscoveryWorkflow:
    """Descubre oportunidades de valor para el cliente."""

    async def run(self, client_id: str, room_id: str):
        # Fase 1: POC (mes 1)
        await poc_phase(room_id)

        # Fase 2: Measuring (mes 2-3)
        insights = await measuring_phase(room_id, days=60)
        # {
        #   "microdespertares_per_night": 3.2,
        #   "false_positive_rate": 0.18,
        #   "detection_coverage": 0.90
        # }

        # Fase 3: Discovery (identificar oportunidades)
        opportunities = discover_opportunities(insights)
        # [
        #   {type: "new_scenario", scenario: "sleep_quality"},
        #   {type: "second_camera", reason: "10% blind spots"}
        # ]

        # Fase 4: Propuesta consultiva
        for opp in opportunities:
            proposal = generate_proposal(opp)
            send_to_client(proposal)

            # Esperar decisión (puede tardar semanas)
            decision = await wait_for_client_decision(timeout=30days)

            if decision.approved:
                implement_upgrade(room_id, opp)
```

**Discovery ejemplo:**
```
Insights (mes 2):
├─ Microdespertares: 3.2/noche (alto)
└─> Opp: Agregar SleepQualityExpert (+€150/mes)

Insights (mes 6):
├─ Cobertura: 90% (10% ángulos muertos)
└─> Opp: Segunda cámara lateral (+€300/mes)
```

---

### **3. PolicyOptimizationWorkflow (A/B Testing)**

```python
@workflow.defn
class PolicyOptimizationWorkflow:
    """Mejora políticas con A/B testing."""

    async def run(self, room_id: str):
        # 1. Analizar últimas 100 evaluaciones
        evals = get_recent_evaluations(limit=100)

        # 2. Calcular métricas
        metrics = {
            "false_positive_rate": 0.18,  # Malo (>15%)
            "false_negative_rate": 0.05,
            "avg_time_to_outcome": "3min"
        }

        # 3. Si métricas malas, optimizar
        if metrics["false_positive_rate"] > 0.15:
            new_policy = optimize_policy(
                target="reduce_false_positives"
            )

            # 4. A/B test (10% de rooms, 7 días)
            await ABTestWorkflow.start(
                new_policy=new_policy,
                rollout_pct=0.10,
                duration_days=7
            )
```

---

### **4. ComplianceWorkflow (Auditoría Regulatoria)**

```python
@workflow.defn
class ComplianceWorkflow:
    """Compliance regulatorio (largo plazo)."""

    async def run(self, facility_id: str, period: str):
        # 1. Recolectar eventos (desde EventStore)
        events = query_eventstore(facility_id, period)

        # 2. Generar reporte
        report = generate_compliance_report(events)

        # 3. Validar contra regulaciones
        validation = validate_regulations(report)

        # 4. Si falla, workflow de remediación
        if not validation.passed:
            RemediationWorkflow.start(validation.issues)

        # 5. Archivar (legal: 7 años)
        archive_report(report, retention_years=7)
```

---

## 💼 Modelo de Negocio: Consultivo B2B (Discovering)

### **Principio: "Menos es Más"**

**NO vendemos:**
- ❌ Sistema completo de 30 cámaras upfront (€150K capex)
- ❌ Dashboard con 500 métricas que nadie usa
- ❌ "Todas las features posibles"

**SÍ vendemos:**
- ✅ POC de 1 cama (€500/mes, mensual)
- ✅ Discovery progresivo (medimos, aprendemos, proponemos)
- ✅ Upgrades incrementales (software first, hardware solo si necesario)
- ✅ KPIs claros (reducir caídas 50%, no "monitoreo genérico")

---

### **Proceso de Venta (Spin B2B)**

#### **Mes 1: POC (Probar Valor)**

```
Sales:
├─ "No le vendemos 30 cámaras hoy"
├─ "Empecemos con 1 cama, 1 residente de alto riesgo"
├─ "30 días para probar, €500/mes"
└─ "Si no ve reducción de caídas, cancela"

Cliente:
├─ "Ok, bajo riesgo" (vs €150K upfront)
└─ "Lo pruebo"

Implementación:
├─ Día 1: Instalar Care Cell (4 horas)
├─ Día 2-7: Calibración y baseline
└─ Día 8-30: Monitoreo activo

KPI:
└─ "Reducir caídas nocturnas 50%"
```

---

#### **Mes 2-3: Measuring & Discovering**

```
Temporal (automático):
├─ Analiza 30 días de datos
└─> Descubre:
    ├─ "José: 0 caídas" (vs 2/mes antes) ✅
    ├─ "Pero: 3.2 microdespertares/noche" (patrón)
    ├─ "85% salidas entre 02:00-04:00"
    └─ "12% falsos positivos"

Sales (mes 2):
├─ "Datos reales de José (no teoría)"
├─> "Redujo caídas 100% ✅"
├─> "Detectamos 3.2 microdespertares/noche"
└─> "¿Quiere investigar calidad de sueño?"

Cliente:
└─> "Sí, eso explica por qué José está cansado"
```

---

#### **Mes 4: Upgrade Incremental (Software)**

```
Sales:
├─ "Propuesta: Agregar SleepQualityExpert"
├─> "Qué incluye:"
│   ├─ Métricas: ISD, deep/light/REM breakdown
│   └─ Alertas: patrones anormales
├─> "KPI: Mejorar ISD +20% en 60 días"
├─> "Costo: +€150/mes (€650 total)"
└─> "Sin hardware nuevo, activación en 1 día"

Cliente:
├─> "€150/mes por entender sueño de José"
└─> "Aprobado"

Temporal:
└─> Activa SleepQualityExpert remotamente (software)
```

---

#### **Mes 7: Hardware Upgrade (Solo si Necesario)**

```
Temporal (automático):
├─> Analiza cobertura de cámara
├─> Simula: "Con cámara lateral → 99% cobertura"
└─> "10% de exits no detectados (ángulo muerto)"

Sales:
├─ "Datos: 10% eventos perdidos"
├─> "Simulación: cámara lateral → 99%"
├─> "Costo: +€300/mes (incluye cámara + instalación)"
└─> "ROI: Prevenir 1 caída = €5,000"

Cliente:
└─> "€300/mes vs €5,000 caída, obvio"
```

---

### **Revenue Growth Path (1 Cliente)**

```
Mes 1-2: POC (1 cama)
  Revenue: €500/mes
  MRR: €500

Mes 3-6: +SleepQuality Scenario
  Revenue: €650/mes (+€150)
  MRR: €650

Mes 7-12: +Segunda Cámara
  Revenue: €950/mes (+€300)
  MRR: €950

Mes 13+: 4 Camas Total (expansión)
  Revenue: €3,800/mes (4x €950)
  MRR: €3,800

Growth: €500 → €3,800 (7.6x en 13 meses)
```

---

### **Unit Economics**

```
CAC (Customer Acquisition Cost): €2,000
  ├─ Sales cycle: 2 meses
  ├─ Demo + POC setup
  └─ Onboarding

LTV (Lifetime Value): €60,000
  ├─ Retention: 5 años (95% anual)
  ├─ Average MRR: €1,000 (post-expansión)
  └─ Total: €1,000 x 12 x 5 = €60,000

LTV/CAC: 30x ✅

Churn anual: 5% (muy bajo, ven valor constante)
```

---

## 📊 Métricas de Éxito

### **Métricas de Cliente (KPIs Clínicos)**

```
Mes 1 (baseline):
├─ Caídas nocturnas: 2/mes
├─ Falsas alarmas: N/A (no había sistema)
└─ Satisfacción enfermeros: N/A

Mes 3 (post-POC):
├─ Caídas nocturnas: 0/mes (-100%) ✅
├─ Falsas alarmas: 12% (aceptable <15%)
└─ Satisfacción enfermeros: 4.2/5

Mes 12 (optimizado):
├─ Caídas nocturnas: 0/mes
├─ Falsas alarmas: 5% (A/B testing mejoró)
├─ Calidad sueño (ISD): +18%
└─ Satisfacción enfermeros: 4.6/5
```

---

### **Métricas de Negocio**

```
Año 1:
  Clientes activos: 50
  MRR: €40,000 (avg €800/cliente)
  ARR: €480,000
  Churn anual: 8%

Año 3:
  Clientes activos: 200
  MRR: €200,000 (avg €1,000/cliente)
  ARR: €2.4M
  Churn: 5%
```

---

## 🗺️ Roadmap de Evolución

### **v1.0 - Care Cell KISS (Edge Only) - AHORA**

```yaml
deployment: Single Care Cell (i7 NUC)
scope: 8 habitaciones max
tech:
  - MQTT (mosquitto local)
  - Python services
  - Room Orchestrators (8 instances)
  - Orion (8 instances)
  - Expert Graph Service
  - Scene Experts

cloud: None (edge-only)
offline_capable: true
latency: <100ms
```

**Objetivo:** Validar arquitectura edge, POC con 1-5 clientes.

---

### **v1.5 - Care Cell + Temporal Integration (Hybrid)**

```yaml
deployment: 10 Care Cells (10 residencias)
cloud: Temporal Cloud (trial)
event_store: EventStore (self-hosted)

care_cell:
  - MQTT (local coreografía)
  - Emite eventos críticos a cloud

temporal:
  - SupervisorWorkflow (evalúa decisiones)
  - DiscoveryWorkflow (detecta oportunidades)
  - ComplianceWorkflow (reportes)

integration:
  - Care Cell → EventBus bridge → EventStore
  - Temporal lee EventStore
  - Temporal envía políticas a Care Cells
```

**Objetivo:** Probar supervisión cloud, discovery workflow, compliance.

---

### **v2.0 - Full Temporal + EventStore (Production)**

```yaml
deployment: 100+ residencias (1000+ Care Cells)
cloud: Temporal Cloud (production)
event_store: EventStore cluster

workflows:
  - SupervisorWorkflow (evaluación continua)
  - DiscoveryWorkflow (consultivo B2B)
  - PolicyOptimizationWorkflow (A/B testing)
  - ComplianceWorkflow (auditoría)
  - FacilityWorkflow (gestión multi-facility)

features:
  - Gemelo Digital completo
  - A/B testing de políticas
  - ML training (políticas optimizadas)
  - Multi-camera (v2.0 scene calibration)
```

**Objetivo:** Escalar a empresa, governance completo, compliance regulatorio.

---

## 🎯 Por qué Esta Arquitectura es Brillante

### **1. Separation of Concerns Quirúrgica**

```
Orion: "Veo persona en (x,y) con confianza 0.92"
Expert: "Eso significa edge_of_bed.intent"
Room Orch: "Activa HQ por 5s"
Temporal: "Esa decisión fue correcta (outcome: exit confirmed)"
```

**Resultado:**
- ✅ Equipos paralelos (ML, clínica, DevOps)
- ✅ Zero acoplamiento
- ✅ Cambiar modelo AI no rompe reglas clínicas

---

### **2. Event-Driven Real (No "Event-Driven de Mentira")**

```
Orion → MQTT → Fire & Forget
Experts → MQTT → Fire & Forget
Room Orch → MQTT commands → Fire & Forget
```

**vs Microservicios REST:**
- ❌ REST: Orion llama Expert vía HTTP → timeout → retry hell
- ✅ MQTT: Pub/sub puro. Si Expert cae, Orion sigue vivo.

**Resultado:**
- ✅ Fault isolation real
- ✅ Residencia NO se cae si un expert crashea

---

### **3. Mallado de Expertos (Explicabilidad)**

```
SleepExpert → produce: sleep.state
  └─> EdgeExpert (consume sleep.state)
      └─> ExitExpert (consume edge.confirmed)
```

**vs ML End-to-End:**
- ❌ Red neuronal negra: "riesgo 0.87" (no explica)
- ✅ Expertos: "ExitExpert activó porque EdgeExpert confirmó y SleepExpert reportó awake"

**Resultado:**
- ✅ Trazabilidad médica
- ✅ Compliance
- ✅ Confianza clínica (médicos entienden)

---

### **4. Autonomous Operator + Supervisor Evaluator**

```
Edge (Room Orch):
├─ Opera AUTÓNOMAMENTE (<100ms)
├─ NO espera aprobación del cloud
└─ Reporta decisiones (async)

Cloud (Temporal):
├─ Observa decisiones (post-facto)
├─ Evalúa outcome
├─ Aprende y mejora
└─ Envía nuevas políticas (async)
```

**Resultado:**
- ✅ Safety (decisiones críticas en <100ms)
- ✅ Reliability (edge funciona offline)
- ✅ Continuous improvement (cloud aprende)

---

### **5. Consultivo Incremental (Business Model)**

```
No vendemos: Sistema completo €150K
Vendemos: POC €500/mes → Discovery → Upsell incremental

Cliente paga por valor entregado, no por "features posibles"
```

**Resultado:**
- ✅ Low barrier to entry (€500 vs €150K)
- ✅ High retention (95%, ven valor constante)
- ✅ LTV/CAC: 30x (unit economics brutales)

---

## 🏆 Veredicto Final

**Esta NO es solo una arquitectura técnica. Es una ESTRATEGIA DE NEGOCIO.**

### **Tech Stack:**
- ✅ Edge (MQTT, Python, Room Orch) → Autonomous, offline-first
- ✅ Cloud (Temporal, EventStore) → Supervisor, compliance, discovery
- ✅ Coreografía (edge) + Orquestación (cloud) → Best of both worlds

### **Business Model:**
- ✅ Consultivo B2B (discovering, no feature dump)
- ✅ Mensual (€500 POC → €3,800 expansión)
- ✅ Discovery-driven (Temporal detecta oportunidades)
- ✅ LTV/CAC: 30x (unit economics)

### **Moat Defendible:**
- ✅ Gemelo Digital (aprende de cada cliente)
- ✅ A/B testing de políticas (mejora continua)
- ✅ Event Sourcing (compliance regulatorio)
- ✅ Explicabilidad (médicos confían)

---

## 📚 Documentación Relacionada

### **Arquitectura:**
- [README_ARQUITECTURA.md](README_ARQUITECTURA.md) - Visión general
- [ARQUITECTURA_SEPARACION_DE_RESPONSABILIDADES.md](ARQUITECTURA_SEPARACION_DE_RESPONSABILIDADES.md)
- [INDICE_ARQUITECTURA.md](INDICE_ARQUITECTURA.md) - Índice maestro

### **Componentes:**
- **Orion:** [ORION_README.md](ORION_README.md) | [ORION_SERVICE_DESIGN.md](ORION_SERVICE_DESIGN%20(v1).md)
- **Room Orchestrator:** [ROOM_ORCHESTRATOR_README.md](ROOM_ORCHESTRATOR_README.md) | [ROOM_ORCHESTRATOR_DESIGN.md](ROOM_ORCHESTRATOR_DESIGN.md)
- **Scene Experts:** [SCENE_EXPERTS_MESH.md](SCENE_EXPERTS_MESH.md) | [SCENE_EXPERTS_TEMPORAL_CONTEXT.md](SCENE_EXPERTS_TEMPORAL_CONTEXT.md)
- **Expert Graph Service:** [EXPERT_GRAPH_SERVICE_DESIGN.md](EXPERT_GRAPH_SERVICE_DESIGN.md)

### **Temporal:**
- [docs/temporal/](./temporal/) - Papers y artículos sobre Temporal.io

---

## 💡 Próximos Pasos

### **Inmediatos (v1.0 KISS):**
1. Implementar Room Orchestrator (Python)
2. Implementar Orion v1.0 (ROIs flat, 3 modelos)
3. Implementar Scene Experts (Sleep, Edge, Exit)
4. Implementar Expert Graph Service (scenarios básicos)
5. POC con 1 cliente real (1 cama)

### **Corto Plazo (v1.5 Hybrid):**
1. Integrar Temporal Cloud (trial)
2. Implementar SupervisorWorkflow
3. Implementar DiscoveryWorkflow
4. EventStore setup (self-hosted)
5. Gemelo Digital básico

### **Largo Plazo (v2.0 Production):**
1. PolicyOptimizationWorkflow (A/B testing)
2. ComplianceWorkflow (auditoría)
3. ML training (políticas optimizadas)
4. Multi-camera support (scene calibration)
5. Escalar a 100+ residencias

---

**"Orion ve. Expertos entienden. Room Orch opera. Temporal supervisa, aprende, y mejora."**

**"No vendemos features. Descubrimos valor. Incremental. Consultivo. Mensual."**

---

*Documento creado: 2025-10-03*
*Última actualización: 2025-10-03*
*Versión: 1.0 - Charla de Terraza con Café ☕*
