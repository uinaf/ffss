![slopmachine — deterministic and structured approach to slop cannoning.](https://uinaf.dev/og/banner/slopmachine.png)

# slopmachine

`slopmachine` turns an approved implementation plan into a resumable,
evidence-gated workflow for coding agents. Humans release the work; the CLI
then enforces each build, verification, review, and delivery transition.

```text
plan  →  /slopmachine  →  clarify  →  human releases  →  machine runs
```

- **Human-controlled:** scope, release, required reviewers, and recovery stay
  explicit.
- **Deterministic:** a Go state machine decides what can happen next.
- **Auditable:** structured evidence and an SQLite event log make every
  transition inspectable.

## Install

### macOS

Install the signed CLI from the `uinaf/tap` Homebrew tap:

```bash
brew install --cask uinaf/tap/slopmachine
slopmachine version
```

### Linux

Install the latest amd64 or arm64 release without Go, Homebrew, `jq`, or
`sudo`:

```bash
curl --proto '=https' --tlsv1.2 -fsSL \
  https://raw.githubusercontent.com/uinaf/ffss/main/cli/slopmachine/install.sh | sh
~/.local/bin/slopmachine version
```

- To pin a release or choose an install directory, pass `--version "$TAG"`
  or `--dest /chosen/bin` after `sh -s --`.
- The installer verifies the archive against the release checksum before
  atomically replacing the binary.
- Installer-managed binaries upgrade in place with `slopmachine selfupdate`
  (`--check` probes without touching the binary); brew-managed installs
  upgrade through `brew upgrade` instead.
- See [Release verification](docs/RELEASES.md#linux-installer-trust-boundary)
  for independent Cosign and GitHub attestation checks.

The installer finishes with a non-fatal `PATH` check: it warns when `PATH`
resolves a different `slopmachine` than the install destination and names
the winning copy.

To build from source without replacing the packaged CLI on `PATH`, see
[Run locally](CONTRIBUTING.md#run-locally).

- State is stored at `$XDG_DATA_HOME/slopmachine/slopmachine.sqlite`, or
  `~/.local/share/slopmachine/slopmachine.sqlite` when `XDG_DATA_HOME` is
  unset. The CLI creates the state directory and database on first use with
  private permissions.
- Set `SLOPMACHINE_DB` to use a different writable database path; relative
  overrides resolve from the Git worktree root.
- Inspect the selected location with `slopmachine storage --json`;
  repository-local databases are rejected unless the database and its SQLite
  sidecars are untracked and ignored.

## Quick start

### 1. Define and release the work

```bash
slopmachine init --run demo
slopmachine intake --file - --run demo <<'JSON'
{
  "delivery_mode": "pr-hold",
  "required_reviewers": ["slopguard"],
  "risk_tier": "low",
  "series_bound": 1,
  "units": [
    {"id": "u1", "title": "Ship the change", "blockers": [],
     "acceptance_criteria": ["tests prove the agreed behavior"]}
  ]
}
JSON
slopmachine status --json --run demo
slopmachine release --revision N --run demo
```

Replace `N` with the exact `intake_revision` returned by `status`. Release is
the human approval boundary: the machine loop cannot begin without it.

### 2. Run the machine loop

```bash
slopmachine build --run demo
slopmachine verify --cmd 'go test ./...' --run demo
slopmachine review --evidence - --run demo <<'JSON'
{"reviewer":"slopguard","verdict":"clean","artifact_ref":"slopguard://local"}
JSON
slopmachine deliver --evidence - --run demo <<'JSON'
{"delivery_mode":"pr-hold","pr_url":"https://github.com/example/repo/pull/1"}
JSON
slopmachine observe --signal merged --run demo
slopmachine status --json --run demo
```

Delivery opens the change request; the unit settles only when an observed
signal closes it. `checks_failed` and `review_feedback` return the unit to
the build loop while later units keep building.

`status` is the compass. Its default output is one compact line with an
executable `next_action`; `--json` exposes the full contract for agents. Check
it after every transition instead of guessing the next command.

## Agent interface

Agents should discover the live schema, keep status reads narrow, and validate
each mutation before applying it:

```bash
slopmachine schema --command intake
slopmachine status --json --fields state,next_action,allowed_commands,required_evidence
slopmachine release --revision N --run demo --dry-run --json
```

Pass raw command payloads with `--input -` and convenience evidence with
`--evidence -`; do not leave transport JSON in the repository. See the
[agent CLI contract](docs/AGENT_INTERFACE.md) for structured input and output,
dry runs, state storage, sandbox overrides, and error recovery.

For multi-unit intake, start with
[`examples/intake.multi.example.json`](examples/intake.multi.example.json).
Every command also has focused help, such as `slopmachine review --help`.

## Common operations

| Need | Command |
| --- | --- |
| Inspect a run or find its next action | `slopmachine status [--json]` |
| Discover commands and raw payloads | `slopmachine schema [--command NAME]` |
| Inspect state location and Git safety | `slopmachine storage --json` |
| Validate a mutation without applying it | `slopmachine COMMAND --dry-run --json` |
| Pause for a human answer | `slopmachine ask --question "…"` |
| Record the answer | `slopmachine decide --answer "…"` |
| Record an external blocker | `slopmachine block --reason "…"` |
| Resume after confirmed recovery | `slopmachine retry --reason "…"` |
| Re-enter the build loop after findings | `slopmachine rework` |
| Record a forge signal for a delivered unit | `slopmachine observe --signal merged\|checks_failed\|review_feedback\|head_moved` |
| Observe delivered units on the forge automatically | `slopmachine watch [--once \| --interval SECONDS]` |
| Inspect or declare the repo profile | `slopmachine repo [show\|register\|update\|unregister]` |
| Record transition telemetry | `slopmachine COMMAND --telemetry tel.json` |
| Inspect all runs in a browser | `slopmachine serve` |

Commands that act on a run accept `--run ID`. When the repository has exactly
one open run, the CLI selects it automatically.

## Repo profiles

A repository can declare its profile: role bindings (review, qa, venue,
memory) plus policy (forge kind, trust tier, canonical verify command,
default delivery mode, recorded readiness verdict). Declaration over
detection: the binary never probes for tools.

```bash
slopmachine reviewers --add slopzapper
slopmachine repo register \
  --forge github --trust low \
  --verify-cmd 'mise run verify' --delivery pr-hold \
  --bind 'review=slopguard,review=slopzapper' \
  --forge-reviewer 'slopzapper=slopzapper'
```

A registered repo drives defaults and gates; unregistered repositories keep
profile-less behavior.

- New runs inherit the delivery mode.
- `next_action` names the canonical verify command verbatim.
- Release fails closed when a required reviewer holds no review binding.

A forge-bound profile also moves evidence from narrated to observed (status
states the mode in `evidence_verification`):

- Deliver evidence must name a change request that exists and matches the
  delivered head.
- Review evidence from a `--forge-reviewer`-mapped identity must be
  corroborated by a submitted review from that login on the referenced
  change request.
- When the forge is unreachable the command exits 7 rather than trusting the
  claim; `--unverified --reason TEXT` records an explicit, audited bypass.

## Telemetry

- Every transition can carry recorded telemetry (wall-clock duration,
  estimated tokens and cost, and the route actually used: venue, harness,
  role→model map) via `--telemetry PATH|-` or a `telemetry` object in raw
  `--input` payloads.
- `verify --cmd` measures its own duration.
- Absent telemetry is always valid; wrong shapes fail closed.
- `status --json` exposes per-run totals (`total_duration_ms`,
  `total_tokens`, `total_cost_cents`, `telemetry_events`) and `serve` shows
  them per run.
- This is recorded input for the routing ledger; the machine never enforces
  spend and never handles model keys.

## Browser view

Project the same SQLite state in a read-only local browser view:

```bash
slopmachine serve                 # http://127.0.0.1:7780
slopmachine serve --addr 127.0.0.1:9000
```

The browser is a projector, not a second state authority; workflow changes
still go through the CLI.

## Agent skill

The family [agent skill](../../skills/slopmachine/SKILL.md) teaches agents to drive
the installed CLI and obey `next_action`. It is intentionally thin: the binary
owns the state machine, schemas, and store.

Independent review remains a companion step.

- Reviewer identities are registered, not hardcoded: `slopguard` and
  `bugbot` are built in, and `slopmachine reviewers --add NAME` registers
  others (a hosted bot such as slopzapper, a CI reviewer, a QA provider).
- The intake's `required_reviewers` selects among them; run the matching
  installed tool, such as the
  [`slopguard`](https://github.com/uinaf/ffss/tree/main/cli/slopguard) CLI
  or Cursor's `/review-bugbot`, when the intake requires it.

## Documentation

- [Agent CLI contract](docs/AGENT_INTERFACE.md)
- [Release artifacts and verification](docs/RELEASES.md)
- [Contributing](CONTRIBUTING.md)
- [Security](../../SECURITY.md)

## License

[MIT](LICENSE)
