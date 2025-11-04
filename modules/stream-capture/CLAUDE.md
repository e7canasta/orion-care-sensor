# CLAUDE.md - Stream Capture

This file provides guidance to Claude Code when working with this module.

## Module Overview

**Bounded Context**: Stream Acquisition
**Module Path**: `github.com/e7canasta/orion-care-sensor/modules/stream-capture`
**Version**: v0.1.0
**Sprint**: Sprint 1.1

---

## 📋 Responsibility

**What this module DOES**:
- ✅ Capture RTSP video frames via GStreamer
- ✅ Automatic reconnection on stream failure
- ✅ Adaptive FPS measurement during warm-up

---

## 🚫 Anti-Responsibility

**What this module DOES NOT do**:
- ❌ Does NOT process frames (that's FrameBus)
- ❌ Does NOT decide what to capture (that's Control Plane)
- ❌ Does NOT know about workers (that's Worker Lifecycle)

---

## 🔌 Public API

### Interfaces

```go
// stream-capture/provider.go
package streamcapture

type StreamProvider interface {
    Start(ctx context.Context) (<-chan Frame, error)
    Stop() error
    SetTargetFPS(fps float64) error
}
```

### Types

```go
// stream-capture/types.go
package streamcapture

type RTSPStream struct {
    url string
    targetFPS float64
    pipeline *gst.Pipeline
    frameChan chan Frame
}
```

---

## 🏗️ Internal Structure

```
modules/stream-capture/
├── go.mod
├── CLAUDE.md                  # This file
├── README.md                  # User-facing overview
├── BACKLOG.md                 # Sprint-specific tasks
├── docs/
│   ├── DESIGN.md              # Architectural decisions
│   └── proposals/             # RFCs before implementation
├── internal/
│   ├── rtsp/
│   └── warmup/
├── provider.go     # Public interface
├── capture.go # Implementation
├── types.go                   # Exported types
└── stream-capture_test.go     # Tests
```

---

## 📦 Dependencies

### Internal Packages

- `internal/rtsp` - GStreamer RTSP pipeline setup and management
- `internal/warmup` - FPS measurement during 5-second warm-up period

### External Modules

{{#if HAS_EXTERNAL_DEPS}}
- `{{EXTERNAL_MODULE_1}}` - {{EXTERNAL_MODULE_1_PURPOSE}}
- `{{EXTERNAL_MODULE_2}}` - {{EXTERNAL_MODULE_2_PURPOSE}}
{{else}}
None (leaf module)
{{/if}}

### Workspace Modules

{{#if HAS_WORKSPACE_DEPS}}
- `github.com/e7canasta/orion-care-sensor/modules/{{WORKSPACE_DEP_1}}` - {{WORKSPACE_DEP_1_PURPOSE}}
{{else}}
None (independent module)
{{/if}}

---

## ⚙️ Configuration

### Config Structure

```yaml
# Reads from workspace-level config/orion.yaml
camera:
  rtsp_url: rtsp://camera-ip/stream
  resolution: 720p
```

### Environment Variables

{{#if HAS_ENV_VARS}}
- `{{ENV_VAR_1}}` - {{ENV_VAR_1_DESCRIPTION}}
- `{{ENV_VAR_2}}` - {{ENV_VAR_2_DESCRIPTION}}
{{else}}
None (uses workspace config only)
{{/if}}

---

## 🧪 Testing

### Unit Tests

```bash
# Run module-specific tests
cd modules/stream-capture
go test ./...

# Run with coverage
go test -cover ./...

# Run with race detector
go test -race ./...
```

### Integration Tests

{{#if HAS_INTEGRATION_TESTS}}
```bash
# Integration tests (requires external dependencies)
go test -tags=integration ./...
```
{{else}}
_To be implemented in Sprint 1.1_
{{/if}}

### Test Organization

- Unit tests: `stream-capture_test.go`, `internal/*/..._test.go`
- Integration tests: `integration_test.go` (tag: `integration`)
- Mocks: `internal/mocks/` (if needed)

---

## 🔧 Development Workflow

### Before Coding

1. ✅ Read workspace `CLAUDE.md` + this module's `CLAUDE.md`
2. ✅ Review `BACKLOG.md` for sprint-specific tasks
3. ✅ Check `docs/DESIGN.md` for architectural decisions
4. ✅ Understand bounded context boundaries

### During Development

1. ✅ Preserve public API (breaking changes require ADR)
2. ✅ Keep `internal/` truly internal (implementation details)
3. ✅ Update tests alongside code
4. ✅ Compile frequently: `go build ./...`

### After Coding

1. ✅ Run tests: `go test ./...`
2. ✅ Update `BACKLOG.md` with lessons learned
3. ✅ Update `docs/DESIGN.md` if architecture changed
4. ✅ Create ADR if significant decision made

---

## 📚 Backlog

See [BACKLOG.md](BACKLOG.md) for Sprint 1.1 specific tasks.

---

## 🎯 Design Decisions

See [docs/DESIGN.md](docs/DESIGN.md) for architectural decisions specific to this module.

For workspace-level ADRs, see [../../docs/DESIGN/ADR/](../../docs/DESIGN/ADR/).

---

## 🔗 References

### Workspace Documentation

- [C4 Model](../../docs/DESIGN/C4_MODEL.md#c3---component-diagram)
- [Plan Evolutivo](../../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#11-stream-capture-module)
- [Workspace CLAUDE.md](../../CLAUDE.md)

### Module-Specific

- [README.md](README.md) - User-facing overview
- [BACKLOG.md](BACKLOG.md) - Sprint tasks
- [docs/DESIGN.md](docs/DESIGN.md) - Design decisions

---

## 🎸 Philosophy

**Bounded Context Enforcement**:
- This module is responsible ONLY for Stream Acquisition
- Anti-responsibilities are as important as responsibilities
- Public API is contract, `internal/` is implementation

**Complejidad por Diseño**:
- Attack complexity through architecture, not code tricks
- Keep `internal/` packages focused (SRP)
- Document architectural decisions (ADR style)

**KISS Auto-Recovery**:
- Fail fast at load time, not runtime
- Simple error handling (no panic recovery unless explicit)
- Log errors, continue processing (graceful degradation)

---

**Last updated**: 2025-11-03
**Authors**: Ernesto (Visiona) + Gaby (AI Companion)
