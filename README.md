# ffsstack

**ffsstack** — short for *flipflopslopstack* — is an agentic software factory,
not another \*stack. Deterministic state machines, observed evidence, and
adversarial review for shipping slop that survives contact with reality.

## Members

| Member | What it is |
| --- | --- |
| [`cli/slopshipper`](cli/slopshipper/) | The spine: Go CLI + SQLite state machine that gates intake → release → build → verify → review → deliver → watch, with forge-verified evidence |
| [`cli/slopguard`](cli/slopguard/) | Pre-ship second-model review closeout CLI (formerly autoreview) |
| [`skills/slopspec`](skills/slopspec/) | Turn agreed work into durable tracker plans |
| [`skills/slopscriber`](skills/slopscriber/) | Audit and rewrite repo docs and agent guidance |
| [`skills/slopprep`](skills/slopprep/) | Make a repository agent-ready |
| skills/slopcourier | Forge-neutral delivery skill — planned ([uinaf/skills#57](https://github.com/uinaf/skills/issues/57)) |

Reference bindings elsewhere in the family: slopzapper (forge review bot),
slopscouter (QA, planned), slopbench (evals, planned), slopwake.

## Status

Under construction ([migration epic](https://github.com/uinaf/slopshipper/issues/50)).
Histories of the member repos were imported intact; releases, installers, and
the agent-plugin marketplace still ship from the legacy repos until the
corresponding migration milestones land. Each member keeps its own gates:
`mise run verify` inside `cli/slopshipper/` and `cli/slopguard/`.

## License

MIT — see [LICENSE](LICENSE); members carry their own copies.
