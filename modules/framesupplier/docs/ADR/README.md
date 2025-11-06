# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the FrameSupplier module.

---

## 🎯 Start Here

**NEW to FrameSupplier?** Read in this order:
1. **[ADR-000: Architecture Workflow](./000-architecture-workflow.md)** - How we document and evolve decisions (meta-ADR, read FIRST)
2. **[ADR Dependency Graph](./ADR-DEPENDENCY-GRAPH.md)** - Visual map of how decisions relate
3. **Choose your path**: Implementation-first, Architecture-first, or Problem-driven (see below)

---

## 📊 Visual Navigation

**Recommended reading paths**:
- **Implementation-first** (bottom-up): ADR-001 → 002 → 003 → 004 → 005
- **Architecture-first** (top-down): ADR-004 → 001 → 002 → 003 → 005
- **Problem-driven** (discovery order): See dependency graph § Evolution Timeline

---

## ADR Index

| ID      | Title                                  | Status   | Date       | Category      |
|---------|----------------------------------------|----------|------------|---------------|
| ADR-000 | Architecture Workflow                  | Accepted | 2025-01-05 | 📚 Meta       |
| ADR-001 | sync.Cond for Mailbox Semantics        | Accepted | 2025-01-05 | 🔧 Primitives |
| ADR-002 | Zero-Copy Frame Sharing                | Accepted | 2025-01-05 | 🔧 Primitives |
| ADR-003 | Batching with Threshold=8              | Accepted | 2025-01-05 | ⚡ Performance |
| ADR-004 | Symmetric JIT Architecture             | Accepted | 2025-01-05 | 🏛️ Architecture |
| ADR-005 | Graceful Shutdown Semantics            | Accepted | 2025-01-05 | 🔄 Lifecycle  |

## Template

Each ADR follows this structure:

```markdown
# ADR-XXX: Title

**Status**: Proposed | Accepted | Deprecated | Superseded
**Date**: YYYY-MM-DD
**Authors**: Names

## Context
What is the issue we're facing?

## Decision
What decision did we make?

## Consequences
What are the trade-offs?
- ✅ Positive consequences
- ❌ Negative consequences

## Alternatives Considered
What other options did we evaluate?

## References
Links to related documents
```

## Changelog

| Version | Date       | Author          | Changes                                |
|---------|------------|-----------------|----------------------------------------|
| 1.0     | 2025-01-05 | Ernesto + Gaby  | Initial ADR catalog (001-005)          |
| 1.1     | 2025-01-05 | Ernesto + Gaby  | Added ADR-000 (workflow meta-pattern)  |
