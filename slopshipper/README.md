# slopinator

Deterministic and structured approach to slop cannoning.

```text
make a plan  →  /slopinator  →  clarify with the human  →  human releases  →  machine runs
```

`slopinator` is a Go CLI and thin agent skill: an evidence-gated loop×graph state
machine so coding agents build, verify, and deliver work one auditable
transition at a time — mindful token spend, without sacrificing quality.

Canonical plan: [#1](https://github.com/uinaf/slopinator/issues/1).

## Install

```bash
go install github.com/uinaf/slopinator/cmd/slopinator@latest
slopinator version
```

From a checkout:

```bash
go test ./...
go install ./cmd/slopinator
```

State lives in `$XDG_DATA_HOME/slopinator/slopinator.sqlite` (default
`~/.local/share/slopinator/slopinator.sqlite`). Override with `SLOPINATOR_DB`.

## Quick path

```bash
slopinator init --run demo
slopinator intake --file examples/intake.example.json --run demo
slopinator status --json --run demo   # note intake_revision + next_action
slopinator release --revision "$INTAKE_REVISION" --run demo
slopinator build --run demo
slopinator verify --cmd 'go test ./...' --run demo
slopinator review --evidence examples/review.example.json --run demo
slopinator deliver --evidence examples/deliver.example.json --run demo
slopinator status --json --run demo
```

Default `status` is one compact line with `next_action`. Use `--json` for agents.

Park a human question with `slopinator ask --question "…"`, then
`slopinator decide --answer "…"`. Multi-unit intake:
[`examples/intake.multi.example.json`](examples/intake.multi.example.json).

CI / local gate: `go test ./...` (includes multi-unit ask→decide→pr-hold CLI e2e
and fail-closed release/evidence/`--run` checks).

## Agent skill

Bundled at [`skills/slopinator/SKILL.md`](skills/slopinator/SKILL.md). Manual
invocation only. The skill drives the CLI; it does not reimplement the machine.

## Companion tools

- [`autoreview`](https://github.com/uinaf/autoreview) — preferred portable independent review CLI
- On Cursor: `/review-bugbot` as a cheap local review option (confirm before use)

## License

[MIT](LICENSE)
