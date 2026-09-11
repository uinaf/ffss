# Cursor Agent engine

Select with `--engine cursor`. The adapter always passes an explicit model; an
empty model setting resolves to `cursor-grok-4.6-high`, with no fallback.
Cursor model IDs encode effort, so a separate non-default `reasoning_effort`
is rejected; the default model keeps high reasoning.

## Runtime contract

The adapter requires the non-interactive JSON, Ask mode, workspace, trust, and
model CLI surfaces. The Cursor review process resolves authentication from the
preserved environment and user configuration, including API-key wrappers. The
reviewed source is never mounted in the provider workspace; the frozen prompt
is delivered on standard input.

Compatibility is capability-based with no date or build upper bound. Fixtures
cover Cursor Agent `2026.07.23-e383d2b` through `2026.09.02-c22c1a3`; other
builds must expose the same required flags and enumerated option values.

Review preflight also runs `models`. Cursor answers `--version` and `--help`
identically whether or not it holds a credential, so a logged-out executable
clears every capability check and would otherwise fail only after the prompt was
spent. `doctor` skips this probe and stays offline.

## Web access

Cursor Agent has no documented per-run web-disable flag, so explicit CLI
selection with `--engine cursor` enables otherwise-unset web access. Engine
selection from repository, environment, or XDG configuration does not. An
explicit `web_access: false` is authoritative and fails capability preflight
rather than claiming an unenforceable guarantee. The review keeps the existing
environment, user configuration, and configured provider or session
authentication, and runs in Ask mode in an empty workspace with user sandbox
configuration preserved.

## Output contract

The outer JSON must be one successful Cursor `result` envelope with a non-empty
string result. The adapter appends a trusted protocol instruction and the exact
embedded review schema after the frozen bundle.

An outer object with `type: result`, `subtype: error`, `is_error: true`, and a
non-empty `result` is a provider-reported failure and does not consume the
malformed-review retry. Unknown subtypes and missing or blank result text are
malformed protocol output. Provider result text remains private.

- The inner result is first decoded as exactly one canonical review object.
- If that fails, the only recovery accepts non-JSON prose followed by one
  complete canonical object that consumes the remaining suffix.
- Fences, ambiguous braces, JSON-value prefixes, malformed or multiple objects,
  suffix prose, and non-canonical reviews fail closed.
- Successful recovery is recorded as `cursor_trailing_object`.
- Rejected documents receive a sanitized category such as invalid JSON, invalid
  document shape, multiple documents, suffix content, fenced output, or schema
  mismatch.
- The one shared retry converts that category into trusted correction guidance
  without echoing provider output.

## Verify

Default tests use a controlled fake executable and a complete recovery matrix.
Optional authenticated smoke:

```bash
SLOPGUARD_TEST_LIVE_CURSOR=1 go test ./internal/provider -run '^TestCursorLive$' -count=1 -v
```
