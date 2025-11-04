# C4 Model: Stream-Capture Module

**Orion 2.0 - Bounded Context: Stream Acquisition**

This document describes the architecture of the `stream-capture` module using the [C4 Model](https://c4model.com/) (Context, Container, Component, Code).

**Purpose**: Provide visual architecture reference for future Claude Code sessions and development team.

**Last Updated**: 2025-11-04
**Revision**: 1.0

---

## Table of Contents

1. [C1: System Context](#c1-system-context)
2. [C2: Container Diagram](#c2-container-diagram)
3. [C3: Component Diagram](#c3-component-diagram)
4. [C4: Code Diagram](#c4-code-diagram)
5. [Key Architectural Decisions](#key-architectural-decisions)

---

## C1: System Context

**Scope**: How stream-capture fits within Orion 2.0 ecosystem.

```mermaid
graph TB
    subgraph "External Systems"
        Camera[RTSP Camera, H.264 Stream, 1-30 fps]
        MQTTBroker[MQTT Broker, Control Plane, Mosquitto]
    end

    subgraph "Orion 2.0 System"
        StreamCapture[Stream Capture Module, Bounded Context, RTSP Acquisition]
        WorkerLifecycle[Worker Lifecycle Module, Inference Orchestration, Python Workers]
        ControlPlane[Control Plane Module, Hot-Reload Commands, MQTT Handler]
    end

    subgraph "Infrastructure"
        GStreamer[GStreamer 1.0+, Multimedia Framework, VAAPI Support]
        VAAPI[Intel VAAPI, Hardware Decode, Quick Sync]
    end

    Camera -->|RTSP/TCP, H.264 stream| StreamCapture
    StreamCapture -->|Frame Channel, RGB bytes| WorkerLifecycle
    MQTTBroker -->|Control Commands, set_fps, pause| ControlPlane
    ControlPlane -->|Hot-Reload, SetTargetFPS| StreamCapture
    StreamCapture -.->|Uses| GStreamer
    GStreamer -.->|Hardware Accel| VAAPI

    classDef external fill:#e1f5ff,stroke:#333,stroke-width:2px
    classDef orion fill:#d4edda,stroke:#333,stroke-width:2px
    classDef infra fill:#fff3cd,stroke:#333,stroke-width:2px

    class Camera,MQTTBroker external
    class StreamCapture,WorkerLifecycle,ControlPlane orion
    class GStreamer,VAAPI infra
```
# End of Selection


**Key Interactions**:
- **RTSP Camera → Stream-Capture**: TCP-only H.264 stream (protocols=4)
- **Stream-Capture → Worker Lifecycle**: Non-blocking frame channel (RGB format, 10 buffer)
- **Control Plane → Stream-Capture**: Hot-reload commands (SetTargetFPS, Pause/Resume)
- **Stream-Capture ↔ GStreamer**: Pipeline management via go-gst bindings
- **GStreamer ↔ VAAPI**: Hardware decode acceleration (optional, auto-fallback)

**External Dependencies**:
- RTSP Camera (H.264 baseline/main profile)
- GStreamer 1.0+ (plugins: base, good, bad, vaapi)
- VAAPI drivers (intel-media-va-driver for Quick Sync)

---

## C2: Container Diagram

**Scope**: Technology containers within stream-capture module.

```mermaid
graph TB
    subgraph "Stream-Capture Module Go 1.21+"
        subgraph "Public API"
            StreamProvider[StreamProvider Interface: Start, Stop, Stats, SetTargetFPS, Warmup]
        end

        subgraph "Implementation"
            RTSPStream[RTSPStream Struct: GStreamer Pipeline, Lifecycle Management]
            InternalPackages[Internal Packages: rtsp, warmup]
        end

        subgraph "Dependencies"
            GoGst[go-gst Library: github.com/tinyzimmer/go-gst, CGo Bindings]
            UUID[UUID Library: github.com/google/uuid, TraceID Generation]
        end
    end

    subgraph "GStreamer Runtime C/C++"
        Pipeline[GStreamer Pipeline: rtspsrc → decoder → appsink, VAAPI or Software]
        Plugins[GStreamer Plugins: rtsp, h264, vaapi, base]
    end

    subgraph "System Libraries"
        VAAPI2[VAAPI Driver: libva, i965/iHD, Intel Quick Sync]
        Kernel[Linux Kernel: DRM/KMS, Hardware Access]
    end

    Consumer[Consumer Goroutine: Frame Processing, Inference Workers] -->|frameChan, RGB bytes| StreamProvider
    StreamProvider --> RTSPStream
    RTSPStream --> InternalPackages
    RTSPStream --> GoGst
    RTSPStream --> UUID
    GoGst -->|CGo| Pipeline
    Pipeline --> Plugins
    Plugins -->|Hardware Decode| VAAPI2
    VAAPI2 --> Kernel

    classDef golang fill:#00ADD8,stroke:#333,stroke-width:2px,color:#fff
    classDef gstreamer fill:#f39c12,stroke:#333,stroke-width:2px
    classDef system fill:#95a5a6,stroke:#333,stroke-width:2px,color:#fff
    classDef api fill:#3498db,stroke:#333,stroke-width:3px,color:#fff

    class StreamProvider,RTSPStream,InternalPackages,GoGst,UUID,Consumer golang
    class Pipeline,Plugins gstreamer
    class VAAPI2,Kernel system
    class StreamProvider api
```


**Technology Stack**:
- **Go 1.21+**: Main language (concurrency, orchestration)
- **go-gst (CGo)**: GStreamer bindings (pipeline management)
- **GStreamer 1.0+**: Multimedia framework (RTSP, H.264 decode, format conversion)
- **VAAPI**: Hardware acceleration (Intel Quick Sync, optional)
- **Linux Kernel**: Hardware access (DRM/KMS for GPU decode)

**Key Container Boundaries**:
1. **Go Process** (stream-capture): Lifecycle, concurrency, API
2. **GStreamer Runtime** (C/C++): Pipeline execution, codec processing
3. **System Libraries** (VAAPI/Kernel): Hardware acceleration layer

---

## C3: Component Diagram

**Scope**: Internal components within stream-capture Go module.

```mermaid
graph TB
    subgraph "Public Interface [provider.go]"
        StreamProvider[StreamProvider Interface - 5 methods: Start, Stop, Stats, SetTargetFPS, Warmup]
    end

    subgraph "Core Implementation [rtsp.go]"
        RTSPStream[RTSPStream Struct - Pipeline state, Atomic counters, Reconnection state]
        Start[Start Method - Create pipeline, Launch goroutines, Non-blocking return]
        Stop[Stop Method - Cancel context, Wait goroutines 3s, Double-close protection]
        Stats[Stats Method - Atomic loads, Thread-safe, Calculate rates]
        SetFPS[SetTargetFPS Method - Update capsfilter, Rollback on failure, 5s timeout]
        Warmup[Warmup Method - Consume frames 5s, Calculate FPS/jitter, Fail-fast if unstable]
    end

    subgraph "internal/rtsp Package"
        Pipeline[pipeline.go - CreatePipeline, VAAPI vs Software, Optimization Levels]
        Callbacks[callbacks.go - OnNewSample, Frame extraction, Latency telemetry]
        Reconnect[reconnect.go - RunWithReconnect, Exponential backoff, Max 5 retries]
        Errors[errors.go - ClassifyGStreamerError, 4 categories, Regex patterns]
    end

    subgraph "internal/warmup Package"
        WarmupStream[warmup.go - WarmupStream, FPS statistics, Jitter calculation]
        StatsCalc[stats_internal.go - calculateFPSStatsInternal, Mean, stddev, P95, Stability criteria]
    end

    subgraph "Frame Flow"
        FrameChan[Frame Channel - buffered 10, Non-blocking drops]
        FrameBus[Frame Distribution - Fan-out pattern, Drop telemetry]
    end

    Consumer[Consumer - Worker Lifecycle] -->|Read| StreamProvider
    StreamProvider --> RTSPStream
    RTSPStream --> Start
    RTSPStream --> Stop
    RTSPStream --> Stats
    RTSPStream --> SetFPS
    RTSPStream --> Warmup

    Start --> Pipeline
    Pipeline --> Callbacks
    Callbacks --> FrameChan
    FrameChan --> FrameBus
    FrameBus -->|frameChan| Consumer

    Start --> Reconnect
    Reconnect --> Errors

    Warmup --> WarmupStream
    WarmupStream --> StatsCalc

    Stop -->|Close| FrameChan

    classDef interface fill:#3498db,stroke:#333,stroke-width:3px,color:#fff
    classDef impl fill:#2ecc71,stroke:#333,stroke-width:2px
    classDef internal fill:#e74c3c,stroke:#333,stroke-width:2px,color:#fff
    classDef flow fill:#f39c12,stroke:#333,stroke-width:2px

    class StreamProvider interface
    class RTSPStream,Start,Stop,Stats,SetFPS,Warmup impl
    class Pipeline,Callbacks,Reconnect,Errors,WarmupStream,StatsCalc internal
    class FrameChan,FrameBus flow
```
# End of Selection


**Component Responsibilities**:

| Component | Responsibility | Thread-Safety | Key Files |
|-----------|---------------|---------------|-----------|
| **StreamProvider** | Public API contract | Interface (no state) | provider.go:17-124 |
| **RTSPStream** | Lifecycle orchestration | RWMutex + atomics | rtsp.go:16-822 |
| **Pipeline** | GStreamer pipeline creation | Stateless (called from Start) | internal/rtsp/pipeline.go |
| **Callbacks** | Frame extraction from GStreamer | Lock-free (atomic counters) | internal/rtsp/callbacks.go |
| **Reconnect** | Exponential backoff retry | Context-based cancellation | internal/rtsp/reconnect.go |
| **Errors** | GStreamer error classification | Stateless (pure function) | internal/rtsp/errors.go |
| **WarmupStream** | FPS stability validation | Consumes from channel | internal/warmup/warmup.go |
| **Frame Channel** | Non-blocking frame delivery | Go channel (buffered 10) | rtsp.go:126 |

---

## C4: Code Diagram

**Scope**: Key interfaces, structs, and functions (implementation details).

### Core Types

```mermaid
classDiagram
    class StreamProvider {
        <<interface>>
        +Start(ctx) (chan Frame, error)
        +Stop() error
        +Stats() StreamStats
        +SetTargetFPS(fps float64) error
        +Warmup(ctx, duration) (*WarmupStats, error)
    }

    class RTSPStream {
        -rtspURL string
        -width int
        -height int
        -targetFPS float64
        -acceleration HardwareAccel
        -elements *PipelineElements
        -frames chan Frame
        -ctx context.Context
        -cancel context.CancelFunc
        -frameCount uint64 (atomic)
        -framesDropped uint64 (atomic)
        -reconnectState *ReconnectState
        -decodeLatencies atomic.Pointer[LatencyWindow]
        +Start(ctx) (chan Frame, error)
        +Stop() error
        +Stats() StreamStats
        +SetTargetFPS(fps) error
        +Warmup(ctx, duration) (*WarmupStats, error)
    }

    class Frame {
        +Seq uint64
        +Timestamp time.Time
        +Width int
        +Height int
        +Data []byte
        +SourceStream string
        +TraceID string
    }

    class StreamStats {
        +FrameCount uint64
        +FramesDropped uint64
        +DropRate float64
        +FPSTarget float64
        +FPSReal float64
        +ErrorsNetwork uint64
        +ErrorsCodec uint64
        +DecodeLatencyMeanMS float64
        +DecodeLatencyP95MS float64
        +UsingVAAPI bool
    }

    class RTSPConfig {
        +URL string
        +Resolution Resolution
        +TargetFPS float64
        +Acceleration HardwareAccel
        +MaxReconnectAttempts int
    }

    class PipelineElements {
        +Pipeline *gst.Pipeline
        +AppSink *app.Sink
        +VideoRate *gst.Element
        +CapsFilter *gst.Element
        +RTSPSrc *gst.Element
        +UsingVAAPI bool
    }

    StreamProvider <|.. RTSPStream : implements
    RTSPStream --> Frame : produces
    RTSPStream --> StreamStats : provides
    RTSPStream --> PipelineElements : owns
    RTSPConfig --> RTSPStream : configures
```

### Key Algorithms

```mermaid
sequenceDiagram
    participant User
    participant RTSPStream
    participant Pipeline
    participant GStreamer
    participant Callbacks
    participant Consumer

    User->>RTSPStream: NewRTSPStream(cfg)
    RTSPStream->>RTSPStream: Validate config (fail-fast)
    RTSPStream-->>User: stream (or error)

    User->>RTSPStream: Start(ctx)
    RTSPStream->>Pipeline: CreatePipeline(cfg)
    Pipeline->>GStreamer: Build pipeline (VAAPI/Software)
    Pipeline-->>RTSPStream: PipelineElements
    RTSPStream->>GStreamer: SetState(PLAYING)
    RTSPStream->>RTSPStream: Launch goroutines (3x)
    RTSPStream-->>User: frameChan (non-blocking)

    Note over RTSPStream,GStreamer: Pipeline reaches PLAYING (~3s)

    loop Frame Processing
        GStreamer->>Callbacks: OnNewSample()
        Callbacks->>Callbacks: Extract RGB data
        Callbacks->>Callbacks: Capture decode latency (VAAPI)
        Callbacks->>Callbacks: Update atomic counters
        Callbacks->>RTSPStream: frameChan <- Frame (non-blocking)
        alt Channel full
            Callbacks->>Callbacks: Drop frame, increment framesDropped
        end
    end

    RTSPStream-->>Consumer: Frame via channel

    User->>RTSPStream: Warmup(ctx, 5s)
    RTSPStream->>RTSPStream: Consume frames for 5s
    RTSPStream->>RTSPStream: Calculate FPS/jitter stats
    alt Stream unstable
        RTSPStream-->>User: error (fail-fast)
    else Stream stable
        RTSPStream-->>User: WarmupStats (IsStable=true)
    end

    User->>RTSPStream: SetTargetFPS(1.0)
    RTSPStream->>Pipeline: UpdateFramerateCaps()
    Pipeline->>GStreamer: Update capsfilter (~2s interruption)
    alt Update fails
        RTSPStream->>Pipeline: Rollback to old FPS
        RTSPStream-->>User: error
    else Update succeeds
        RTSPStream-->>User: nil
    end

    User->>RTSPStream: Stop()
    RTSPStream->>RTSPStream: Cancel context
    RTSPStream->>RTSPStream: Wait goroutines (3s timeout)
    RTSPStream->>GStreamer: SetState(NULL)
    RTSPStream->>RTSPStream: Close frameChan (double-close protection)
    RTSPStream-->>User: nil
```

### Thread Safety Model

```mermaid
graph LR
    subgraph "Goroutine 1: Frame Converter"
        G1[Convert internal Frame<br/>to public Frame]
        G1 --> Update1[Update lastFrameAt<br/>sync.Mutex]
        Update1 --> Send[Send to frameChan<br/>non-blocking select]
        Send --> Drop[Drop if full<br/>atomic.AddUint64 framesDropped]
    end

    subgraph "Goroutine 2: Pipeline Monitor"
        G2[Monitor GStreamer bus<br/>50ms poll]
        G2 --> Error[On error:<br/>RunWithReconnect]
        Error --> Reconnect[Exponential backoff<br/>max 5 retries]
    end

    subgraph "Goroutine 3: GStreamer Callbacks"
        G3[OnNewSample<br/>GStreamer thread]
        G3 --> Extract[Extract frame data<br/>Map buffer]
        Extract --> Atomic1[atomic.AddUint64<br/>frameCount++]
        Atomic1 --> Latency[Lock-free latency update<br/>atomic.Pointer swap]
        Latency --> SendInternal[Send to internal chan]
    end

    subgraph "Main Goroutine"
        Main[User code]
        Main --> Start[Start call<br/>RWMutex.Lock]
        Main --> Stop[Stop call<br/>RWMutex.Lock]
        Main --> Stats[Stats call<br/>RWMutex.RLock]
        Main --> SetFPS[SetTargetFPS call<br/>RWMutex.Lock]
        Main --> Warmup[Warmup call<br/>RWMutex.RLock]
    end

    classDef goroutine fill:#3498db,stroke:#333,stroke-width:2px,color:#fff
    classDef atomic fill:#e74c3c,stroke:#333,stroke-width:2px,color:#fff
    classDef mutex fill:#f39c12,stroke:#333,stroke-width:2px

    class G1,G2,G3 goroutine
    class Atomic1,Drop,Latency atomic
    class Start,Stop,Stats,SetFPS,Warmup,Update1 mutex
```

**Thread-Safety Guarantees**:
- **RWMutex**: Protects pipeline state during Start/Stop/SetTargetFPS (exclusive), Stats/Warmup (shared)
- **Atomic operations**: frameCount, framesDropped, bytesRead, reconnects (lock-free)
- **atomic.Pointer**: decodeLatencies (lock-free read/write with copy-on-write pattern)
- **Channels**: Frame delivery (Go's built-in synchronization)
- **Context**: Goroutine lifecycle (cancel propagation)

---

## Key Architectural Decisions

### AD-1: Non-Blocking Frame Channel with Drop Policy

**Decision**: Use buffered channel (10 frames) with non-blocking send + drop tracking.

**Rationale**:
- Latency > Completeness (Orion philosophy)
- Prevents head-of-line blocking (slow consumer doesn't stall stream)
- Bounded memory (10 frames × 1280×720×3 bytes ≈ 26 MB worst case)
- Drop telemetry enables monitoring (DropRate in StreamStats)

**Trade-offs**:
- ✅ Predictable latency (<2s target)
- ✅ Graceful degradation (drops instead of crashes)
- ❌ Frame loss at high drop rates (>10% indicates consumer too slow)

**Code**: rtsp.go:246-258 (non-blocking select with drop tracking)

---

### AD-2: Fail-Fast Validation at Construction

**Decision**: Validate all configuration at `NewRTSPStream()` construction time.

**Rationale**:
- Detect errors early (before Start, before goroutines)
- Clear error messages at configuration time (not buried in runtime logs)
- Follows Go idiom: "Make zero values useful, but validate when needed"

**Validated at construction**:
- RTSP URL not empty
- Target FPS in range (0.1 - 30.0)
- Resolution valid
- GStreamer availability
- VAAPI availability (if forced)

**Code**: rtsp.go:64-106

---

### AD-3: Warmup Fail-Fast Pattern

**Decision**: `Warmup()` returns error if stream is unstable (FPS stddev > 15% or jitter > 20%).

**Rationale**:
- Prevents production use of unreliable streams
- Forces operator to fix network/camera issues before deployment
- Clear signal: "This stream is not production-ready"

**Stability Criteria**:
- FPS standard deviation < 15% of mean
- Jitter mean < 20% of expected interval
- Minimum 2 frames received

**Code**: rtsp.go:763-769 (fail-fast error return)

---

### AD-4: Lock-Free Telemetry with atomic.Pointer

**Decision**: Use `atomic.Pointer[LatencyWindow]` for decode latency tracking.

**Rationale**:
- Hot path (every frame) - locks would kill performance
- Copy-on-write pattern: read old, modify copy, swap pointer
- Bounded memory (ring buffer, 100 samples)
- Lock-free reads in Stats() method (no contention)

**Performance**: ~50-100ns overhead per frame (vs ~500ns with mutex)

**Code**: internal/rtsp/callbacks.go:126-146 (atomic pointer swap)

---

### AD-5: Exponential Backoff with Max 5 Retries

**Decision**: Reconnection uses exponential backoff (1s, 2s, 4s, 8s, 16s), then stops.

**Rationale**:
- "KISS Auto-Recovery" - one attempt at recovery, not infinite loops
- Persistent failures indicate deeper issues (corrupt model, missing deps)
- Prevents log spam and resource exhaustion
- Operator intervention required for persistent failures

**Trade-offs**:
- ✅ Simple, predictable behavior
- ✅ Prevents infinite restart loops
- ❌ Requires manual intervention after max retries

**Code**: internal/rtsp/reconnect.go:51-104

---

### AD-6: VAAPI with Auto-Fallback

**Decision**: Default acceleration mode is `AccelAuto` (try VAAPI, fallback to software).

**Rationale**:
- Best of both worlds: performance when available, reliability always
- Transparent to consumer (same API, same Frame format)
- Avoids deployment complexity (works on any hardware)

**Fallback Conditions**:
- VAAPI elements not installed (gstreamer1.0-vaapi)
- Hardware not compatible (AMD GPU, old Intel)
- VM environment (no GPU passthrough)

**Code**: internal/rtsp/pipeline.go:152-205 (auto-detection logic)

---

### AD-7: H.264-Specific Decoder

**Decision**: Use `vaapih264dec` (H.264-specific) instead of `vaapidecodebin` (generic).

**Rationale**:
- Eliminates codec probing overhead (~100ms at startup)
- Enables H.264-specific optimizations (low-latency mode for no B-frames)
- Clear failure mode (H.265 not supported, error at construction)

**Trade-offs**:
- ✅ Faster startup (no codec probing)
- ✅ Lower latency (H.264 optimizations)
- ❌ No H.265/HEVC support (requires pipeline change)

**Code**: internal/rtsp/pipeline.go:102-115 (vaapih264dec selection)

---

### AD-8: RGB Format Lock

**Decision**: Add `capsRGB` capsfilter after `videoconvert` to force RGB format.

**Rationale**:
- Prevents caps negotiation issues between VAAPI (NV12) and final RGB
- Without capsRGB: GStreamer attempts runtime negotiation, fails with `video/x-raw(memory:VASurface)`
- With capsRGB: Format locked early, GStreamer handles GPU→CPU transfer automatically

**Trade-offs**:
- ✅ Reliable caps negotiation
- ✅ No runtime failures
- ❌ One extra element in pipeline (minimal overhead)

**Code**: internal/rtsp/pipeline.go:283-295

---

## Diagram Legend

### C1 (Context) Colors:
- 🔵 External Systems (light blue)
- 🟢 Orion 2.0 Modules (green)
- 🟡 Infrastructure (yellow)

### C2 (Container) Colors:
- 🔵 Go Components (cyan)
- 🟠 GStreamer Runtime (orange)
- ⚫ System Libraries (gray)

### C3 (Component) Colors:
- 🔵 Public Interface (blue)
- 🟢 Core Implementation (green)
- 🔴 Internal Packages (red)
- 🟠 Frame Flow (orange)

---

## Future Enhancements (Out of Scope for v1.0)

1. **Multi-stream support** (C3: Add StreamPool component)
2. **H.265/HEVC codec** (C3: Add codec detection + vaapih265dec)
3. **UDP transport** (C3: Modify pipeline.go protocols parameter)
4. **Dynamic ROI** (C3: Add vaapipostproc crop parameters)
5. **Probe functionality** (C3: Re-enable GStreamer mainloop for bus probes)

---

## References

- **CLAUDE.md**: Module documentation for AI companion
- **ARCHITECTURE.md**: Orion 2.0 global architecture (4+1 views)
- **Code**: modules/stream-capture/*.go
- **C4 Model**: https://c4model.com/

---

**Maintained by**: Orion Architecture Team
**For questions**: See CLAUDE.md or contact team lead
