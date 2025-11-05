# Orion Care Sensor - Technical Memory for AI Copilots

**Document Type:** Onboarding Guide for AI Agents & Human Developers  
**Audience:** Future copilots working on Orion ecosystem  
**Purpose:** Understand the big picture BEFORE diving into bounded contexts  
**Last Updated:** 2025-11-05  
**Maintainers:** Visiona Architecture Team  

---

## 📖 **How to Read This Document**

This is **NOT** a code reference. This is a **mental model builder**.

**Read top-down** (System → Container → Component):
1. **System Context** - What is Orion in the world?
2. **Container View** - What are the major pieces?
3. **Component Details** - How do they work internally?
4. **Integration Patterns** - How do they talk to each other?

**For AI Copilots**: Read this FIRST before touching code. It will save you from designing in the wrong bounded context (like I almost did with FrameBus 😅).

---

## 🌍 **C1: System Context - The Big Picture**

### What is Orion?

**Orion** is a **smart sensor** for eldercare monitoring. It's NOT a complete solution - it's ONE PIECE of a larger ecosystem.

**Philosophy**: **"Orion Ve, No Interpreta"** (Orion Sees, Doesn't Interpret)

```
┌─────────────────────────────────────────────────────────────────────┐
│                    Care Ecosystem (Full Solution)                    │
│                                                                       │
│  ┌──────────────┐        ┌──────────────┐        ┌──────────────┐  │
│  │   Camera     │  RTSP  │    Orion     │  MQTT  │     Sala     │  │
│  │  (Hardware)  │───────>│ (Smart Lens) │───────>│ (Interpreter)│  │
│  └──────────────┘        └──────────────┘        └──────────────┘  │
│                                                           │          │
│                                                           ↓          │
│                                               ┌──────────────────┐  │
│                                               │   Care Staff     │  │
│                                               │   (Alerts/UX)    │  │
│                                               └──────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### Orion's Job (What it DOES):

1. **Capture** video frames from RTSP cameras (30 fps stream)
2. **Process** frames with AI models (person detection, pose estimation, optical flow)
3. **Emit** raw visual facts via MQTT (NOT interpretations, just observations)
4. **Accept** configuration commands via MQTT Control Plane

**Orion does NOT**:
- ❌ Decide if a situation is dangerous (that's Sala's job)
- ❌ Send alerts to staff (that's Care UX's job)
- ❌ Store video/data long-term (that's Data Platform's job)

### Sala's Job (What Orion feeds INTO):

**Sala** (Scene Expert Mesh) is the "brain" that interprets Orion's facts:

1. **Consume** Orion's MQTT inference messages
2. **Correlate** multiple facts over time (temporal reasoning)
3. **Diagnose** clinical events (fall risk, sleep disruption, bed exit)
4. **Emit** domain events for care staff (alerts, notifications)

**Analogy**: 
- **Orion** = Radiologist technician (operates X-ray machine, describes what's visible)
- **Sala** = Radiologist doctor (reads X-ray, diagnoses disease)

### Boundary Between Orion & Sala

**The boundary is MQTT**:

```
Orion Output:
{
  "topic": "care/inferences/orion-nuc-001/person_detection",
  "payload": {
    "person_detected": true,
    "bounding_box": {"x": 120, "y": 340, "w": 80, "h": 200},
    "confidence": 0.92,
    "roi_overlap": {"BED_RIGHT_EDGE": 0.65}
  }
}

Sala Input: (same MQTT message)
EdgeExpert logic:
  IF person_detected AND roi_overlap.BED_RIGHT_EDGE > 0.5
     AND pose_angle > 60 degrees (from separate inference)
     AND persistence > 1.2 seconds
  THEN emit domain event: "Edge_of_Bed_Confirmed" (fall risk)
```

**Key Insight**: Orion emits **facts** ("person at coordinates X,Y"), Sala emits **interpretations** ("fall risk detected").

---

## 🏗️ **C2: Container View - Orion Architecture**

### High-Level Containers

```
┌─────────────────────────────────────────────────────────────────────┐
│                            Orion System                              │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                    Stream-Capture Module                      │  │
│  │  (GStreamer pipeline: RTSP → decode → scale → JPEG)          │  │
│  └────────────────────┬─────────────────────────────────────────┘  │
│                       │ Frames (30 fps)                             │
│                       ↓                                              │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                      FrameBus Module                          │  │
│  │  (Non-blocking fan-out with priority-based load shedding)    │  │
│  └─┬────────────┬────────────┬────────────┬─────────────────────┘  │
│    │            │            │            │                         │
│    ↓            ↓            ↓            ↓                         │
│  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐                      │
│  │Person  │ │ Pose   │ │ Flow   │ │  VLM   │  ← Workers (Python)  │
│  │Detector│ │ Worker │ │ Worker │ │ Exp.   │                       │
│  │(YOLO)  │ │(Pose)  │ │(OptFlo)│ │(LLaVA) │                       │
│  └────┬───┘ └────┬───┘ └────┬───┘ └────┬───┘                      │
│       │          │          │          │                            │
│       └──────────┴──────────┴──────────┘                            │
│                   │ Inferences (JSON)                               │
│                   ↓                                                  │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                    MQTT Emitter Module                        │  │
│  │  (Publishes to care/inferences/{instance_id}/{type})         │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                  Control Plane Handler                        │  │
│  │  (Subscribes to care/control/{instance_id})                  │  │
│  │  (Commands: pause, set_rate, activate_worker, set_priority)  │  │
│  └──────────────────────────────────────────────────────────────┘  │
│                                                                       │
│  ┌──────────────────────────────────────────────────────────────┐  │
│  │                  Orion Core Orchestrator                      │  │
│  │  (Manages worker lifecycle, subscribes to FrameBus)          │  │
│  └──────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

### Technology Stack

| Component       | Technology           | Purpose                                    |
|-----------------|----------------------|--------------------------------------------|
| Stream Capture  | GStreamer (C/Go)     | RTSP decode, scale, format conversion      |
| FrameBus        | Pure Go              | Non-blocking fan-out distribution          |
| Workers         | Python + ONNX        | AI inference (YOLO, Pose, Flow models)     |
| IPC (Go↔Python) | MsgPack over stdin   | 5x faster than JSON+base64                 |
| MQTT Client     | Paho Go              | Data plane (inferences) + Control plane    |
| Orchestrator    | Pure Go              | Worker lifecycle, FrameBus subscription    |

### Data Flow (Frame Journey)

```
1. Camera (RTSP) → Stream-Capture
   Input:  rtsp://192.168.1.100:554/stream
   Output: JPEG frame (512x512, 30 fps)

2. Stream-Capture → FrameBus
   Input:  Frame{Data: []byte, Seq: 12345, Timestamp: time.Now()}
   Output: Fan-out to N workers (non-blocking, priority-sorted)

3. FrameBus → Workers (via channel + priority)
   Critical:   PersonDetectorWorker  (ch buffer=10, Priority=0)
   High:       PoseWorker            (ch buffer=10, Priority=1)
   Normal:     FlowWorker            (ch buffer=5,  Priority=2)
   BestEffort: VLMExperimentalWorker (ch buffer=5,  Priority=3)

4. Workers → MQTT Emitter
   Input:  Inference{detections: [...], confidence: 0.92}
   Output: MQTT publish to care/inferences/orion-nuc-001/person_detection

5. MQTT → Sala Experts
   (Outside Orion boundary - different system)
```

---

## 🔧 **C3: Component Deep Dive - FrameBus**

### Why FrameBus Exists

**Problem**: One video stream, multiple AI workers at different speeds.

- PersonDetector: ~20ms/frame (fast)
- PoseWorker: ~50ms/frame (medium)
- VLMWorker: ~500ms/frame (slow)

**Without FrameBus**: 
- Stream-Capture would need to know about all workers (tight coupling)
- Slow workers would block fast workers (head-of-line blocking)
- Adding new worker = modify Stream-Capture code

**With FrameBus** (fan-out pattern):
- Stream-Capture publishes frames, doesn't care who consumes
- Each worker reads at its own pace (independent channels)
- Slow workers drop frames (by design - "latency > completeness")

### FrameBus Design Decisions

**Core Philosophy**: "Drop frames, never queue"

```go
// FrameBus distributes frames to workers
for frame := range streamCaptureOutput {
    bus.Publish(frame) // Non-blocking, microseconds latency
}

// Workers receive frames (or don't, if channel full)
for frame := range workerChannel {
    inference := runModel(frame) // Takes 20-500ms
    emitToMQTT(inference)
}
```

**Key Design Points**:

1. **Non-blocking Publish**: `Publish()` NEVER waits for slow subscribers
2. **Drop Policy**: If worker channel full → drop frame (intentional)
3. **Priority-based Load Shedding**: Critical workers get frames first under load
4. **Observable**: Stats tracking (sent/dropped per subscriber)

### Priority Levels (Why They Matter)

**Use Case**: Hardware-constrained deployment (1 NUC, 5 cameras)

Under load, FrameBus protects critical workers:

```go
// Critical: Person detection (base for fall detection in Sala)
bus.SubscribeWithPriority("person-detector-bed-roi", ch, PriorityCritical)
// → 0% drops guaranteed (sorted first, protected)

// High: Pose estimation (important for edge-of-bed analysis)
bus.SubscribeWithPriority("pose-worker-bed-edge", ch, PriorityHigh)
// → <10% drops tolerated

// Normal: Flow analysis (micro-awakening detection)
bus.SubscribeWithPriority("flow-worker-head-roi", ch, PriorityNormal)
// → <50% drops acceptable

// BestEffort: Experimental VLM (research, no SLA)
bus.SubscribeWithPriority("vlm-experimental", ch, PriorityBestEffort)
// → 90%+ drops expected (sacrificed first)
```

**Business Value**:
- Single NUC can run 4+ workers with differentiated SLAs
- Critical features (fall detection) never degraded
- Experimental features (VLM) run "best effort" without extra hardware

### FrameBus vs Other Patterns

| Pattern              | Use Case                          | FrameBus Fit? |
|----------------------|-----------------------------------|---------------|
| Worker Pool          | Load balancing identical workers  | ❌ Different models/speeds |
| Queue (FIFO)         | Guaranteed delivery               | ❌ Real-time > completeness |
| Pub/Sub (MQTT)       | Network distribution              | ❌ In-process, microsecond latency |
| Channel Fan-out      | In-process broadcast              | ✅ Exact fit |

---

## 🔗 **C4: Integration Patterns**

### Pattern 1: Orion Core Orchestrator ↔ FrameBus

**Orion Core Orchestrator** is the internal component that:
- Receives MQTT commands from Control Plane
- Starts/stops Python workers (subprocess management)
- Subscribes workers to FrameBus with appropriate priority

**Example Flow**:

```
1. MQTT Control Plane → Orion:
   Topic: care/control/orion-nuc-001
   Payload: {"command": "activate_worker", "type": "pose", "priority": "high"}

2. Orion Core Orchestrator:
   - Spawn Python subprocess: models/pose_worker.py
   - Create channel: poseCh := make(chan Frame, 10)
   - Subscribe to FrameBus: bus.SubscribeWithPriority("pose-worker-302", poseCh, PriorityHigh)

3. Worker receives frames:
   - Read from poseCh
   - Run inference (ONNX model)
   - Emit to MQTT via stdout (MsgPack protocol)

4. MQTT Emitter → Sala:
   Topic: care/inferences/orion-nuc-001/pose_keypoints
   Payload: {"keypoints": [...], "confidence": 0.88}
```

### Pattern 2: Go ↔ Python Worker (MsgPack IPC)

**Why MsgPack over stdin/stdout?**
- 5x faster than JSON + base64 for binary data (frame bytes)
- Process isolation (Python crash doesn't kill Orion)
- Simple protocol (4-byte length prefix + MsgPack payload)

**Go → Python (frame data)**:
```
[4-byte length][MsgPack payload]
{
  "frame_data": <raw JPEG bytes>,  // NO base64!
  "width": 512,
  "height": 512,
  "meta": {
    "instance_id": "orion-nuc-001",
    "room_id": "302",
    "seq": 12345,
    "roi": {"x": 0, "y": 0, "w": 512, "h": 512}
  }
}
```

**Python → Go (inference result)**:
```
[4-byte length][MsgPack payload]
{
  "data": {
    "detections": [{"x": 120, "y": 340, "w": 80, "h": 200, "conf": 0.92}],
    "person_count": 1
  },
  "timing": {
    "total_ms": 23.4,
    "inference_ms": 18.2
  }
}
```

### Pattern 3: Two Orchestrators, Two Contexts

**CRITICAL DISTINCTION** (common source of confusion):

```
┌─────────────────────────────────────────────────────────────┐
│                    Orion (Bounded Context)                   │
│                                                               │
│  ┌────────────────────────────────────────────────────┐     │
│  │       Orion Core Orchestrator (Internal)           │     │
│  │  - Manages worker lifecycle (start/stop)           │     │
│  │  - Subscribes workers to FrameBus with priority    │     │
│  │  - Receives commands from MQTT Control Plane       │     │
│  │  - Responsibility: "What lenses to put on?"        │     │
│  └────────────────────────────────────────────────────┘     │
│                                                               │
└───────────────────────────┬───────────────────────────────────┘
                            │ MQTT (facts)
                            ↓
┌─────────────────────────────────────────────────────────────┐
│                    Sala (Bounded Context)                    │
│                                                               │
│  ┌────────────────────────────────────────────────────┐     │
│  │         Room Orchestrator (External to Orion)      │     │
│  │  - Manages expert lifecycle (activate/deactivate)  │     │
│  │  - Uses Expert Graph Service (dependencies)        │     │
│  │  - Sends commands TO Orion via MQTT Control Plane  │     │
│  │  - Responsibility: "What analysis do I need?"      │     │
│  └────────────────────────────────────────────────────┘     │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

**Example Interaction**:

```
1. Sala Room Orchestrator: 
   "Room 302 - Person detected near bed edge"
   
2. Expert Graph Service: 
   "EdgeExpert needs: person_detection + pose_keypoints inferences"
   
3. Room Orchestrator → MQTT Control Plane → Orion:
   {"command": "activate_worker", "type": "pose", "room": "302", "priority": "high"}
   
4. Orion Core Orchestrator:
   - Start pose worker
   - Subscribe to FrameBus with PriorityHigh
   
5. Pose Worker → MQTT → EdgeExpert (Sala):
   EdgeExpert logic: "Pose angle = 62°, hip near edge → Fall risk confirmed"
   
6. EdgeExpert → Care Staff Alert:
   "Room 302: Resident at edge of bed, fall risk detected"
```

**Key Insight**: 
- **Orion Core Orchestrator** = Internal (manages lenses)
- **Room Orchestrator (Sala)** = External (orchestrates analysis)
- They are **NOT the same thing** (different bounded contexts)

---

## 🎯 **Common Pitfalls for New Copilots**

### Pitfall 1: "FrameBus distributes to Sala Experts"

**❌ WRONG**:
```
FrameBus → EdgeExpert (Sala)
         → SleepExpert (Sala)
```

**✅ CORRECT**:
```
FrameBus → PersonDetectorWorker (Orion) → MQTT → EdgeExpert (Sala)
         → PoseWorker (Orion)           → MQTT → SleepExpert (Sala)
```

**Why it matters**: FrameBus is **internal to Orion**. It never crosses the MQTT boundary.

---

### Pitfall 2: "Workers and Experts are the same thing"

**❌ WRONG**: Workers = Experts

**✅ CORRECT**:
- **Workers (Orion)**: Process frames → Emit facts ("person at X,Y")
- **Experts (Sala)**: Consume facts → Emit interpretations ("fall risk")

**Analogy**:
- **Worker** = X-ray technician (operates machine, describes image)
- **Expert** = Radiologist doctor (reads image, diagnoses disease)

---

### Pitfall 3: "Orion makes decisions about fall risk"

**❌ WRONG**: Orion detects fall risk

**✅ CORRECT**: 
- Orion emits: "Person bounding box overlaps BED_EDGE ROI by 65%"
- Sala decides: "65% overlap + pose angle 62° = fall risk"

**Why it matters**: Orion is a **sensor**, not a **decision engine**. All interpretation logic lives in Sala.

---

### Pitfall 4: "There's only one orchestrator"

**❌ WRONG**: "The orchestrator manages everything"

**✅ CORRECT**: Two orchestrators, two contexts:
- **Orion Core Orchestrator**: Internal (worker lifecycle, FrameBus subscription)
- **Room Orchestrator (Sala)**: External (expert activation, analysis orchestration)

**Why it matters**: Confusing them leads to designing features in the wrong bounded context.

---

## 📚 **Key Documents by Bounded Context**

### Orion (Stream Processing)

| Document                          | Purpose                                      |
|-----------------------------------|----------------------------------------------|
| `modules/stream-capture/CLAUDE.md` | Stream acquisition, GStreamer pipeline      |
| `modules/framebus/CLAUDE.md`       | Frame distribution, priority subscribers     |
| `modules/framebus/FRAMEBUS_CUSTOMERS.md` | Who uses FrameBus (Workers), SLAs    |
| `modules/worker-lifecycle/CLAUDE.md` | Worker spawning, subprocess management     |
| `VAULT/arquitecture/wiki/2-core-service-oriond.md` | Orion service overview    |

### Sala (Scene Interpretation)

| Document                          | Purpose                                      |
|-----------------------------------|----------------------------------------------|
| `vault/about_us/Orion_Ve,_Sala_Entiende.md` | Complete architecture podcast         |
| `vault/about_us/El Viaje de un Fotón.md` | End-to-end data flow                   |
| `vault/about_us/Sistema IA Deliberadamente Tonto.md` | Design philosophy          |

### Cross-Cutting

| Document                          | Purpose                                      |
|-----------------------------------|----------------------------------------------|
| `VAULT/D002 About Orion.md`       | "Orion Ve, No Interpreta" philosophy         |
| `VAULT/D003 The Big Picture.md`   | System context overview                      |
| `VAULT/arquitecture/ARCHITECTURE.md` | 4+1 architectural views                   |

---

## 🎸 **Design Philosophy (The Blues Way)**

### "Complejidad por Diseño, No por Accidente"

Attack complexity through **architecture** (separation of concerns), not through complicated code.

**Example**: 
- ❌ Monolith: One service does capture + inference + interpretation + alerting
- ✅ Orion 2.0: Bounded contexts (Orion sees, Sala interprets, Care UX alerts)

### "Pragmatismo > Purismo"

Make decisions based on **context**, not dogma.

**Example (FrameBus)**:
- Could use pre-sorted cache for subscribers → Faster Publish()
- BUT: Premature optimization (10 subscribers, sorting takes 200ns)
- Decision: Simple implementation first, optimize if benchmarks show bottleneck

### "Drop Frames, Never Queue"

For real-time video: **Processing recent frame > Processing stale queue**

**Example**:
- 30 fps stream, 1 Hz inference → 97% drop rate is **expected, not a bug**
- Better to process frame #30 (recent) than queue #1-#29 (stale)

### "Orión Ve, No Interpreta"

**Strict separation**: Orion emits facts, Sala interprets meanings.

**Example**:
- Orion: "Keypoint hip at (x=340, y=450), confidence 0.88"
- Sala: "Hip 11cm from bed edge + torso angle 62° = Edge of bed (fall risk)"

---

## 🚀 **Onboarding Workflow for New Copilots**

### Step 1: Read Context (30 mins)

1. Read this document (ORION_SYSTEM_CONTEXT.md) - **START HERE**
2. Read `vault/about_us/Orion_Ve,_Sala_Entiende.md` - Architecture podcast
3. Read `VAULT/D002 About Orion.md` - Design philosophy

**Goal**: Build mental model of **System Context** (C1 level)

---

### Step 2: Understand Boundaries (15 mins)

**Key Questions to Answer**:
- Where does Orion end and Sala begin? → **MQTT boundary**
- What does Orion emit? → **Raw visual facts (inferences)**
- What does Sala emit? → **Clinical interpretations (domain events)**
- Can Orion make decisions? → **NO, only observations**

**Goal**: Understand **Container View** (C2 level)

---

### Step 3: Pick a Bounded Context (Before Coding)

**Before making ANY code changes, identify**:
- Which bounded context am I working in? (Orion? Sala? Care UX?)
- Does my feature belong here? (Check anti-responsibilities)
- What are the boundaries I cannot cross?

**Goal**: Avoid designing in wrong context (like FrameBus → Sala Experts)

---

### Step 4: Read Module-Specific Docs

**Once you know your bounded context**, read:
- `modules/{module}/CLAUDE.md` - Detailed API, anti-patterns
- `modules/{module}/ARCHITECTURE.md` - Technical deep dive (C3 level)
- `modules/{module}/docs/adr/` - Architecture decisions

**Goal**: Understand **Component Details** (C3/C4 level)

---

### Step 5: Code with Context

**While coding, constantly ask**:
- Am I respecting boundaries? (Orion doesn't interpret, Sala doesn't see)
- Am I adding dependencies that cross contexts? (RED FLAG)
- Would this work if Orion and Sala were separate services? (GOOD TEST)

**Goal**: Write code that respects architectural boundaries

---

## 🔮 **Future Evolution**

### Orion 2.0 Roadmap

**Phase 1 (v1.5)**: Foundation
- ✅ Multi-module monorepo
- ✅ FrameBus with priority subscribers
- ✅ Stream-Capture module
- 🚧 Worker-Lifecycle module

**Phase 2 (v2.0)**: Multi-Stream
- 🎯 Support 5+ simultaneous camera streams
- 🎯 Per-stream worker allocation
- 🎯 Shared worker pools (load balancing)

**Phase 3 (v3.0)**: Cell Orchestration
- 🎯 Distributed Orion instances (multi-NUC)
- 🎯 Kubernetes-native deployment
- 🎯 Horizontal scaling with stateless design

### Potential New Modules

**Within Orion Bounded Context**:
- `modules/model-manager` - ONNX model versioning, hot-reload
- `modules/roi-processor` - Dynamic ROI calculation (move from Go to Python)
- `modules/telemetry` - OpenTelemetry integration

**Outside Orion** (different services):
- **Sala** - Expert Mesh, Room Orchestrator, Expert Graph Service
- **Care UX** - Staff alerts, dashboard, compliance reporting
- **Data Platform** - Storage, analytics, ML training pipeline

---

## 🎯 **Quick Reference Cheat Sheet**

### Is this Orion or Sala?

| Feature                          | Orion | Sala |
|----------------------------------|-------|------|
| Process video frames             | ✅    | ❌   |
| Run YOLO/Pose/Flow models        | ✅    | ❌   |
| Emit "person detected at X,Y"    | ✅    | ❌   |
| Emit "fall risk detected"        | ❌    | ✅   |
| Correlate facts over time        | ❌    | ✅   |
| Activate/deactivate AI models    | ✅    | ❌   |
| Activate/deactivate experts      | ❌    | ✅   |
| Send alerts to staff             | ❌    | ❌   |

**Remember**: 
- **Orion** = Sees (captures, processes, observes)
- **Sala** = Interprets (analyzes, correlates, diagnoses)
- **Care UX** = Acts (alerts, displays, records)

### FrameBus Quick Facts

- **What**: Non-blocking fan-out for frame distribution
- **Where**: Internal to Orion (between Stream-Capture and Workers)
- **Why**: Decouple publisher (stream) from subscribers (workers)
- **Priority**: Protect critical workers under load (fall detection SLA)
- **Clients**: Orion Workers (NOT Sala Experts)

### MQTT Topics Structure

```
# Data Plane (Orion → Sala)
care/inferences/{instance_id}/person_detection
care/inferences/{instance_id}/pose_keypoints
care/inferences/{instance_id}/flow_roi

# Control Plane (Sala → Orion)
care/control/{instance_id}
  Commands: activate_worker, set_priority, update_roi, pause, resume
```

---

## 📞 **When in Doubt**

**Ask these questions**:

1. **"Does this cross the MQTT boundary?"**
   - If YES → You're mixing Orion and Sala concerns

2. **"Am I making Orion interpret facts?"**
   - If YES → Move logic to Sala

3. **"Am I making Sala process video?"**
   - If YES → Move logic to Orion

4. **"Would this work if Orion and Sala were on different servers?"**
   - If NO → You have tight coupling (BAD)

5. **"Can I explain this feature without mentioning the other system?"**
   - If NO → You're violating separation of concerns

---

## 🎸 **Closing Notes**

This document is **alive** - update it as the architecture evolves.

**For AI Copilots**: You'll make mistakes (I did with FrameBus → Sala Experts). That's OK. The important thing is to **understand the boundaries** and **ask questions** when confused.

**For Human Developers**: This is the mental model we want ALL team members to have. Use it in onboarding, architecture reviews, and design discussions.

**Remember**: 

> **"Orión Ve, No Interpreta"**  
> **"Complejidad por Diseño, No por Accidente"**  
> **"Pragmatismo > Purismo"**

---

**Document Version:** 1.0  
**Last Updated:** 2025-11-05  
**Authors:** Ernesto Canales, Gaby de Visiona (AI Companion)  

🎸 **Blues Style**: We improvise with knowledge of the rules, we don't follow the sheet music literally.

---

## Appendix: Glossary

| Term                  | Definition                                                                 |
|-----------------------|---------------------------------------------------------------------------|
| **Orion**             | Smart sensor service (sees, processes, emits facts)                      |
| **Sala**              | Scene interpretation service (analyzes, correlates, diagnoses)            |
| **Worker**            | Orion component that runs AI models (person-detector, pose, flow)        |
| **Expert**            | Sala component that interprets facts (EdgeExpert, SleepExpert)           |
| **FrameBus**          | Internal Orion module for non-blocking frame distribution                |
| **Inference**         | Raw AI model output (bounding boxes, keypoints, optical flow)            |
| **Domain Event**      | Interpreted clinical event (fall risk, bed exit, sleep disruption)       |
| **MQTT Boundary**     | The line between Orion (publisher) and Sala (subscriber)                |
| **Orion Core Orchestrator** | Internal component managing Orion worker lifecycle            |
| **Room Orchestrator** | Sala component managing expert activation based on room context          |
| **Bounded Context**   | DDD pattern - clear boundaries between subsystems                        |
| **ROI**               | Region of Interest (bed, door, window polygon in video frame)            |
