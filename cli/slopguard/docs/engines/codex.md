# Codex CLI engine

Select this engine with `--engine codex`. The adapter always passes an explicit
model; an empty model setting resolves to `gpt-5.6-sol`, with no fallback.
Reasoning effort defaults to `medium`.

## Runtime contract

The adapter capability-probes the following installed CLI surfaces before model
invocation:

- top level: `--ask-for-approval`, plus `--search` when web access is enabled;
- `exec`: `--ephemeral`, `--skip-git-repo-check`, `--output-schema`,
  `--output-last-message`, `--json`, and `--cd`.

It checks `--version` and both help surfaces before invocation. The actual
`codex exec` process resolves authentication from the preserved Codex
configuration, including custom model-provider credential helpers.
Provider authentication, capability, timeout, cancellation, process, and
protocol failures remain distinct.

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

Documented `error` and `turn.failed` events are provider failures and do not
consume the malformed-review retry. Their payloads remain private.

## Verify

Default tests use a controlled fake executable. Run the optional authenticated
smoke explicitly:

```bash
SLOPGUARD_TEST_LIVE_CODEX=1 go test ./internal/provider -run '^TestCodexLive$' -count=1 -v
```
