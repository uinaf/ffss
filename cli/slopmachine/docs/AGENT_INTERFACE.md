# Agent CLI contract

`slopmachine` is the workflow authority. Agents provide structured commands;
the binary validates them, applies the state machine, and stores the resulting
event. Do not reproduce transition logic in a skill, prompt, or script.

The agent is not a trusted operator: the CLI validates strict JSON and
resource IDs at its boundary, then applies state transitions through revision
and guard checks. The dry-run guarantees, the `verify --cmd` shell rule, and
the repository-local state rules in this contract are that boundary's
enforcement points. Report suspected vulnerabilities through the repo's
[security policy](../../../SECURITY.md), never a public issue.

## Harness conformance

The protocol is harness-independent. Any agent that can run shell commands,
pipe JSON through stdin, and read stdout completes a full run using
`status --json --fields`, `schema`, and the executable commands `next_action`
returns: no bundled skill, no harness features. `next_action` never names a
harness capability; placeholders in angle brackets are fields for the caller
to fill. `scripts/test-conformance.sh` proves this bar with a skill-less
shell driver and runs inside `mise run verify`.

## Discover the live contract

Inspect runtime schemas before composing an unfamiliar command:

```bash
slopmachine schema
slopmachine schema --command intake
```

The schema describes accepted flags, raw input properties, required fields,
nested types, all status fields, `recommended_status_fields`, and the error
envelope. It is authoritative when prose and the installed binary differ.

Use a field mask to keep the control loop compact:

```bash
slopmachine status --json \
  --fields state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivered_units,delivery_mode,blocker,decision_question,evidence_verification
```

Run only a command named in `allowed_commands`, satisfy
`required_evidence`, and re-read status after plain output or an error. A
successful JSON mutation already returns the resulting status document.

## Send structured input through stdin

Every mutation accepts a strict raw JSON object through `--input PATH`. Use
`--input -` for stdin:

```bash
slopmachine intake --input - --dry-run --json <<'JSON'
{
  "run": "demo",
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
```

Raw input cannot be mixed with convenience flags such as `--run`; the payload
carries those values. Human-oriented convenience inputs also accept stdin:

```bash
slopmachine intake --file - --run demo
slopmachine review --evidence - --run demo
slopmachine deliver --evidence - --run demo
```

`next_action` uses `--file -` and `--evidence -` for structured payloads. Do
not create `*.evidence.json` or other command payloads in the repository.

When `-` selects stdin, pipe or redirect JSON. An interactive terminal fails
immediately with a pointer to `slopmachine schema` instead of waiting for EOF.

Raw JSON is fail-closed: unknown, duplicate, and `null` fields are rejected.
Run and unit IDs are at most 64 bytes, start with an ASCII letter or digit, and
then contain only ASCII letters, digits, `.`, `_`, or `-`. Put prose in title,
question, answer, or reason fields.

## Validate, then apply

Add `--dry-run --json` to any mutation before applying it:

```bash
slopmachine release --revision 1 --run demo --dry-run --json
```

A valid projection contains `dry_run: true` and `validated_command`. Confirm
the projected state and `next_action`, then repeat the same command without
`--dry-run`. Dry runs open existing state read-only and never create, migrate,
or update the database. `verify --cmd --dry-run` also skips shell execution.
Because its exit code is unknown, that response keeps the current state and
sets `outcome_undetermined: true`; validate the command, not a guessed
transition outcome.

A real `verify --cmd` always records one deterministic digest from separately
hashed stdout and stderr streams. The final SHA-256 hashes the labeled text
`stdout=sha256:<hex>\nstderr=sha256:<hex>`. JSON mode streams both child
channels to stderr so stdout remains machine-readable; plain mode preserves
the child's stdout and stderr.
A non-dry `verify --cmd` intentionally invokes the local shell; never
construct it from untrusted text.

## Observe external signals

Delivery does not settle a unit.

- `slopmachine watch --once` observes every delivered unit's change request on
  the forge and records the signals itself; passes are idempotent, and
  `--interval SECONDS` polls with a bounded iteration count.
- `slopmachine observe` records one signal manually: `merged` settles the
  unit; `checks_failed`, `review_feedback`, and `head_moved` return it to the
  build loop with the cause recorded, and later units keep building while
  earlier ones wait (`AWAITING_SIGNALS`).
- `observe.unit` is required only when several units are delivered; the
  `delivered_units` status field lists the candidates.
- Exit 7 from watch means the final pass left at least one delivered unit
  unobserved (auth, rate limit, transient failure, or a missing change
  request); failures recovered by a later interval pass do not affect the exit
  status.
- Stdout still carries the watch document (observations already recorded plus
  an `error_kind` field), not the ok:false error envelope.
- Feedback identity has two accepted limits under the bounded thread sample: a
  thread reopened without a new comment is not re-detected (any new comment
  is), and with more than ten unresolved threads a sample shift may
  conservatively re-trigger one extra rework rather than miss a signal.

## Verified evidence

Status states the mode plainly in `evidence_verification`: `observed` when a
registered repo profile binds a forge kind, `recorded` otherwise. In observed
mode the binary checks evidence before accepting it and stamps a
`verification` field into it; never supply that field yourself.

- `deliver` for a change-request delivery observes the live change request:
  a missing one fails (exit 3), a head that differs from `commit_sha` fails
  (exit 3), and a verified delivery without `commit_sha` adopts the observed
  head so watch can detect later movement. Direct-trunk deliveries stay
  recorded input.
- `review` evidence from a reviewer mapped with `repo --forge-reviewer`
  requires a change-request URL as `artifact_ref` and at least one submitted
  review by the mapped login on that change request (exit 3 otherwise).
  Unmapped reviewers keep recorded-input behavior.
- When the forge is unreachable the evidence is unprovable, not accepted:
  exit 7 with `error_kind` `observation_auth`, `observation_rate_limit`, or
  `observation_transient`. Retry, or record an explicit bypass with
  `--unverified --reason TEXT` (raw input: `"unverified": true` plus
  `"unverified_reason"`); the bypass is itself recorded in the evidence as
  `verification: "overridden"`. `--unverified` is rejected when nothing
  would be verified.

## Record telemetry

Transitions accept optional recorded telemetry (`--telemetry PATH|-` or a
`telemetry` object in `--input` payloads): `duration_ms`, `tokens`,
`cost_cents`, and `route` (venue, harness, role→model map). Record real
numbers only; omit what was not measured. `verify --cmd` measures its own
wall clock. Totals appear in status as `total_duration_ms`,
`total_tokens`, `total_cost_cents`, and `telemetry_events`.

## Treat SQLite as canonical state

By default, state is stored outside the repository:

- `$XDG_DATA_HOME/slopmachine/slopmachine.sqlite` when `XDG_DATA_HOME` is set.
- `~/.local/share/slopmachine/slopmachine.sqlite` otherwise.

Accepted review, verification, and delivery evidence is canonicalized into the
SQLite event log. JSON supplied through stdin or a file is command transport,
not durable state, and can be discarded after the command succeeds.

Set `SLOPMACHINE_DB` when a sandbox needs a different writable location:

```bash
sandbox_dir="$(mktemp -d)"
export SLOPMACHINE_DB="$sandbox_dir/slopmachine.sqlite"
```

- There is no automatic repository-local fallback.
- If a constrained environment requires repository-local state, explicitly
  point `SLOPMACHINE_DB` at `.slopmachine/slopmachine.sqlite`. Relative values
  resolve from the Git worktree root.
- The CLI refuses repository-local state unless the database, `-wal`, and
  `-shm` paths are untracked and ignored.
- A `/.slopmachine/` entry in `$(git rev-parse --git-path info/exclude)`
  satisfies that boundary without a shared `.gitignore` change.

Inspect resolution without creating or migrating state:

```bash
slopmachine storage --json
```

The command reports the absolute path, resolution source, scope, existence,
and Git safety. The CLI never edits ignore files and never silently falls back
to another database.

## Handle structured output and errors

Place `--json` before or after a command. Successful mutations return status;
failures return a stable envelope:

```json
{
  "schema_version": 1,
  "ok": false,
  "error": {
    "kind": "invalid_input",
    "message": "...",
    "exit_code": 2
  }
}
```

Use `error.kind` for control flow and `message` for diagnostics. Fix malformed
input, or re-read status after an illegal transition, unmet guard, ambiguous
run, revision conflict, or verification failure. Never bypass the state
machine because a suggested transition failed.

- `run_exists` identifies a caller-selected run ID collision.
- `invalid_state_config` identifies a malformed XDG or relative database
  selection.
- `state_unavailable` identifies a resolved state location that cannot be
  prepared, such as an unwritable directory; its message names the resolved
  path and the recovery is a writable `SLOPMACHINE_DB`.
- All three exit 2 and are recoverable by changing caller input.
