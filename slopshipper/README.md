![slopomatic — deterministic and structured approach to slop cannoning.](https://uinaf.dev/og/banner/slopomatic.png)

# slopomatic

`slopomatic` is a Go CLI and thin agent skill for evidence-gated loop×graph
execution. Coding agents build, verify, and deliver work one auditable
transition at a time — mindful token spend, without sacrificing quality.

```text
make a plan  →  /slopomatic  →  clarify with the human  →  human releases  →  machine runs
```

## Install

On macOS, install the signed CLI from the `uinaf/tap` Homebrew tap:

```bash
brew install --cask uinaf/tap/slopomatic
slopomatic version
```

On Linux amd64 or arm64, install the latest released binary without Go,
Homebrew, `jq`, or `sudo`:

```bash
curl --proto '=https' --tlsv1.2 -fsSL \
  https://raw.githubusercontent.com/uinaf/slopomatic/main/install.sh | sh
~/.local/bin/slopomatic version
```

Pass `--version "$TAG"` or `--dest /chosen/bin` after `sh -s --` to pin a
release or override `${HOME}/.local/bin`. The installer downloads the archive
and `checksums.txt` from the same GitHub Release, verifies the exact SHA-256,
and atomically replaces the destination binary. See
[Release verification](docs/RELEASES.md#linux-installer-trust-boundary) for the
HTTPS trust boundary and independent Cosign and GitHub attestation checks.

Consumers that prefer Go tooling can instead install with Go 1.26 or newer:

```bash
go install github.com/uinaf/slopomatic/cmd/slopomatic@latest
slopomatic version
```

State lives in `$XDG_DATA_HOME/slopomatic/slopomatic.sqlite` (default
`~/.local/share/slopomatic/slopomatic.sqlite`). Override with `SLOPOMATIC_DB`.

## Quick use

```bash
slopomatic init --run demo
slopomatic intake --file - --run demo <<'JSON'
{
  "delivery_mode": "pr-hold",
  "review_consent": "autoreview",
  "series_bound": 1,
  "units": [{"id": "u1", "title": "Ship the change", "blockers": []}]
}
JSON
slopomatic status --json --run demo   # note intake_revision and next_action
slopomatic release --revision N --run demo
slopomatic build --run demo
slopomatic verify --cmd 'go test ./...' --run demo
slopomatic review --evidence - --run demo <<'JSON'
{"reviewer":"autoreview","verdict":"clean","artifact_ref":"autoreview://local"}
JSON
slopomatic deliver --evidence - --run demo <<'JSON'
{"delivery_mode":"pr-hold","pr_url":"https://github.com/example/repo/pull/1"}
JSON
slopomatic status --json --run demo
```

Replace `N` with the exact `intake_revision` printed by status. Default
`status` is one compact line with an executable `next_action`; use `--json` for
agents. Every command has focused help, for example `slopomatic review --help`.

Park a human question with `slopomatic ask --question "…"`, then
`slopomatic decide --answer "…"`. Record an external blocker with
`slopomatic block --reason "…"`; after the human confirms recovery, resume with
`slopomatic retry --reason "…"`. Multi-unit intake:
[`examples/intake.multi.example.json`](examples/intake.multi.example.json).

## Control plane

Project the same sqlite store in a loopback browser UI (Go templates +
`@uinaf/design` tables). Read-only — not a second state authority.

```bash
slopomatic serve                 # http://127.0.0.1:7780
slopomatic serve --addr 127.0.0.1:9000
```

Smoke: start `serve`, open `/`, click a run, confirm units + evidence timeline
match `slopomatic status --json --run <id>`.

## Agent skill

The bundled [agent skill](skills/slopomatic/SKILL.md) drives the CLI. It does
not contain a second runtime. Manual invocation only.

## Companion tools

- [`autoreview`](https://github.com/uinaf/autoreview) — preferred portable independent review CLI
- On Cursor: `/review-bugbot` as a cheap local review option (confirm before use)

## Documentation

- [Release artifacts and verification](docs/RELEASES.md)
- [Contributing](CONTRIBUTING.md) — setup, gates, and pull-request expectations

## License

[MIT](LICENSE)
