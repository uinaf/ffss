# Contributing

## Setup

Requirements: Go 1.26+ and [mise](https://mise.jdx.dev/).

```bash
git clone https://github.com/uinaf/slopomatic.git
cd slopomatic
mise trust
mise install
```

## Run locally

Build and invoke the development binary from the checkout:

```bash
mise run build
./bin/slopomatic version
```

The build task never installs an unversioned `slopomatic` into the active Go
toolchain or replaces a packaged CLI on `PATH`.

## Remove a legacy Go-installed binary

An older `go install` workflow may have left a development binary ahead of a
packaged release on `PATH`. List every copy before removing anything:

```bash
type -a slopomatic
```

After confirming `command -v slopomatic` points into a Go or mise installation,
remove that exact copy and refresh the shell command cache:

```bash
legacy_slopomatic="$(command -v slopomatic)"
case "$legacy_slopomatic" in
  */mise/installs/go/*/bin/slopomatic|*/go/bin/slopomatic)
    rm -- "$legacy_slopomatic"
    hash -r
    ;;
  *) printf 'Refusing to remove non-Go path: %s\n' "$legacy_slopomatic" >&2 ;;
esac
```

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

## Releases

Publication uses the protected `release` Environment (shared Apple signing
secrets with other uinaf CLIs) and the `uinaf-releaser` GitHub App scoped to
`slopomatic` + `homebrew-tap`. Do not delete published tags to retry; re-run
the workflow on the tagged HEAD instead.

Skill publication uses a separate `skill-release` Environment with
`TESSL_TOKEN` (Tessl workspace `uinaf` publisher). GitHub Actions runs Tessl
plugin lint before merge, then `uinaf/tessl-publish-action` publishes the
manifest version and smokes a Codex install. Local skill lint additionally
requires Tessl CLI 0.94.0:

```bash
mise run skill:lint
```

Bump `skills/slopomatic/.tessl-plugin/plugin.json` when the skill should ship a
new immutable revision.

A scheduled `govulncheck` scan covers changes in the vulnerability database
without making the deterministic local gate depend on the network.

## Security

Do not open public issues for vulnerabilities. Use private vulnerability
reporting (see [SECURITY.md](SECURITY.md)).
