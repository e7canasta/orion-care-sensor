# Repository Guidelines

## Project Structure & Module Organization
- Core runtime lives in `framebus.go`, with opt-in helpers in `helpers.go` and private concurrency primitives under `internal/bus`.
- Public-facing docs, diagrams, and rationale sit in `docs/`, `ARCHITECTURE.md`, and `C4_MODEL.md`; keep them aligned with implementation changes.
- Example integrations reside in `examples/basic`, which compiles against this module and demonstrates subscriber drop behavior.
- `bin/` is reserved for locally compiled binaries; avoid committing artifacts.

## Build, Test, and Development Commands
- `go build ./...` compiles the module with Go 1.23 toolchain; run it before sending a PR.
- `go test ./...` executes unit tests in `helpers_test.go` and `internal/bus/bus_test.go`.
- `go test ./... -cover` prints package coverage; target 90%+ for new logic affecting the bus contract.
- `go vet ./...` surfaces common correctness pitfalls; treat warnings as failures.
- `go run ./examples/basic` runs the live demo for manual smoke verification of publish/subscribe flow.

## Coding Style & Naming Conventions
- Format with `gofmt`; run `go fmt ./...` or rely on your editor’s format-on-save.
- Follow Go naming: exported types and functions use PascalCase (`Bus`, `Subscribe`); internal helpers stay lowerCamelCase.
- Keep files focused: new transports or stats helpers go beside related types rather than in `framebus.go`.
- Prefer table-driven tests and explicit timeouts when asserting concurrent behavior.

## Testing Guidelines
- Place tests in the same package as the code, using `*_test.go` files mirroring the source filename.
- When adding concurrency logic, use `t.Parallel()` and deterministic timers (e.g., `time.AfterFunc`) to curb flakiness.
- Capture coverage for both happy paths and drop-policy branches; leverage existing stats helpers to assert totals.
- Record any new manual verification steps in `docs/proposals` when behavior materially changes.

## Commit & Pull Request Guidelines
- Follow the conventional commit style already in history: `type(scope): summary`, with ≤72 character subject lines.
- Reference related issues or ADRs in the body and note any API or configuration impacts.
- PRs must include: a clear summary, test evidence (`go test` output or coverage delta), and doc updates when architecture shifts.
- Request review from a module maintainer before merging; avoid force-pushing after review unless you coordinate.
