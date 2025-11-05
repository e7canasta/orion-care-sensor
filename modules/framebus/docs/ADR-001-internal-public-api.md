# ADR-001: Internal/Public API Boundary Design

**Status:** Proposed  
**Date:** 2025-11-05  
**Authors:** Claude Code + Gaby de Visiona  

## Context

FrameBus necesita evolucionar su implementación (Option 1 → Option 2/3) sin romper contratos con consumidores. Siguiendo el estilo Amazon, las APIs públicas son **contratos permanentes** que requieren versionamiento semántico para cambios breaking.

**Problema actual:**
- Todo está en `package framebus` (público)
- Implementación (`bus`, `latestFrameHolder`) expuesta
- Sin capacidad de refactorizar internals sin breaking changes

**Objetivo:**
- Public API: Contrato estable e inmutable
- Internal API: Libre evolución (Option 1 → 2 → 3)
- Clear upgrade path para futuros cambios

## Decision

### Public API Surface (Package `framebus`)

Exponer **solo** abstracciones y tipos necesarios para consumidores:

```go
// framebus.go (PUBLIC CONTRACT)
package framebus

// Core Types - IMMUTABLE after v1.0
type Frame struct {
    Data      []byte
    Width     int
    Height    int
    Sequence  uint64
    Timestamp int64
    Meta      map[string]interface{}
}

type DropPolicy int

const (
    DropNew DropPolicy = iota
    DropOld
)

// Public Interfaces - VERSIONED CONTRACTS
type Bus interface {
    Subscribe(id string, ch chan<- Frame) error
    SubscribeDropOld(id string) (FrameReceiver, error)
    Publish(frame Frame)
    Unsubscribe(id string) error
    Stats(id string) (*SubscriberStats, error)
    Close()
}

type FrameReceiver interface {
    Receive() Frame
    TryReceive() (Frame, bool)
    Close()
}

type SubscriberStats struct {
    Sent    uint64
    Dropped uint64
}

// Public Errors - STABLE
var (
    ErrBusClosed          = errors.New("framebus: bus is closed")
    ErrSubscriberExists   = errors.New("framebus: subscriber already exists")
    ErrSubscriberNotFound = errors.New("framebus: subscriber not found")
    ErrNilChannel         = errors.New("framebus: nil channel provided")
    ErrReceiverClosed     = errors.New("framebus: receiver is closed")
)

// Factory Function - ONLY public constructor
func New() Bus
```

### Internal Implementation (Package `internal/bus`)

Toda la lógica de implementación, libre para evolucionar:

```go
// internal/bus/bus.go (PRIVATE - Can change freely)
package bus

import "github.com/e7canasta/orion-care-sensor/modules/framebus"

type subscriberHolder struct { /* ... */ }
type bus struct { /* ... */ }
type latestFrameHolder struct { /* ... */ }

func NewBus() framebus.Bus {
    return &bus{
        subscribers: make(map[string]*subscriberHolder),
    }
}
```

## Architecture

```
framebus/                           # Public API Package
├── framebus.go                     # Interfaces, types, errors (STABLE)
├── doc.go                          # Package documentation
└── internal/
    ├── bus/
    │   ├── bus.go                  # Bus implementation (EVOLVES)
    │   ├── receiver.go             # FrameReceiver impl (EVOLVES)
    │   └── subscriber.go           # Subscriber management
    ├── policy/
    │   ├── dropnew.go              # DropNew strategy
    │   └── dropold.go              # DropOld strategy
    └── ringbuffer/                 # Future: Option 2 implementation
        └── ringbuffer.go
```

## Benefits

### 1. **Amazon-Style Immutability**
- Public API = Permanent contract
- Breaking changes require new major version
- Internal changes = zero consumer impact

### 2. **Clear Evolution Path**
```
v1.0.0 → internal/bus/bus.go (Option 1: Mutex)
v1.1.0 → internal/bus/bus.go (Option 2: RingBuffer)  # SAME PUBLIC API
v2.0.0 → framebus.go changes (new interface method)  # VERSIONED
```

### 3. **Testability**
```go
// Public API tests (contract validation)
framebus_test.go

// Internal implementation tests (white-box)
internal/bus/bus_test.go
internal/policy/dropold_test.go
```

### 4. **Documentation Separation**
- `framebus/doc.go`: User-facing API guide
- `internal/bus/doc.go`: Implementation notes for maintainers

## Migration Plan

### Phase 1: Extract Internal Package (Non-breaking)
1. Move `bus`, `latestFrameHolder`, `subscriberHolder` → `internal/bus/`
2. Keep public API in `framebus.go`
3. Update `New()` to call `bus.NewBus()`
4. All tests pass, zero API changes

### Phase 2: Strategy Pattern for Drop Policies (Non-breaking)
1. Create `internal/policy/{dropnew,dropold}.go`
2. Refactor `Publish()` to use strategy pattern
3. Prep for future policies (DropPriority, DropOldest-N)

### Phase 3: Optional - RingBuffer (Non-breaking)
1. Implement `internal/ringbuffer/` (Option 2)
2. Add factory option: `NewWithRingBuffer()`
3. Benchmark and decide if needed

## Comparison with Alternatives

### Alternative 1: Everything Public
**Rejected** - No evolution path without breaking changes

### Alternative 2: Separate Module per Implementation
```
framebus-core/       # Interfaces only
framebus-mutex/      # Option 1
framebus-lockfree/   # Option 3
```
**Rejected** - Over-engineering for current needs, complex dependency management

### Alternative 3: Feature Flags
```go
New(WithRingBuffer())  // Dynamic selection
```
**Deferred** - Good for v2.0 when multiple backends exist

## ADR Dependencies

- Depends on: None
- Impacts: Future ADR-002 (Priority-based dropping)
- Impacts: Future ADR-003 (Multi-stream support)

## Validation Criteria

✅ Public API compiles separately from internal  
✅ Internal changes don't require consumer code updates  
✅ Tests separated by scope (contract vs implementation)  
✅ Documentation clearly states public vs internal  
✅ Semantic versioning enforced (pre-commit hooks)  

## References

- [Go Project Layout](https://github.com/golang-standards/project-layout)
- [AWS API Design Guidelines](https://aws.amazon.com/builders-library/)
- [Hyrum's Law](https://www.hyrumslaw.com/)
- [Semantic Import Versioning](https://research.swtch.com/vgo-import)

---

Co-authored-by: Gaby de Visiona <noreply@visiona.app>
