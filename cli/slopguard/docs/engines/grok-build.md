# Grok Build engine

Select with `--engine grok`. Install the official CLI and authenticate before
the first native review:

```bash
npm install --global @xai-official/grok
grok login
```

Defaults and shared runtime rules: [Review engines](README.md).

## Runtime contract

The adapter requires the official headless prompt-file, JSON Schema, explicit
model and effort, bounded-turn, permission, feature-disable, and
working-directory surfaces, and checks `--version` and `--help` before model
invocation. The Grok review process resolves authentication from the preserved
environment and user configuration.

- The frozen prompt is written to a private `0600` file inside the empty
  provider workspace and removed with it after the run. It never appears in
  process arguments.
- Every run disables plan mode, subagents, memory, shell, edits, file reads,
  grep, and Model Context Protocol (MCP) tools with `--no-plan`,
  `--no-subagents`, and the documented `GROK_MEMORY=0` and `GROK_SUBAGENTS=0`
  environment controls. Versions before 1.0.5 also receive `--no-memory`,
  which later releases removed.
- `dontAsk` permission mode silently denies tools without an explicit allow
  rule and prevents interactive approval prompts.
- Tool filtering uses Grok's documented internal IDs. The adapter always
  disallows the `search_tool` and `use_tool` MCP meta-tools and `Agent`.

## Web access

Off by default: the adapter also disallows `web_search` and passes
`--disable-web-search`, leaving no tools for the model to call. When on,
`web_search` and `web_fetch` are the only tools.

## Output contract

The adapter fixes `--max-turns 2`; the supported CLI can exit successfully with
a cancelled, missing structured result when bounded to one turn. Success
requires `stopReason: end_turn`, non-empty session and request identifiers, no
structured-output error, and complete canonical review objects in both `text`
and `structuredOutput` that agree exactly. Prose extraction and engine-local
protocol recovery are not accepted.

Grok also receives a trusted single-shot completion policy after the frozen
bundle. Its provider-only schema wraps the canonical review with completion
evidence: at least 160 characters of overall explanation and one inclusive
zero-based range covering the complete frozen file order exactly once.
Findings stay linked to reviewed files by the canonical review location.

- Because Grok can mechanically populate that shape while still describing
  future review work, the contract also requires at least 0.7 overall
  confidence that the entire review is complete.
- This threshold does not filter individual findings; every finding is
  retained regardless of its own confidence.
- The local decoder validates the range against the exact frozen file count,
  checks every finding path against that file set, then discards the private
  evidence before rendering the stable public result.
- These checks also reject explicit progress commitments such as starting,
  interim, or future review work in the overall explanation.
- The normal protocol retry gets one chance to return a complete review; a
  second incomplete result fails closed.

Grok treats the schema's unanchored non-whitespace string pattern as a
full-string constraint and otherwise truncates explanations, titles, bodies,
and paths to one character, so its provider-facing projection
([schema/embed.go](../../schema/embed.go)) omits that pattern along with the
`$schema` keyword and the path `not` rule, and adds the completion bounds
above. Canonical decoding still enforces non-blank text, length bounds, and
safe relative paths.

Capability discovery checks every trusted PATH candidate, skipping
incompatible tool-manager targets before selecting a real Grok executable,
and fails closed when no candidate preserves the version-specific required
flags or enumerated values.

## Verify

Default tests use a controlled fake executable. Optional authenticated smoke:

```bash
SLOPGUARD_TEST_LIVE_GROK=1 go test ./internal/provider -run '^TestGrokLive$' -count=1 -v
```
