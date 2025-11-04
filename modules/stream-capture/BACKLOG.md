# Backlog - Stream Capture

**Sprint**: Sprint 1.1
**Module**: `modules/stream-capture`
**GitHub Issue**: [#1](https://github.com/e7canasta/orion-care-sensor/issues/1)

---

## 🎯 Sprint Goal

Implement RTSP stream capture with reconnection and FPS adaptation

---

## 📋 Tasks

| Task | Status | Estimación | Owner |
|------|--------|------------|-------|
| Setup GStreamer pipeline for RTSP | ⬜ Todo | 3 días | Ernesto + Gaby |
| Implement reconnection logic | ⬜ Todo | 3 días | Ernesto + Gaby |
| Add FPS warm-up measurement | ⬜ Todo | 2 días | Ernesto + Gaby |
| Integrate with FrameBus | ⬜ Todo | 2 días | Ernesto + Gaby |

**Total estimado**: 2 semanas

---

## ✅ Acceptance Criteria

### Functional

- [ ] RTSP stream captures frames correctly
- [ ] Reconnection works after stream failure
- [ ] FPS measured during 5-second warm-up

### Non-Functional

- [ ] Latency < 2 seconds
- [ ] Graceful degradation on errors
- [ ] Memory usage stable (no leaks)

### Testing

- [ ] Unit tests: 80% coverage
- [ ] Integration tests: Test with real RTSP camera
- [ ] Manual testing: Visual inspection of frame capture

---

## 🏗️ Implementation Plan

### Phase 1: Setup (3 días)

**Goal**: GStreamer pipeline functional

**Tasks**:
1. Setup GStreamer CGo bindings
2. Create RTSP pipeline (rtspsrc → videorate → jpegenc)
3. Emit frames to channel

**Deliverables**:
- RTSPStream implementation
- Basic frame capture working

---

### Phase 2: Core Implementation (5 días)

**Goal**: Reconnection and FPS adaptation

**Tasks**:
1. Implement reconnection on failure
2. Add FPS warm-up measurement (5s)
3. SetTargetFPS triggers reconnection

**Deliverables**:
- Reconnection logic implemented
- FPS adaptation working

---

### Phase 3: Testing & Integration (2 días)

**Goal**: Testing and integration

**Tasks**:
1. Unit tests with mock RTSP
2. Integration tests with real camera
3. FrameBus integration validation

**Deliverables**:
- 80% test coverage
- Integration validated

---

## 🔧 Technical Details

### Public API Design

```go
// stream-capture/{{API_FILE}}.go
package streamcapture

type {{INTERFACE_NAME}} interface {
    Start(ctx context.Context) (<-chan Frame, error)  // Start streaming, returns channel of frames
    Stop() error  // Stop streaming gracefully
}
```

### Internal Structure

```
modules/stream-capture/
├── internal/
│   ├── rtsp/  # GStreamer pipeline management
│   └── warmup/  # FPS measurement logic
├── {{API_FILE}}.go              # Public interface
├── {{IMPL_FILE}}.go             # Implementation
└── types.go                     # Exported types
```

### Dependencies

**Required**:
- {{DEPENDENCY_1}} - {{DEPENDENCY_1_PURPOSE}}
- {{DEPENDENCY_2}} - {{DEPENDENCY_2_PURPOSE}}

**Optional**:
- {{OPTIONAL_DEP_1}} - {{OPTIONAL_DEP_1_PURPOSE}}

---

## 🚧 Blockers

{{#if HAS_BLOCKERS}}
| Blocker | Impact | Resolution |
|---------|--------|------------|
| {{BLOCKER_1}} | {{BLOCKER_1_IMPACT}} | {{BLOCKER_1_RESOLUTION}} |
{{else}}
_Ninguno por ahora_
{{/if}}

---

## 🤔 Decisiones Pendientes

{{#if HAS_PENDING_DECISIONS}}
- [ ] {{DECISION_1}} - _Opciones: {{DECISION_1_OPTIONS}}_
- [ ] {{DECISION_2}} - _Opciones: {{DECISION_2_OPTIONS}}_
{{else}}
_Ninguna por ahora_
{{/if}}

---

## 📝 Session Checklist

### Antes de codear

- [ ] Leo workspace `CLAUDE.md` + module `CLAUDE.md`
- [ ] Identifico bounded context (Responsibility + Anti-responsibility)
- [ ] Reviso `docs/DESIGN.md` para decisiones existentes
- [ ] Propongo 2-3 opciones de diseño (si aplica)
- [ ] Evalúo trade-offs con Ernesto
- [ ] Elijo "quick win"

### Durante desarrollo

- [ ] Commits atómicos
- [ ] Compilo después de cada paso: `go build ./...`
- [ ] Tests unitarios + integration
- [ ] Preservo API pública (breaking changes → ADR)

### Después de codear

- [ ] Pair review con Ernesto
- [ ] Actualizo `CLAUDE.md` si API cambió
- [ ] Actualizo `docs/DESIGN.md` si arquitectura cambió
- [ ] Documento lecciones aprendidas (sección abajo)
- [ ] Identifico próximos pasos

---

## 💡 Lecciones Aprendidas

_Se actualizará al completar el sprint_

### Lo que Funcionó Bien ✅

- {{LESSON_SUCCESS_1}}
- {{LESSON_SUCCESS_2}}

### Mejoras para Próximas Sesiones 📈

- {{LESSON_IMPROVEMENT_1}}
- {{LESSON_IMPROVEMENT_2}}

### Deuda Técnica Identificada 🚨

{{#if HAS_TECH_DEBT}}
- {{TECH_DEBT_1}} - _Prioridad: {{TECH_DEBT_1_PRIORITY}}_
- {{TECH_DEBT_2}} - _Prioridad: {{TECH_DEBT_2_PRIORITY}}_
{{else}}
_Ninguna por ahora_
{{/if}}

---

## 🔗 Referencias

### Workspace Documentation

- [C4 Model - Stream Capture Component](../../docs/DESIGN/C4_MODEL.md#c3---component-diagram)
- [Plan Evolutivo - Sprint 1.1](../../docs/DESIGN/ORION_2.0_PLAN_EVOLUTIVO.md#11-stream-capture-module)
- [BACKLOG - Fase 1](../../BACKLOG/FASE_1_FOUNDATION.md#sprint-11-stream-capture-module)

### Module Documentation

- [CLAUDE.md](CLAUDE.md) - Module guide
- [README.md](README.md) - User-facing overview
- [docs/DESIGN.md](docs/DESIGN.md) - Design decisions

---

**Última actualización**: 2025-11-03
**Estado**: 🔄 In Progress
**Próximo paso**: Implement GStreamer pipeline setup
