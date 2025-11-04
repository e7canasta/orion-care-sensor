# Stream Capture

**Bounded Context**: Stream Acquisition
**Version**: v0.1.0
**Status**: 🔄 In Development (Sprint 1.1)

---

## 📋 Overview

Stream Capture module handles RTSP video stream acquisition with automatic reconnection and FPS adaptation. It provides frames to downstream consumers via a channel.

**Key Features**:
- ✅ RTSP video capture via GStreamer
- ✅ Automatic reconnection on failure
- ✅ FPS measurement and adaptation

---

## 🎯 Responsibility

This module is responsible for:
- Capture RTSP video frames via GStreamer
- Automatic reconnection on stream failure
- Adaptive FPS measurement during warm-up

**Anti-responsibilities** (what this module does NOT do):
- ❌ Does NOT process frames (that's FrameBus)
- ❌ Does NOT decide what to capture (that's Control Plane)

---

## 🚀 Quick Start

### Installation

```bash
# From workspace root
cd modules/stream-capture

# Install dependencies
go mod download
```

### Usage

```go
import "github.com/e7canasta/orion-care-sensor/modules/stream-capture"

func main() {
    // Example usage
    stream := NewRTSPStream("rtsp://camera/stream", WithTargetFPS(30))
    frames, _ := stream.Start(ctx)
    for frame := range frames {
        // Process frame
    }
}
```

---

## 🔌 Public API

### Interfaces

#### StreamProvider

```go
type StreamProvider interface {
    {{INTERFACE_METHOD_1}}
    {{INTERFACE_METHOD_2}}
    {{INTERFACE_METHOD_3}}
}
```

**Implementations**:
- `RTSPStream` - Production RTSP stream via GStreamer
- `MockStream` - {{IMPLEMENTATION_2_DESCRIPTION}} (if applicable)

### Types

#### RTSPStream

```go
type RTSPStream struct {
    {{FIELD_1}} {{FIELD_1_TYPE}}  // {{FIELD_1_DESCRIPTION}}
    {{FIELD_2}} {{FIELD_2_TYPE}}  // {{FIELD_2_DESCRIPTION}}
}
```

---

## ⚙️ Configuration

{{#if HAS_CONFIG}}
```yaml
# config/orion.yaml (workspace-level)
camera:
  rtsp_url: rtsp://camera-ip/stream  # RTSP stream URL
  resolution: 720p  # Target resolution (512p/720p/1080p)
```
{{else}}
This module does not require configuration.
{{/if}}

---

## 🧪 Testing

### Run Tests

```bash
# Unit tests
cd modules/stream-capture
go test ./...

# Integration tests
go test -tags=integration ./...

# With coverage
go test -cover ./...
```

### Test Coverage

- Unit tests: `stream-capture_test.go`
- Integration tests: `integration_test.go`
- Current coverage: 80% (target: 80%)

---

## 📦 Dependencies

### External

{{#if HAS_EXTERNAL_DEPS}}
- `github.com/tinyzimmer/go-gst/gst` - GStreamer Go bindings
- `github.com/tinyzimmer/go-glib/glib` - GLib bindings (required by GStreamer)
{{else}}
None (pure Go stdlib)
{{/if}}

### Workspace Modules

{{#if HAS_WORKSPACE_DEPS}}
- `modules/{{WORKSPACE_DEP_1}}` - {{WORKSPACE_DEP_1_PURPOSE}}
{{else}}
None (leaf module)
{{/if}}

---

## 🏗️ Architecture

### Component Diagram

```
Stream Capture
├── Public API (StreamProvider)
├── Implementation (capture.go)
└── Internal
    ├── rtsp  (GStreamer pipeline management)
    └── warmup  (FPS measurement logic)
```

### Bounded Context

See [C4 Model - Stream Capture Component](../../docs/DESIGN/C4_MODEL.md#c3---component-diagram)

---

## 📚 Documentation

- [CLAUDE.md](CLAUDE.md) - AI companion guide
- [BACKLOG.md](BACKLOG.md) - Sprint tasks
- [docs/DESIGN.md](docs/DESIGN.md) - Architectural decisions
- [docs/proposals/](docs/proposals/) - RFCs

---

## 🔗 Related Modules

{{#if HAS_RELATED_MODULES}}
- [framebus](../framebus) - Consumes frames from stream-capture
- [{{RELATED_MODULE_2}}](../{{RELATED_MODULE_2}}) - {{RELATED_MODULE_2_RELATION}}
{{else}}
None (independent module)
{{/if}}

---

## 📝 Changelog

### v0.1.0 (2025-11-03)

- ✅ Initial implementation (Sprint 1.1)
- ✅ RTSP video capture via GStreamer implemented
- ✅ Automatic reconnection on failure implemented
- ✅ Basic test coverage

---

## 🤝 Contributing

See workspace [CONTRIBUTING.md](../../CONTRIBUTING.md) for contribution guidelines.

---

**Maintained by**: Visiona Team
**License**: MIT
**Status**: 🔄 Active Development
