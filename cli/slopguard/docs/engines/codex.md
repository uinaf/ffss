# Codex CLI engine

Select with `--engine codex`. Defaults and shared runtime rules:
[Review engines](README.md). The built-in model is GPT-6.1 Sol with medium
reasoning; explicit model and effort settings override these defaults.

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

## Repository access

With `--repository-access`, the adapter runs Codex's own reviewer instead of
`codex exec` with the bundle. Slopguard writes a fresh two-commit Git
repository, the redacted base and the redacted target with no other history,
and runs `codex exec review` there in the read-only sandbox with
`project_doc_max_bytes=0`. The trusted task prompt goes on stdin after an
instruction to review `HEAD~1..HEAD`. The probe also requires `--ephemeral`,
`--output-last-message`, and `--json` on `exec review`.

`exec review` ignores `--output-schema` and prints its findings in a fixed
text template: a summary, then `- [P1] title — /path:start-end` lines with
indented bodies. The adapter converts that template into the review document
and fails closed on any other line, a finding before the findings heading, or
a path outside the review repository. The template reports no confidence or
category, so every finding records confidence 0.5 and category `bug`, and the
review records overall confidence 0.5.

## Web access

On by default, which adds only `--search`. When off, the adapter passes
`--config web_search="disabled"`.

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

Duplicate JSON fields are rejected, except a repeated `item.id`, which Codex
emits on `web_search` items. An
`invalid_envelope` failure names the violated rule, such as
`event after turn.completed`, without provider payload content.

## Verify

Default tests use a controlled fake executable. Optional authenticated smoke:

```bash
SLOPGUARD_TEST_LIVE_CODEX=1 go test ./internal/provider -run '^TestCodexLive$' -count=1 -v
```
