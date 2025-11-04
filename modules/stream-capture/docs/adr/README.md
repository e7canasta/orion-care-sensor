# Architecture Decision Records (ADRs)

This directory contains Architecture Decision Records for the stream-capture module.

## Index

| ADR | Title | Status | Date |
|-----|-------|--------|------|
| [000](000-template.md) | ADR Template | - | - |
| [001](001-tcp-only-transport.md) | TCP-Only Transport for RTSP | Accepted | 2025-01-04 |

## Future ADRs (to be created)

The following decision records should be migrated from `ARCHITECTURE.md` when time permits:

- **ADR-002**: Atomic Statistics (Lock-Free Telemetry)
- **ADR-003**: Double-Close Protection (Atomic Bool)
- **ADR-004**: RGB Format Lock (VAAPI Pipeline)
- **ADR-005**: Warmup Fail-Fast Pattern
- **ADR-006**: Non-Blocking Channels with Drop Policy

## Format

We use the [MADR 3.0.0](https://adr.github.io/madr/) template (Markdown Any Decision Records).

## Workflow

1. Copy `000-template.md` to new ADR file (e.g., `002-new-decision.md`)
2. Fill in Context, Decision Drivers, Options, and Decision Outcome
3. Submit for review
4. Update status after team discussion
5. Update this README index

---

**Co-authored-by:** Gaby de Visiona <noreply@visiona.app>
