# Agent Guide

`slopguard` is a Go CLI for structured, independent code review. Its agent
skill lives in [skills/slopguard](../../skills/slopguard/SKILL.md).

## Commands

`mise run verify` is the deterministic gate; [mise.toml](mise.toml) owns its
steps and the Go parallelism caps. Setup, release, and live provider checks:
[Contributing](CONTRIBUTING.md).

## Boundaries

- The CLI reviews and reports. Editing source, running tests, committing,
  pushing, and nested reviews belong to the caller.
- Keep provider execution, protocol validation, review policy, configuration,
  and report rendering in separate packages.
- Parse provider output at the boundary and fail closed on ambiguous or invalid
  results.
- The skill stays thin and matches the released CLI contract.
- JSON output is a stable machine contract; terminal output is a separate
  renderer.
