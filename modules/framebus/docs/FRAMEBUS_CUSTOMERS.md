# FrameBus Customers: Who Uses It & Why Priority Matters

**Document Type:** Living Business Context  
**Last Updated:** 2025-11-05  
**Owner:** Visiona Architecture Team  

---

## Executive Summary

FrameBus is **not** a generic pub/sub library. It's a specialized component designed for **specific customers**: **Orion Workers** (AI inference workers within the Orion bounded context).

Understanding this customer is essential for making informed technical decisions. This document answers:
- **Who** subscribes to FrameBus?
- **What** are their SLA requirements?
- **How** does the business model influence architecture?

---

## The Customer: Orion Workers

### What are "Workers"?

**Orion Workers** are **Python-based AI inference processes** that consume video frames and emit raw visual facts. Each worker focuses on a **specific AI model** (YOLO, Pose, Flow, VLM).

**Core Philosophy:** "Orion sees, doesn't interpret."

- **Workers** process frames → Emit **raw inferences** (person at X,Y, pose keypoints, optical flow)
- **Sala Experts** (downstream, outside Orion) → Consume inferences via MQTT → Diagnose clinical events

**Key Distinction**: Workers are **internal to Orion**. They emit facts to MQTT, where Sala Experts consume them.

### Current Worker Types (as of v1.5)

| Worker ID                  | AI Model / Task                | Criticality  | SLA Requirement          | Downstream Consumer (Sala) |
|----------------------------|--------------------------------|--------------|--------------------------|----------------------------|
| **PersonDetectorWorker**   | YOLO person detection (bed ROI) | **Critical** | 0% frame drops tolerated | EdgeExpert, ExitExpert     |
| **PoseWorker**             | Pose estimation (keypoints)     | **High**     | <10% frame drops         | EdgeExpert, SleepExpert    |
| **FlowWorker**             | Optical flow (head ROI)         | **Normal**   | <50% frame drops         | SleepExpert (micro-awake)  |
| **VLMExperimentalWorker**  | Vision-Language Model (LLaVA)   | **BestEffort** | 90%+ drops acceptable   | Research pipeline          |

### Why Different Criticalities?

**PersonDetectorWorker (Critical)** → **Foundation for fall detection in Sala**
- EdgeExpert (Sala) **depends** on person detection inferences to detect fall risk
- If PersonDetectorWorker drops frames → EdgeExpert has no data → Fall detection fails
- Elderly falls at night → 20% mortality rate within 6 months
- **SLA: Near-zero frame drops** (protects downstream critical experts)

**PoseWorker (High)** → **Important for edge-of-bed analysis**
- EdgeExpert (Sala) uses pose keypoints to calculate torso angle (fall risk indicator)
- SleepExpert (Sala) uses pose to detect micro-awakenings
- Missing some pose frames is tolerable (EdgeExpert has fallback heuristics)
- **SLA: <10% frame drops** (high quality for clinical analysis)

**FlowWorker (Normal)** → **Quality-of-life monitoring**
- SleepExpert (Sala) uses optical flow for micro-awakening detection
- Missing flow data reduces sleep quality insights, but not life-critical
- **SLA: <50% frame drops** (acceptable for trend analysis)

**VLMExperimentalWorker (BestEffort)** → **Research, no production use**
- Used for discovering new patterns (object recognition, scene understanding)
- No Sala expert depends on VLM data (yet)
- **SLA: Best-effort** (runs when resources available)

---

## Business Model: Consultive B2B "Discovering Value"

### How Visiona Sells Orion

**NOT**: "Buy our complete AI monitoring system for $100k upfront"  
**YES**: "Start with 1 bed, prove value, scale incrementally"

### Typical Customer Journey

**Phase 1: POC (Month 1-3) - Single Bed, Critical Workers Only**
```
Hardware: 1x Intel NUC (CPU-limited)
Orion: 1 camera stream @ 1fps
Workers: PersonDetectorWorker (Critical)
Sala Experts (downstream): EdgeExpert + ExitExpert
Cost: ~$200/month
```

**Phase 2: Expansion (Month 4-6) - Multi-Bed, Add Quality Workers**
```
Hardware: Same 1x NUC (shared compute)
Orion: 5 camera streams @ 1fps
Workers: PersonDetector (Critical) + PoseWorker (High) + FlowWorker (Normal)
Sala Experts (downstream): Edge + Exit + Sleep + Caregiver
Cost: ~$800/month
```

**Phase 3: Full Deployment (Month 7+) - Entire Facility**
```
Hardware: 3x NUCs (load balanced)
Orion: 20 camera streams @ 1fps
Workers: All workers active (including VLM experimental)
Sala Experts (downstream): All experts active
Cost: ~$3,000/month
```

### Why This Matters for FrameBus

**The Problem**: As customers grow from Phase 1 → Phase 3, **compute resources become constrained**.

- **Phase 1**: 1 worker, 1 stream → FrameBus never saturates
- **Phase 3**: 4 workers, 20 streams → **CPU bottleneck, frame drops occur**

**Without Priority**: All workers suffer equally → **PersonDetector drops frames → EdgeExpert (Sala) has no data → Fall detection fails**  
**With Priority**: Critical workers protected → **PersonDetector maintains 0% drops → EdgeExpert (Sala) gets reliable data → SLA maintained**

**Key Insight**: Priority in FrameBus protects **the entire chain**:
```
PersonDetectorWorker (Critical, 0% drops)
  → MQTT inference
    → EdgeExpert (Sala, reliable fall detection)
      → Care Staff Alert (lives saved)
```

---

## Scale & Volume Projections

### Current Deployment (Q4 2024)

- **Customers**: 3 pilot sites (residences)
- **Beds Monitored**: 8 total
- **Orion Instances**: 4 NUCs
- **Sala Experts**: ~15 active expert instances

### Target Scale (Q4 2025)

- **Customers**: 50 residences
- **Beds Monitored**: 500-1000
- **Orion Instances**: 200 NUCs
- **Orion Workers**: ~800 active worker instances (4 workers × 200 NUCs)
- **Sala Experts**: ~1,000 active expert instances (downstream consumers)

### Peak Load Scenarios

**Scenario: Night Shift (2-6 AM)**
- **80% of residents asleep** → FlowWorker analyzing micro-awakenings (sends to SleepExpert)
- **5-10% bathroom trips** → PersonDetectorWorker + PoseWorker at peak load (send to EdgeExpert + ExitExpert)
- **1-2 caregiver rondines** → PersonDetectorWorker tracking movement (sends to CaregiverExpert)

**FrameBus Load**:
- 20 streams × 1 fps = 20 frames/sec published
- 4 workers × 20 frames = 80 channel send operations/sec
- **If any worker is slow** → Backpressure risk → Priority protects Critical workers

---

## SLA Requirements (Detailed)

### Critical Workers (PersonDetectorWorker)

**Drop Rate SLA**: < 1% (target 0%)  
**Latency SLA**: < 100ms (frame → inference)  
**Availability SLA**: 99.9% uptime  

**Justification**: PersonDetector is the **foundation** for fall detection. EdgeExpert (Sala) **depends** on person detection inferences. A missed frame = no data for fall risk analysis.

**Downstream Impact**: EdgeExpert, ExitExpert (both critical Sala experts)

### High Priority Workers (PoseWorker)

**Drop Rate SLA**: < 10%  
**Latency SLA**: < 200ms (frame → inference)  
**Availability SLA**: 99% uptime  

**Justification**: Pose keypoints enable accurate fall risk analysis (torso angle, hip position). Some drops tolerable (EdgeExpert has fallback heuristics).

**Downstream Impact**: EdgeExpert, SleepExpert (Sala experts)

### Normal Priority Workers (FlowWorker)

**Drop Rate SLA**: < 50%  
**Latency SLA**: < 500ms (frame → inference)  
**Availability SLA**: 95% uptime  

**Justification**: Optical flow for sleep quality monitoring. Missing data reduces insights, not life-critical.

**Downstream Impact**: SleepExpert (Sala expert)

### Best-Effort Workers (VLMExperimentalWorker)

**Drop Rate SLA**: None (90%+ drops acceptable)  
**Latency SLA**: None  
**Availability SLA**: Best-effort  

**Justification**: Experimental research. No production Sala expert depends on VLM data yet.

**Downstream Impact**: Research pipeline only

---

## Dynamic Expert Activation (Expert Graph Service)

**Key Insight**: Not all experts run all the time.

The **Room Orchestrator** uses an **Expert Graph Service** to dynamically activate/deactivate experts based on:
- **Customer subscription tier** (what they pay for)
- **Current room context** (person in bed? standing? out of room?)
- **Compute resources available** (CPU, memory limits)

**Example**:
```
Room 302: Resident sleeping peacefully

Orion Workers (FrameBus subscribers):
  ✅ PersonDetectorWorker: ACTIVE (Critical) → MQTT → EdgeExpert, ExitExpert
  ✅ FlowWorker:          ACTIVE (Normal)   → MQTT → SleepExpert
  ❌ PoseWorker:          INACTIVE (not needed yet)
  ❌ VLMWorker:           INACTIVE (not subscribed)

Sala Experts (MQTT consumers):
  ✅ SleepExpert:     ACTIVE (monitoring sleep)
  ❌ EdgeExpert:      STANDBY (waiting for edge detection)
  ❌ ExitExpert:      STANDBY (waiting for exit signal)

[30 seconds later: Person moves to edge of bed]

Orion Core Orchestrator receives MQTT command from Room Orchestrator (Sala):
  "Activate PoseWorker with PriorityHigh - EdgeExpert needs pose data"

Orion Workers (FrameBus subscribers):
  ✅ PersonDetectorWorker: ACTIVE (Critical, 0% drops) → MQTT → EdgeExpert
  ✅ PoseWorker:          ACTIVE (High, <10% drops)   → MQTT → EdgeExpert
  ✅ FlowWorker:          ACTIVE (Normal, 30% drops)  → MQTT → SleepExpert
  ❌ VLMWorker:           INACTIVE

Sala Experts (MQTT consumers):
  ✅ EdgeExpert:      ACTIVE (fall risk detected) ← Depends on PersonDetector + Pose inferences
  ✅ SleepExpert:     ACTIVE (sleep disrupted)
  ✅ ExitExpert:      STANDBY (prepare for exit)
```

**FrameBus Impact**: Worker count fluctuates dynamically based on Room Orchestrator (Sala) commands. Priority ensures PersonDetectorWorker (Critical) maintains 0% drops even when PoseWorker + FlowWorker are added.

---

## Why FrameBus Needs Priority

### Problem Statement

**Without Priority-Based Load Shedding:**

When FrameBus saturates (all worker channels full), frames are dropped **indiscriminately**. PersonDetectorWorker (Critical) drops frames at the same rate as VLMExperimentalWorker (BestEffort).

**Business Impact:**
- ❌ PersonDetectorWorker drops frames → EdgeExpert (Sala) has no data → Fall detection fails
- ❌ SLA violations for critical fall detection (life-safety issue)
- ❌ Customer trust degraded (paid for critical service, got best-effort)
- ❌ Scaling limited (can't add experimental workers without impacting critical ones)

### Solution: Priority Subscribers

**With Priority-Based Load Shedding:**

When FrameBus saturates, frames are dropped in order of priority:
1. **VLMExperimentalWorker** drops first (90%+ drop rate, expected)
2. **FlowWorker** drops next (30-50% drop rate, acceptable)
3. **PoseWorker** drops minimally (5-10% drop rate, tolerable)
4. **PersonDetectorWorker** maintains 0% drops (protected, sorted first)

**Downstream Impact (Sala Experts)**:
- ✅ EdgeExpert gets reliable person detection data (0% gaps)
- ✅ EdgeExpert gets good pose data (5-10% gaps tolerable)
- ⚠️ SleepExpert gets degraded flow data (30-50% gaps acceptable)
- ❌ Research pipeline gets minimal VLM data (expected, no SLA)

**Business Impact:**
- ✅ SLA guarantees for critical fall detection maintained (PersonDetector protected)
- ✅ Customer trust preserved (paid for life-safety, got life-safety)
- ✅ Scaling enabled (can add experimental workers without impacting critical chain)

---

## References

### Business Narrative Documents
- [El Viaje de un Fotón](../vault/about_us/El%20Viaje%20de%20un%20Fotón%20-%20De%20la%20Cámara%20al%20Evento%20Inteligente.md) - End-to-end data flow (Orion → MQTT → Sala)
- [Sistema IA Deliberadamente Tonto](../vault/about_us/Nuestro%20sistema%20de%20IA%20más%20potente%20es%20deliberadamente%20tonto%20-%205%20lecciones%20de%20diseño%20que%20aprendimos.md) - Design philosophy (Orion sees, Sala interprets)
- [Orion Ve, Sala Entiende](../vault/about_us/Orion_Ve,_Sala_Entiende__Desgranando_la_Arquitectura_Modular_de.md) - Architecture podcast transcript (Expert Mesh, Room Orchestrator, Expert Graph Service)

### Technical Architecture
- [ARCHITECTURE.md](../ARCHITECTURE.md) - FrameBus technical design
- [C4_MODEL.md](../C4_MODEL.md) - Container/component diagrams
- [ADR-009](./adr/ADR-009-Priority-Subscribers.md) - Priority subscribers decision (references this document)

---

## Changelog

**2025-11-05**: Initial version - Sala Expert Mesh context, SLA requirements, scaling projections
