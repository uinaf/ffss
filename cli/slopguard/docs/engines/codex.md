# Codex CLI engine

Select with `--engine codex`. Defaults and shared runtime rules:
[Review engines](README.md).

## Runtime contract

The adapter capability-probes these CLI surfaces before model invocation:

- top level: `--config`, `--model`, `--ask-for-approval`, plus `--search` when
  web access is enabled;
- `exec`: `--ephemeral`, `--skip-git-repo-check`, `--output-schema`,
  `--output-last-message`, `--json`, `--cd`, and `--color`.

The probe also validates the `never` approval and color values the adapter
uses, and checks `--version` and both help surfaces. The `codex exec` process
resolves authentication from the preserved Codex configuration, including
custom model-provider credential helpers. Authentication, capability, timeout,
cancellation, process, and protocol failures remain distinct.

## Web access

Off unless trusted configuration enables it; then only `--search` is added.

## Output contract

Codex structured output uses a projection of the canonical JSON Schema that
omits the unsupported `not` path rule. The returned last-message file and JSONL
agent message must agree after surrounding whitespace is removed. The result is
then decoded against the complete canonical Go contract; schema projection does
not make an invalid result acceptable.

The JSONL decoder accepts `thread.started`, `turn.started`, `item.started`,
`item.updated`, `item.completed`, `turn.completed`, `turn.failed`, and
top-level `error` events. Item-level error notices are non-fatal; top-level
`error` and `turn.failed` events are provider failures and do not consume the
malformed-review retry. Their payloads remain private.

## Verify

Default tests use a controlled fake executable. Optional authenticated smoke:

```bash
SLOPGUARD_TEST_LIVE_CODEX=1 go test ./internal/provider -run '^TestCodexLive$' -count=1 -v
```
