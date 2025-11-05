# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for the FrameBus module.

## Active ADRs

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [001](001-channel-based-subscriber-pattern.md) | Channel-based Subscriber Pattern | ACCEPTED | 2025-11-04 |
| [002](002-non-blocking-publish-drop-policy.md) | Non-blocking Publish with Drop Policy | ACCEPTED | 2025-11-04 |
| [003](003-stats-tracking-design.md) | Stats Tracking Design | ACCEPTED | 2025-11-04 |
| [004](004-subscribe-lock-strategy.md) | Lock-Free Subscribe (Propuesta) | REJECTED | 2025-11-05 |
| [005](005-error-return-for-closed-bus.md) | Error Return for Closed Bus (Propuesta) | REJECTED | 2025-11-05 |
| [006](006-channel-ownership-no-close.md) | Channel Ownership - No Close by FrameBus | ACCEPTED | 2025-11-05 |
| [007](007-concurrent-fan-out.md) | Concurrent Fan-out for Frame Distribution | ACCEPTED | 2025-11-05 |

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

---

### ADR-004: Lock-Free Subscribe (Propuesta) - REJECTED
**Proposal**: Replace RWMutex with lock-free CAS pattern for Subscribe/Unsubscribe.

**Decision**: REJECTED - Maintain current RWMutex implementation.

**Why**: Subscribe is cold path (~1/min) vs Publish hot path (30/sec). Lock-free CAS adds 50% complexity for 0.0005% performance benefit. Pragmatism > Purism.

**Impact**: Documents why lock-free was evaluated and rejected. Re-evaluate if Subscribe frequency > 1 Hz or contention > 1%.

---

### ADR-005: Error Return for Closed Bus (Propuesta) - REJECTED
**Proposal**: Replace `panic` with `error` return when Publish() called on closed bus.

**Decision**: REJECTED - Maintain current panic behavior.

**Why**: Publish-after-Close is programmer error (incorrect shutdown sequence), not expected failure. Go idiom: panic for misuse. FrameBus is local library (deterministic lifecycle), not network library (transient failures).

**Impact**: Fail-fast exposes bugs in tests (not production). Documents correct shutdown sequence. Re-evaluate if FrameBus becomes network-facing.

---

### ADR-006: Channel Ownership - No Close by FrameBus
**Decision**: FrameBus does NOT close subscriber channels during Close(). Subscribers own their channels and are responsible for closing them.

**Why**: Follows Go ownership principle ("creator owns lifecycle"). Prevents fatal double-close panics. Consistent with Go stdlib (`io.Pipe`, `context.Context`) and state-of-the-art libraries (NSQ, NATS).

**Impact**: Clear separation of concerns - FrameBus distributes frames, subscribers manage lifecycle. Explicit documentation added to `Subscribe()` godoc to prevent confusion. Goroutine leak risk is on the correct side (subscriber controls own lifecycle).

---

### ADR-007: Concurrent Fan-out
**Decision**: Use concurrent goroutine-based fan-out instead of sequential loop for frame distribution.

**Why**: Performance by design. Sequential fan-out has O(N) wall-clock latency (5μs for 10 subscribers, 50μs for 100). Concurrent fan-out is O(1) (~1.6μs regardless of subscriber count). FrameBus is highway-level infrastructure where performance always wins. Macro-level API simplicity allows micro-level optimization.

**Impact**: 3-31x speedup (depending on subscriber count). Ready for Multi-stream Orion 2.0 (100+ subscribers). Zero API changes. Fire-and-forget semantics aligned with non-blocking philosophy. Tests require async wait patterns (`time.Sleep()`). Async cache rebuild strengthens streaming semantics (Subscribe @ t takes effect @ t+1).

## References

- [ADR GitHub](https://adr.github.io/) - ADR methodology
- [Orion Design Manifesto](../../../../docs/DESIGN/MANIFESTO_DISENO.md)
- [C4 Model](../../../../docs/DESIGN/C4_MODEL.md)
