# Agent CLI contract

`slopshipper` is the workflow authority. Agents provide structured commands;
the binary validates them, applies the state machine, and stores the resulting
event. Do not reproduce transition logic in a skill, prompt, or script.

## Discover the live contract

Inspect runtime schemas before composing an unfamiliar command:

```bash
slopshipper schema
slopshipper schema --command intake
```

The schema describes accepted flags, raw input properties, required fields,
nested types, all status fields, `recommended_status_fields`, and the error
envelope. It is authoritative when prose and the installed binary differ.

Use a field mask to keep the control loop compact:

```bash
slopshipper status --json \
  --fields state,run_id,next_action,allowed_commands,required_evidence,intake_revision,required_reviewers,completed_reviewers,delivery_mode,blocker,decision_question
```

Run only a command named in `allowed_commands`, satisfy
`required_evidence`, and re-read status after plain output or an error. A
successful JSON mutation already returns the resulting status document.

## Send structured input through stdin

Every mutation accepts a strict raw JSON object through `--input PATH`. Use
`--input -` for stdin:

```bash
slopshipper intake --input - --dry-run --json <<'JSON'
{
  "run": "demo",
  "delivery_mode": "pr-hold",
  "required_reviewers": ["autoreview"],
  "series_bound": 1,
  "units": [
    {"id": "u1", "title": "Ship the change", "blockers": []}
  ]
}
JSON
```

Raw input cannot be mixed with convenience flags such as `--run`; the payload
carries those values. Human-oriented convenience inputs also accept stdin:

```bash
slopshipper intake --file - --run demo
slopshipper review --evidence - --run demo
slopshipper deliver --evidence - --run demo
```

`next_action` uses `--file -` and `--evidence -` for structured payloads. Do
not create `*.evidence.json` or other command payloads in the repository.

When `-` selects stdin, pipe or redirect JSON. An interactive terminal fails
immediately with a pointer to `slopshipper schema` instead of waiting for EOF.

Raw JSON is fail-closed: unknown, duplicate, and `null` fields are rejected.
Run and unit IDs are at most 64 bytes, start with an ASCII letter or digit, and
then contain only ASCII letters, digits, `.`, `_`, or `-`. Put prose in title,
question, answer, or reason fields.

## Validate, then apply

Add `--dry-run --json` to any mutation before applying it:

```bash
slopshipper release --revision 1 --run demo --dry-run --json
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

## Treat SQLite as canonical state

By default, state is stored outside the repository:

- `$XDG_DATA_HOME/slopshipper/slopshipper.sqlite` when `XDG_DATA_HOME` is set.
- `~/.local/share/slopshipper/slopshipper.sqlite` otherwise.

Accepted review, verification, and delivery evidence is canonicalized into the
SQLite event log. JSON supplied through stdin or a file is command transport,
not durable state, and can be discarded after the command succeeds.

Set `SLOPSHIPPER_DB` when a sandbox needs a different writable location:

```bash
sandbox_dir="$(mktemp -d)"
export SLOPSHIPPER_DB="$sandbox_dir/slopshipper.sqlite"
```

There is no automatic repository-local fallback. If a constrained environment
requires repository-local state, explicitly point `SLOPSHIPPER_DB` at
`.slopshipper/slopshipper.sqlite`. Relative values resolve from the Git worktree
root. The CLI refuses repository-local state unless the database, `-wal`, and
`-shm` paths are untracked and ignored. A `/.slopshipper/` entry in
`$(git rev-parse --git-path info/exclude)` satisfies that boundary without a
shared `.gitignore` change.

Inspect resolution without creating or migrating state:

```bash
slopshipper storage --json
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

`run_exists` identifies a caller-selected run ID collision.
`invalid_state_config` identifies a malformed XDG or relative database
selection. `state_unavailable` identifies a resolved state location that
cannot be prepared, such as an unwritable directory; its message names the
resolved path and the recovery is a writable `SLOPSHIPPER_DB`. All three
exit 2 and are recoverable by changing caller input.
