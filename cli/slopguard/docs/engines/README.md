# Review engines

Slopguard runs exactly one review engine against an already frozen prompt and
returns one canonical review. Select the engine explicitly with `--engine` or a
trusted configuration source.

| Engine | Harness | Default model | Default effort | Runtime details |
| --- | --- | --- | --- | --- |
| `codex` | Codex CLI | `gpt-5.6-sol` | `medium` | [Codex CLI](codex.md) |
| `claude` | Claude Code | `claude-opus-5` | `high` | [Claude Code](claude-code.md) |
| `cursor` | Cursor Agent | `cursor-grok-4.6-high-fast` | `high` in model ID | [Cursor Agent](cursor.md) |
| `grok` | Grok Build | `grok-4.6` | `high` | [Grok Build](grok-build.md) |

## Shared runtime boundary

Engine adapters do not collect Git content, select another model, render a
report, retry, or mutate the reviewed repository. Orchestration owns target
collection, the one configured protocol retry, and final report construction.

The runtime resolves the selected executable to a regular executable outside
the reviewed repository and invokes it directly with an argument array.
Repository material is sent on standard input or through a private temporary
prompt file; it is never placed in process arguments.

Implicit PATH discovery checks candidates in order and skips only executables
that fail the provider capability contract. An explicit executable path stays
authoritative and never falls back. Successful executable identity, version,
and capability preparation is cached for the lifetime of one reviewer and one
web policy, so a malformed-review retry does not repeat probes.
Credentials, workspaces, processes, prompt/output files, timeout, and cleanup
state remain fresh for every attempt.

Every engine runs in an empty temporary workspace and its own process group with
a fixed timeout and bounded output. Machine-readable stdout has a hard size
limit. Stderr keeps bounded head and tail segments for private failure
classification; large non-fatal hook and progress diagnostics do not terminate
a successful provider run. The runtime terminates remaining process group
members after success, failure, timeout, cancellation, or stdout overflow.
Diagnostics redact credential-bearing environment values, escape terminal
control characters, and remain bounded.

Compatibility is capability-based, not an open-ended version whitelist. Each
adapter requires a canonical version plus its safety-critical flags and
enumerated option values. The engine pages record the oldest and newest
fixture-tested versions.

See [Configuration](../CONFIG.md) for configuration precedence and web-access
policy.

Use [`slopguard doctor`](../DOCTOR.md) to run only this executable and policy
preflight. Doctor never freezes a target or invokes the model.
