# Security

Do not open a public issue for a suspected vulnerability.

Use GitHub's private vulnerability reporting from this repository's Security
tab (Report a vulnerability). Include the affected version or component, impact,
minimal reproduction, and any known mitigations. Do not include live credentials
or private source code.

Useful reports include secret exposure, command injection, path traversal,
unsafe command execution from stored evidence, and state-machine bypasses that
accept empty review or verify evidence.

The agent is not a trusted operator. The CLI validates strict JSON and resource
IDs at its boundary, then applies state transitions through revision and guard
checks. `--dry-run` never persists a transition, and `verify --cmd --dry-run`
never executes the command. A non-dry `verify --cmd` intentionally invokes the
local shell, so callers must not pass untrusted command text.
Repository-local SQLite overrides fail closed unless the database, WAL, and
shared-memory paths are untracked and ignored by Git.
See the [agent CLI contract](docs/AGENT_INTERFACE.md) for the supported
structured-input and dry-run boundaries.

Security fixes are applied on a best-effort basis to the latest release and the
latest code on `main`.
