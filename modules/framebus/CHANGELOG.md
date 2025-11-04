# Changelog

All notable changes to the FrameBus module will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added (2025-11-04 - Quick Wins)
- Helper functions `CalculateDropRate()` and `CalculateSubscriberDropRate()` for easier stats interpretation
- ASCII diagram in README showing fan-out pattern with visual example
- Usage examples of helper functions in README and example code
- Comprehensive test coverage for helper functions (9 test cases)

### Changed (2025-11-04 - Architecture Refactor)
- **BREAKING**: Moved implementation to `internal/bus` package
- Created public API surface in `framebus.go` using type aliases
- Separated public interface from internal implementation
- All tests passing with new structure (28 tests, race-free)
- Examples updated and working
- Zero breaking changes for consumers (backward compatible)

### Technical Details
- Public API: `framebus.go` (74 lines)
- Implementation: `internal/bus/bus.go` (284 lines)
- Tests: `internal/bus/bus_test.go` (450 lines)
- Helpers: `helpers.go` + `helpers_test.go` (138 lines)

---

## [1.0.0] - 2025-11-04

### Added
- Initial implementation of FrameBus with channel-based subscriber pattern
- Non-blocking publish with drop policy (ADR-002)
- Comprehensive stats tracking (global + per-subscriber) (ADR-003)
- Full test suite with 13 unit tests
- Benchmark suite (Publish, Stats)
- Example: basic usage with simulated video processing
- Complete documentation:
  - CLAUDE.md (bounded context, API, anti-patterns)
  - README.md (overview, quick start)
  - doc.go (package documentation)
  - 3 ADRs (001-003)

### Design Decisions
- ADR-001: Channel-based Subscriber Pattern (vs interface-based)
- ADR-002: Non-blocking Publish with Drop Policy (vs queuing)
- ADR-003: Stats Tracking Design (collection only, no auto-logging)

### Thread Safety
- RWMutex for subscriber map
- Atomic operations for counters
- Safe concurrent Publish + Subscribe/Unsubscribe

### Performance
- Publish: ~1-2µs per subscriber
- Memory: Constant (bounded by channel buffers)
- No allocations in hot path (Publish)

## Notes

This is the initial release created during the FrameBus design session (2025-11-04).
The module follows the Orion 2.0 multi-module monorepo architecture and serves as
the reference implementation for future modules.

### Migration from Prototype

This module replaces `References/orion-prototipe/internal/framebus/bus.go` with
a cleaner bounded context:

**Removed from prototype:**
- `Start()` and `Stop()` methods (lifecycle not FrameBus responsibility)
- `StartStatsLogger()` method (observability not FrameBus responsibility)
- `InferenceWorker` interface coupling (subscribers are now just channels)

**Added in Orion 2.0:**
- Global stats (TotalPublished, TotalSent, TotalDropped)
- Better separation of concerns (collection vs interpretation)
- Channel-based API (more idiomatic Go)

## Future Roadmap

### v0.1.0 (Target: 2025-11-15)
- [ ] Property-based tests for invariants
- [ ] Integration test with real stream-capture module
- [ ] Performance optimization (reduce allocations)

