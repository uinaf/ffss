# Claude Code engine

Select with `--engine claude`. The adapter always passes an explicit model; an
empty model setting resolves to `claude-fable-5-1`, with no fallback. Effort
defaults to `medium` and must be `low`, `medium`, `high`, `xhigh`, or `max`.

If Fable 5.1 is unavailable on your account, select `--model claude-opus-5`
with the same medium effort.

## Runtime contract

The adapter capability-probes the installed CLI before model invocation. The
run requires `--print`, `--no-session-persistence`, JSON-structured output, an
explicit model and effort, a fixed tool inventory, `dontAsk` permissions, and
disabled Chrome integration.

The Claude review process resolves authentication from the preserved
environment and user configuration. A separate auth-status surface cannot
block a configured session, gateway, helper, or key.

Compatibility is capability-based with no numeric upper bound. Fixtures cover
Claude Code `2.1.220` through `2.1.260`; other versions must expose the same
required flags before Slopguard invokes the model.

The frozen prompt is delivered on standard input followed by a trusted review
policy. Each finding location must fit within one reviewed line range;
cross-hunk concerns must be narrowed to one establishing range or split into
separately valid findings.

## Web access

Off by default. When enabled, `WebSearch` is the only exposed tool; filesystem,
shell, MCP, browser, and unrestricted fetch tools stay unavailable. The
provider keeps its existing environment, user configuration, and configured
provider or session authentication, and runs from an empty workspace with an
explicit safe tool inventory.

## Output contract

The adapter accepts one Claude `result` envelope with `subtype: success`,
`is_error: false`, and an object in `structured_output`. The inner object must
pass the complete canonical Go decoder. Prose extraction and engine-local
protocol recovery are not accepted. The shared orchestration retry adds a
sanitized correction for the rejected rule without including the previous
response or repository content.

A valid outer result with `is_error: true` and a documented completion subtype
is a provider-reported failure, not a malformed review. Accepted subtypes:
`success`, `error_during_execution`, `error_max_turns`, `error_max_budget_usd`,
and `error_max_structured_output_retries`. Typed `401` and `403` statuses
become authentication; `429` remains a provider failure with a stable
rate-limit diagnostic. Raw provider error text is never copied into the
report.

## Verify

Default tests use a controlled fake executable. Optional authenticated smoke:

```bash
SLOPGUARD_TEST_LIVE_CLAUDE=1 go test ./internal/provider -run '^TestClaudeLive$' -count=1 -v
```
