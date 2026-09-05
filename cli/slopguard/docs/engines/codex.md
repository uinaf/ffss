# Codex CLI engine

Select this engine with `--engine codex`. The adapter always passes an explicit
model; an empty model setting resolves to `gpt-6-astra`, with no fallback.
Reasoning effort defaults to `medium`.

If Astra is unavailable on your account, select `--model gpt-5.6-sol` with
the same medium effort.

## Runtime contract

The adapter capability-probes the following installed CLI surfaces before model
invocation:

- top level: `--config`, `--model`, `--ask-for-approval`, plus `--search` when
  web access is enabled;
- `exec`: `--ephemeral`, `--skip-git-repo-check`, `--output-schema`,
  `--output-last-message`, `--json`, `--cd`, and `--color`.

The probe also validates the `never` approval and color values used by the
adapter.

It checks `--version` and both help surfaces before invocation. The actual
`codex exec` process resolves authentication from the preserved Codex
configuration, including custom model-provider credential helpers.
Provider authentication, capability, timeout, cancellation, process, and
protocol failures remain distinct.

The compatibility contract is capability-based with no numeric upper bound.
Fixtures cover Codex CLI `0.146.0` through `0.153.2`; versions outside that
tested range are accepted only when they report a semantic version and expose
every required surface above.

## Web access

Web access is off unless trusted configuration enables it. The review keeps
the existing environment, user configuration, and configured provider or
session authentication, and runs in an empty temporary workspace.

## Output contract

Codex structured output uses a projection of the canonical JSON Schema that
omits the unsupported `not` path rule. The returned last-message file and JSON Lines
(JSONL) agent message must agree after surrounding whitespace is removed. The result is
then decoded against the complete canonical Go contract; schema projection does
not make an invalid result acceptable.

The JSON Lines (JSONL) decoder accepts the current `thread.started`,
`turn.started`, `item.started`, `item.updated`, `item.completed`,
`turn.completed`, `turn.failed`, and top-level `error` events. Item-level error
notices are non-fatal; top-level `error` and `turn.failed` events are provider
failures and do not consume the malformed-review retry. Their payloads remain
private.

Machine-readable stdout remains hard-limited. Stderr is truncated to bounded
private diagnostic head and tail segments without cancelling a successful
review. This keeps large Codex hook context or progress output from becoming
`provider output_limit` while retaining timeout and cancellation bounds.

## Verify

Default tests use a controlled fake executable. Run the optional authenticated
smoke explicitly:

```bash
SLOPGUARD_TEST_LIVE_CODEX=1 go test ./internal/provider -run '^TestCodexLive$' -count=1 -v
```
