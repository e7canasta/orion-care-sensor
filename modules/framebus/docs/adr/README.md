# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for the FrameBus module.

## Active ADRs

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [001](001-channel-based-subscriber-pattern.md) | Channel-based Subscriber Pattern | ACCEPTED | 2025-11-04 |
| [002](002-non-blocking-publish-drop-policy.md) | Non-blocking Publish with Drop Policy | ACCEPTED | 2025-11-04 |
| [003](003-stats-tracking-design.md) | Stats Tracking Design | ACCEPTED | 2025-11-04 |

## What is an ADR?

An Architecture Decision Record (ADR) is a document that captures an important architectural decision made along with its context and consequences.

Each ADR includes:
- **Status**: PROPOSED, ACCEPTED, DEPRECATED, SUPERSEDED
- **Context**: The issue motivating this decision
- **Decision**: The change being proposed or accepted
- **Rationale**: Why this decision over alternatives
- **Consequences**: Positive and negative outcomes
- **Trade-offs Considered**: Alternatives evaluated

## ADR Process

1. **Propose**: Create ADR file with PROPOSED status
2. **Review**: Discuss in design session or PR
3. **Accept**: Update status to ACCEPTED when consensus reached
4. **Implement**: Code must align with accepted ADRs
5. **Update**: If decision changes, create new ADR or deprecate old one

## Key Decisions Summary

### ADR-001: Channel-based Subscribers
**Decision**: Use Go channels instead of interfaces for subscribers.

**Why**: Decouples FrameBus from subscriber types, simplifies API, enables idiomatic Go patterns.

**Impact**: Subscribers just create a channel, no interface to implement.

---

### ADR-002: Drop Policy
**Decision**: Non-blocking publish with intentional frame dropping.

**Why**: Maintains bounded latency for real-time processing. Recent frames > stale backlog.

**Impact**: High drop rates (70-97%) are expected and correct behavior.

---

### ADR-003: Stats Tracking
**Decision**: Track comprehensive stats (global + per-subscriber) but don't log automatically.

**Why**: Stats collection is FrameBus responsibility, interpretation is consumer responsibility.

**Impact**: Consumers must implement observability, but have full flexibility.

## References

- [ADR GitHub](https://adr.github.io/) - ADR methodology
- [Orion Design Manifesto](../../../../docs/DESIGN/MANIFESTO_DISENO.md)
- [C4 Model](../../../../docs/DESIGN/C4_MODEL.md)
