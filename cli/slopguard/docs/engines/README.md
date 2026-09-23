# Review engines

Slopguard runs exactly one review engine against an already frozen prompt and
returns one canonical review. Select the engine explicitly with `--engine` or a
trusted configuration source.

| Engine | Harness | Runtime details |
| --- | --- | --- |
| `codex` | Codex CLI | [Codex CLI](codex.md) |
| `claude` | Claude Code | [Claude Code](claude-code.md) |
| `grok` | Grok Build | [Grok Build](grok-build.md) |

Every adapter passes one explicit model and effort with no fallback. An empty
model resolves to the engine's `Default*Model` constant in
[provider/types.go](../../internal/provider/types.go); default effort comes from
[`applyProviderDefaults`](../../internal/config/load.go).
`slopguard config --engine <engine>` prints the effective effort; it shows an
unset model as `""`, and the review report's metadata names the model used.

## Shared runtime boundary

Engine adapters do not collect Git content, select another model, render a
report, retry, or mutate the reviewed repository. Orchestration owns target
collection, the one configured protocol retry, and final report construction.

The runtime resolves the selected executable to a regular executable outside
the reviewed repository and invokes it directly with an argument array.
Repository material goes on standard input or through a private temporary
prompt file, never in process arguments.

Implicit PATH discovery checks candidates in order and skips only executables
that fail the provider capability contract. An explicit executable path is
authoritative and never falls back. Successful executable identity, version,
and capability preparation is cached for one reviewer and one web policy, so a
malformed-review retry does not repeat probes. Credentials, workspaces,
processes, prompt/output files, timeout, and cleanup state are fresh for every
attempt.

Every engine runs in an empty temporary workspace and its own process group
with a fixed timeout and bounded output. Machine-readable stdout has a hard
size limit. Stderr keeps bounded head and tail segments for private failure
classification; large non-fatal hook and progress diagnostics do not terminate
a successful provider run. The runtime terminates remaining process group
members after success, failure, timeout, cancellation, or stdout overflow.
Diagnostics redact credential-bearing environment values, escape terminal
control characters, and remain bounded.

Compatibility is capability-based, not a version whitelist, with no numeric
upper bound. Each adapter requires a canonical version plus its
safety-critical flags and enumerated option values. Recorded help surfaces in
[provider testdata](../../internal/provider/testdata) pin the newest
fixture-tested versions.

Engines keep the existing environment, user configuration, and configured
provider or session authentication. Each attempt invokes the selected model
and may consume plan or API quota; only malformed output consumes the one
configured protocol retry.

See [Configuration](../CONFIG.md) for precedence and web-access policy.
[`slopguard doctor`](../DOCTOR.md) runs only this executable and policy
preflight; it never freezes a target or invokes the model.
