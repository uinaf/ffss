# House Style

The writing shape for every durable artifact: docs, skills, issues, change
requests, review comments, commit messages, and reports. Enforce these rules on
every pass; rewrite violations instead of preserving the author's shape.

## Rules

- Lead with the outcome, capability, or action. Rationale comes after.
- Default to at most 6 lines of prose per section; grow with bullets, tables,
  or trees, not paragraphs.
- Write short sentences, one fact per line.
- Report status as labeled bullets such as `**Updated:**`, `**Verified:**`, and
  `**State:**`, carrying counts, identifiers, and states instead of adjectives.
- Link URLs and issue, PR, or MR references; reserve backticks for code,
  commands, paths, and literal values.
- Prefer plain words over jargon. Expand an acronym, codename, or internal
  label on first use.
- Add a small diagram, tree, or table when structure or flow beats prose.
- No greetings, hedges, marketing tone, or narration of the author's own
  actions. Cap disclaimers and caveats at one clause.
- No em dashes; use a period, colon, or comma instead (banner alt text is
  the one exception).
- Commit to one recommendation with exact commands and `file:line` targets.
  End with at most one concrete next action.
- In a uinaf-owned repository, voice and register (casing and name styling)
  are owned by the live design contract: fetch `voice` via the
  design.uinaf.dev MCP `search_guidelines` (fallback
  [design.md](https://design.uinaf.dev/design.md)) and apply it over this
  file where they overlap. Its rules are not restated here.

## The examples are the specification

Match these shapes exactly.

Bad, dense narration the reader must mine for facts:

> The package pins `^0.1.0` while `1.13.4` renamed both button classes, so
> both buttons render as bare text; on top of that a local stylesheet
> re-implements the shell, heading, and footer already shipped upstream.
> Starting the fix now: bump the pin, rebuild both pages, then open a PR.

Good, the same report as labeled deltas ending in one next action:

> - **Broken:** stale `^0.1.0` pin; `1.13.4` renamed both button classes.
> - **Fix:** bump the pin, rebuild 2 pages, delete 122 duplicated lines.
> - **Verified:** build and 17 tests pass.
> - **State:** pushed [a1b2c3d](https://github.com/example/site/commit/a1b2c3d);
>   2 of 2 deploys green.
>
> Next: merge [#12](https://github.com/example/site/pull/12) once CI is green.

Bad, references trapped in code spans the reader cannot click:

> - **State:** `!142` merged; docs at `https://docs.example.com/setup`.

Good, references as links, backticks only for literals:

> - **State:** [!142](https://gitlab.example.com/acme/app/-/merge_requests/142)
>   merged; [setup docs](https://docs.example.com/setup) now pin `RETRY_LIMIT=5`.

Bad, explaining a system with a stage-by-stage deep dive:

> (40 lines tracing every pipeline stage, runner, and environment variable)

Good, a high-level summary; depth behind a link or on request:

> Push to `main` builds, tests, and deploys to 2 regions via GitHub Actions.
> Rollback is a tag revert.

## Applying to docs

- A paragraph is allowed only for rationale, trade-offs, or causal
  explanation, and holds one idea in two or three sentences.
- Any section where the reader must mine prose for commands, paths, or
  states is a violation; restructure it as bullets or a table.
- Keep examples minimal and normative; one bad/good pair beats three.
- Structure rules (headings, pointers, disclosure) live in
  [agent-first.md](agent-first.md); this file governs the sentences.
