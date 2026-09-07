# Agent Guide

`slopguard` is a Go-only CLI and standalone agent skill for structured,
independent code review.

## Commands

- `mise run verify`
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `go build ./cmd/slopguard`
- `go run ./cmd/slopguard --version`

## Boundaries

- Keep the runtime entirely in Go: no shell or other-language helpers or
  wrappers around the binary.
- Runtime dependencies are Git and the selected review harness.
- The CLI reviews and reports; it never edits reviewed source, runs tests,
  commits, pushes, or invokes nested review workflows.
- Keep provider execution, protocol validation, review policy, configuration,
  and report rendering in separate packages.
- Parse provider output at the boundary and fail closed on ambiguous or invalid
  results.
- Keep the bundled skill thin and aligned with the released CLI contract.
- JSON output is a stable machine contract; terminal output is a separate
  renderer.
