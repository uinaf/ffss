# Claude Code engine

Select this engine with `--engine claude`. The adapter always passes an explicit
model; an empty model setting resolves to `claude-opus-5`, with no fallback.
Effort must be `low`, `medium`, `high`, `xhigh`, or `max`.

## Runtime contract

The adapter capability-probes the installed CLI before model invocation. Every
mode requires `--print`, `--no-session-persistence`, JSON-structured output, an
explicit model and effort, a fixed tool inventory, `dontAsk` permissions, and
disabled Chrome integration. Strict mode also requires `--safe-mode`, user-only
setting sources, strict Model Context Protocol (MCP) configuration, and an
MCP-tool deny rule.

Native mode lets the actual Claude review process resolve authentication from
the preserved environment and user configuration. A separate auth-status
surface cannot block a configured session, gateway, helper, or key.

The frozen prompt is delivered on standard input followed by a trusted review
policy. Each finding location must fit completely within one individual
reviewed line range; cross-hunk concerns must be narrowed to one establishing
range or split into separately valid findings.

## Isolation and web access

Web access is off by default. When enabled, `WebSearch` is the only exposed
tool; filesystem, shell, MCP, browser, and unrestricted fetch tools remain
unavailable.

| Mode | Authentication and configuration | Isolation controls |
| --- | --- | --- |
| `native` (default) | Existing environment, user configuration, and configured provider or session authentication | Empty workspace and explicit safe tool inventory |
| `strict` | Empty home and Claude state; requires `ANTHROPIC_API_KEY` | Safe mode, disabled auto-memory, strict MCP, and no unsafe tools |

## Output contract

The adapter accepts one Claude `result` envelope with `subtype: success`,
`is_error: false`, and an object in `structured_output`. The inner object must
pass the complete canonical Go decoder. Prose extraction and engine-local
protocol recovery are not accepted. The shared orchestration retry adds a
sanitized correction for the specific rejected rule without including the
previous response or repository content.

A valid outer result with `is_error: true` and a documented completion subtype
is a provider-reported failure, not a malformed review. The accepted subtypes
are `success`, `error_during_execution`, `error_max_turns`,
`error_max_budget_usd`, and `error_max_structured_output_retries`. Typed `401`
and `403` statuses become authentication; `429` remains a provider failure with
a stable rate-limit diagnostic. Raw provider error text is never copied into
the report.

## Verify

Default tests use a controlled fake executable. Run the optional authenticated
smoke explicitly:

```bash
SLOPGUARD_TEST_LIVE_CLAUDE=1 go test ./internal/provider -run '^TestClaudeLive$' -count=1 -v
```
