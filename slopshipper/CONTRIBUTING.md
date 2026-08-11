# Contributing

## Setup

Requirements: Go 1.26+ and [mise](https://mise.jdx.dev/).

```bash
git clone https://github.com/uinaf/slopshipper.git
cd slopshipper
mise trust
mise install
```

## Run locally

Build and invoke the development binary from the checkout:

```bash
mise run build
./bin/slopshipper version
```

The build task never installs an unversioned `slopshipper` into the active Go
toolchain or replaces a packaged CLI on `PATH`.

## Validate

Run the deterministic local gate before opening a pull request:

```bash
mise run verify
```

Coverage is enforced per production package: 80% minimum, with the state
machine and status contract held to 90%. The CLI child-process integration
surface has a separate floor. The gate also runs the race detector, installer
fixtures, release configuration checks, and the repo-local build.

## Pull requests

Use the PR template. Prefer small, verifiable slices. Squash-merge is the
default on `main`. Conventional Commits on `main` drive Semantic Release
(see [`docs/RELEASES.md`](docs/RELEASES.md)).

Control plane UI lives in `internal/serve` (Go `html/template` + embedded
`@uinaf/design` tokens). Keep it a read-only projector over sqlite; do not add
mutations that bypass the CLI state machine.

Agent-facing command schemas live with the binary in
`cmd/slopshipper/schema.go`; strict raw payload DTOs live in
`cmd/slopshipper/input.go`; state-location policy lives in
`cmd/slopshipper/storage.go`. When adding or changing a command, keep its
parser, runtime schema, JSON output, dry-run behavior, and child-process
contract in sync. Tests require every parsed command to appear in
`slopshipper schema`.
Keep the checked-in [agent CLI contract](docs/AGENT_INTERFACE.md) aligned with
observable behavior, but do not duplicate runtime schemas in prose.

## Releases

Publication uses the protected `release` Environment (shared Apple signing
secrets with other uinaf CLIs) and the `uinaf-releaser` GitHub App scoped to
`slopshipper` + `homebrew-tap`. Do not delete published tags to retry; re-run
the workflow on the tagged HEAD instead.

Skill publication uses a separate `skill-release` Environment with
`TESSL_TOKEN` (Tessl workspace `uinaf` publisher). GitHub Actions runs Tessl
plugin lint before merge, then `uinaf/tessl-publish-action` publishes the
manifest version and smokes a Codex install. Local skill lint additionally
requires Tessl CLI 0.94.0:

```bash
mise run skill:lint
```

Bump `skills/slopshipper/.tessl-plugin/plugin.json` when the skill should ship a
new immutable revision.

A scheduled `govulncheck` scan covers changes in the vulnerability database
without making the deterministic local gate depend on the network.

## Security

Do not open public issues for vulnerabilities. Use private vulnerability
reporting (see [SECURITY.md](SECURITY.md)).
