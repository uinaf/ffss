# Contributing

## Setup

Requirements: Go 1.26+ and [mise](https://mise.jdx.dev/).

```bash
git clone https://github.com/uinaf/slopomatic.git
cd slopomatic
mise trust
mise install
mise run verify
```

## Pull requests

Use the PR template. Prefer small, verifiable slices. Squash-merge is the
default on `main`. Conventional Commits on `main` drive Semantic Release
(see [`docs/RELEASES.md`](docs/RELEASES.md)).

## Releases

Publication uses the protected `release` Environment (shared Apple signing
secrets with other uinaf CLIs) and the `uinaf-releaser` GitHub App scoped to
`slopomatic` + `homebrew-tap`. Do not delete published tags to retry; re-run
the workflow on the tagged HEAD instead.

Skill publication uses a separate `skill-release` Environment with
`TESSL_TOKEN` (Tessl workspace `uinaf` publisher). GitHub Actions runs Tessl
plugin lint via `uinaf/tessl-publish-action`, then publishes the
manifest version and smokes a Codex install. Locally:

```bash
mise run skill:lint
```

Bump `skills/slopomatic/.tessl-plugin/plugin.json` when the skill should ship a
new immutable revision.

## Security

Do not open public issues for vulnerabilities. Use private vulnerability
reporting (see [SECURITY.md](SECURITY.md)).
