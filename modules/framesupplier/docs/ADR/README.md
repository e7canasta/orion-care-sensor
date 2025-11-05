# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the FrameSupplier module.

## ADR Index

| ID      | Title                                  | Status   | Date       |
|---------|----------------------------------------|----------|------------|
| ADR-001 | sync.Cond for Mailbox Semantics        | Accepted | 2025-01-05 |
| ADR-002 | Zero-Copy Frame Sharing                | Accepted | 2025-01-05 |
| ADR-003 | Batching with Threshold=8              | Accepted | 2025-01-05 |
| ADR-004 | Symmetric JIT Architecture             | Accepted | 2025-01-05 |

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

| Version | Date       | Author          | Changes                    |
|---------|------------|-----------------|----------------------------|
| 1.0     | 2025-01-05 | Ernesto + Gaby  | Initial ADR catalog        |
