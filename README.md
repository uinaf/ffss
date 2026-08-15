# ffsstack

**ffsstack** — short for *flipflopslopstack* — is an agentic software factory,
not another \*stack. Deterministic state machines, observed evidence, and
adversarial review for shipping slop that survives contact with reality.

## CLIs

| CLI | What it is |
| --- | --- |
| [`slopmachine`](cli/slopmachine/) | The deterministic state machine: Go CLI + SQLite spine gating intake → release → build → verify → review → deliver → watch, with forge-verified evidence |
| [`slopguard`](cli/slopguard/) | Pre-ship second-model review closeout CLI |

## Skills

| Skill | What it does |
| --- | --- |
| [`slopmachine`](skills/slopmachine/) | Drive the slopmachine CLI through a governed run |
| [`slopguard`](skills/slopguard/) | Run one independent review through the slopguard CLI |
| [`slopcourier`](skills/slopcourier/) | Deliver finished work as a change request on the repo's forge, with the visual-evidence ladder |
| [`slopnanny`](skills/slopnanny/) | Babysit a delivered change request through review and CI to a settled outcome |
| [`slopspec`](skills/slopspec/) | Turn agreed work into durable tracker plans |
| [`slopscriber`](skills/slopscriber/) | Audit and rewrite repo docs and agent guidance |
| [`slopprep`](skills/slopprep/) | Make a repository agent-ready |

Reference bindings elsewhere in the family: slopzapper (forge review bot),
slopscouter (QA, planned), slopbench (evals), slopwake.

## Status

Under construction ([migration epic](https://github.com/uinaf/slopshipper/issues/50)).
Histories of the member repos were imported intact; releases, installers, and
the agent-plugin marketplace still ship from the legacy repos until the
corresponding migration milestones land. The root workflow is the only live
CI — it runs each member's own gate (`mise run verify` inside
`cli/slopshipper/` and `cli/slopguard/`); member release workflows return
with the release-migration milestone.

## License

MIT — see [LICENSE](LICENSE); members carry their own copies.
