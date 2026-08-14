# Agent protocol

Use `--json` on every command. Successful mutations return the resulting status
document. Failures return:

```json
{
  "schema_version": 1,
  "ok": false,
  "error": {
    "kind": "invalid_input",
    "message": "…",
    "exit_code": 2
  }
}
```

## Discover inputs

Run `slopshipper schema --command <name>` before composing an unfamiliar raw
payload. The runtime schema lists flags, required properties, nested types, and
enums. Do not infer enum spellings from prose.

Every mutation accepts `--input PATH`; use `--input -` for stdin. Raw input is
mutually exclusive with convenience flags, including `--run`, because the raw
payload carries its own `run` field.

The payload is transport, not durable state. Accepted evidence is stored in
the SQLite event log. Never materialize `*.evidence.json` or other command
payloads in the repository; use `--input -`, `--file -`, or `--evidence -`.

## Validate mutations

Add `--dry-run --json` to every mutation first. Apply only if `dry_run` is true,
`validated_command` matches, and the projected state and `next_action` match
the intended transition. Then repeat the command without `--dry-run`.

Dry runs use read-only state and never persist. `verify --cmd --dry-run` also
skips shell execution; its response validates the request against the current
state without pretending to know the command's exit code. It sets
`outcome_undetermined: true`; validate the command itself rather than expecting
a post-verification state.

`verify --cmd` hashes stdout and stderr separately, then records the SHA-256 of
`stdout=sha256:<hex>\nstderr=sha256:<hex>`. This stays deterministic across
presentation modes. JSON mode streams both child channels to stderr so stdout
stays machine-readable; plain mode preserves the child's stdout and stderr.

Treat every ID as a resource identifier, not free text. Use an ASCII letter or
digit first, followed by letters, digits, `.`, `_`, or `-`, up to 64 bytes.
Put human prose in title, question, answer, or reason fields.

## State location

The default database lives under the user's XDG data directory, outside the
repository. A sandbox may set `SLOPSHIPPER_DB` to an explicit writable path.
Relative overrides resolve from the Git worktree root. There is no automatic
repository-local fallback; use `.slopshipper/` only when the environment
requires it and the database, WAL, and shared-memory paths are untracked and
ignored. Run `slopshipper storage --json` to inspect the resolved path and Git
safety. The CLI never edits ignore files.

An existing caller-selected run ID returns `run_exists`; an invalid XDG or
relative database setting returns `invalid_state_config`; a state location
that cannot be prepared (for example an unwritable directory) returns
`state_unavailable` with the resolved path and a `SLOPSHIPPER_DB` recovery.
All exit 2 so an agent can correct its input without treating the failure as
an internal defect.
