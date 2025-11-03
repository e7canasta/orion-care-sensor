# 🌟 Orion - Smart AI Sensor for Care Scene

**Real-time video inference system for geriatric patient monitoring**

> **"Orión Ve, No Interpreta"** (Orion Sees, Doesn't Interpret)

Orion is an edge-first AI sensor that captures video streams and produces structured inference outputs for the Care Scene ecosystem.

---

## 🎯 What is Orion?

Orion is **NOT**:
- ❌ A competitor to Frigate NVR (end-user product)
- ❌ A competitor to DeepStream/DL Streamer (monolithic frameworks)
- ❌ An interpretation or decision engine

Orion **IS**:
- ✅ A configurable "smart sensor" for distributed architectures
- ✅ Best-in-class for event-driven AI sensor deployments
- ✅ A building block for larger monitoring systems (Care Scene)
- ✅ Hardware-agnostic (ONNX enables GPU acceleration)

---

## 🏗️ Architecture

### Technology Stack
- **Go 1.x**: Orchestration, streaming pipeline, control plane
- **Python 3.x**: ONNX inference workers (YOLO11 person detection)
- **GStreamer**: Video capture from RTSP streams
- **MQTT**: Event-driven control and data distribution
- **ONNX Runtime**: Multi-model ML inference
- **MsgPack**: High-performance binary serialization (5x faster than JSON+base64)

### Core Design Philosophy
1. **Complexity by Design, Not by Accident** - Attack complexity through architecture
2. **Pragmatic Performance** - Real-time responsiveness > completeness
3. **Non-blocking Channels** - Drop frames to maintain <2s latency
4. **Hybrid Go-Python** - Go for orchestration, Python for ML
5. **MQTT-centric Control** - Hot-reload without service restart
6. **KISS Auto-Recovery** - One restart attempt only

---

## 🚀 Quick Start

### Prerequisites
- Go 1.21+
- Python 3.10+
- GStreamer 1.20+
- MQTT broker (Mosquitto)

### Build & Run
```bash
# Build binary
make build

# Run with default config
make run

# Run with debug logging
./bin/oriond --debug

# Run with custom config
./bin/oriond --config path/to/orion.yaml
```

### Configuration
Primary config: `config/orion.yaml`

```yaml
instance_id: orion-hab-302
room_id: hab_302

camera:
  rtsp_url: rtsp://camera-ip/stream

stream:
  resolution: 720p
  fps: 30

models:
  person_detector:
    model_path: models/yolo11n.onnx
    max_inference_rate_hz: 1.0

mqtt:
  broker: tcp://localhost:1883
```

---

## 📡 MQTT Control Commands

```bash
# Get status
mosquitto_pub -t care/control/orion-hab-302 -m '{"command":"get_status"}'

# Pause/Resume inference
mosquitto_pub -t care/control/orion-hab-302 -m '{"command":"pause"}'
mosquitto_pub -t care/control/orion-hab-302 -m '{"command":"resume"}'

# Change inference rate
mosquitto_pub -t care/control/orion-hab-302 -m '{"command":"set_inference_rate","rate_hz":2.0}'

# Hot-reload model size
mosquitto_pub -t care/control/orion-hab-302 -m '{"command":"set_model_size","size":"m"}'
```

---

## 🗺️ Roadmap

See [docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md](docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md) for detailed roadmap.

### Phases
- ✅ **Fase 1 (v1.0 → v1.5)**: Foundation - Bounded contexts, single-stream, hot-reload
- 🔄 **Fase 2 (v1.5 → v2.0)**: Scale - Multi-stream (4-8 rooms), resource management
- 📅 **Fase 3 (v2.0 → v3.0)**: Intelligence - Cell orchestration, motion pooling

---

## 📚 Documentation

### Architecture
- [C4 Model](docs/DESIGN/C4_MODEL.md) - Complete architectural views
- [Big Picture](docs/DESIGN/Big%20Picture.md) - Orion 1.0 overview
- [Evolution Plan](docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md) - 3-phase roadmap

### Development
- [CLAUDE.md](CLAUDE.md) - AI-assisted development guide
- [Design Manifesto](MANIFESTO_DISENO%20-%20Blues%20Style.md) - Design philosophy

---

## 🎸 Design Philosophy: "Blues Style"

> **"Las buenas prácticas son vocabulario de diseño - las practicas para tenerlas disponibles cuando improvises, no porque la partitura lo diga."**

- ✅ DDD for bounded contexts clarity
- ✅ SOLID where it matters
- ✅ Pragmatism for utilities
- ❌ NO Hexagonal "because best practice"
- ❌ NO DI everywhere "because SOLID"

**Read the scales, improvise with context.**

---

## 🤝 Contributing

This is a B2B consultative product. Contact [Visiona](https://visiona.app) for collaboration.

### Development Workflow
1. Read `CLAUDE.md` + `Big Picture.md` before coding
2. Attack complexity through architecture, not code tricks
3. Validate compilation as primary test
4. Document architectural decisions (ADR style)

### Commit Standards
- Co-authored by: `Gaby de Visiona <noreply@visiona.app>`
- Focus on "why" rather than "what"

---

## 📄 License

Proprietary - Visiona © 2024-2025

---

## 🔗 Related Projects

Part of the **Care Scene** ecosystem:
- **Orion** - Smart sensor (this repo)
- **Scene Experts Mesh** - Event interpretation
- **Room Orchestrator** - Resource coordination
- **Temporal Supervisor** - Continuous learning

---

**Built with pragmatism, designed for scale.** 🚀
