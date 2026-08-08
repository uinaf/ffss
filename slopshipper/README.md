# slopomatic

Deterministic and structured approach to slop cannoning.

```text
make a plan  →  /slopomatic  →  clarify with the human  →  human releases  →  machine runs
```

`slopomatic` is a Go CLI and thin agent skill: an evidence-gated loop×graph state
machine so coding agents build, verify, and deliver work one auditable
transition at a time — mindful token spend, without sacrificing quality.

Canonical plan: [#1](https://github.com/uinaf/slopomatic/issues/1).

## Install

```bash
go install github.com/uinaf/slopomatic/cmd/slopomatic@latest
slopomatic version
```

From a checkout:

```bash
go test ./...
go install ./cmd/slopomatic
```

State lives in `$XDG_DATA_HOME/slopomatic/slopomatic.sqlite` (default
`~/.local/share/slopomatic/slopomatic.sqlite`). Override with `SLOPOMATIC_DB`.

## Quick path

```bash
slopomatic init --run demo
slopomatic intake --file examples/intake.example.json --run demo
slopomatic status --json --run demo   # note intake_revision + next_action
slopomatic release --revision "$INTAKE_REVISION" --run demo
slopomatic build --run demo
slopomatic verify --cmd 'go test ./...' --run demo
slopomatic review --evidence examples/review.example.json --run demo
slopomatic deliver --evidence examples/deliver.example.json --run demo
slopomatic status --json --run demo
```

Default `status` is one compact line with `next_action`. Use `--json` for agents.

Park a human question with `slopomatic ask --question "…"`, then
`slopomatic decide --answer "…"`. Multi-unit intake:
[`examples/intake.multi.example.json`](examples/intake.multi.example.json).

CI / local gate: `go test ./...` (includes multi-unit ask→decide→pr-hold CLI e2e
and fail-closed release/evidence/`--run` checks).

## Agent skill

Bundled at [`skills/slopomatic/SKILL.md`](skills/slopomatic/SKILL.md). Manual
invocation only. The skill drives the CLI; it does not reimplement the machine.

## Companion tools

- [`autoreview`](https://github.com/uinaf/autoreview) — preferred portable independent review CLI
- On Cursor: `/review-bugbot` as a cheap local review option (confirm before use)

## License

[MIT](LICENSE)
