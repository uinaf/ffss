---
name: docs
description: "Audit, compress, restructure, and update repo documentation and durable agent-facing artifacts such as AGENTS.md, README.md, docs/, specs, decisions, and runbooks. Use when changes risk doc drift or when docs need current-state cleanup or context-efficient agent-first rewriting. Do not use to create tactical work plans, epics, or tracker tickets, or for code review, runtime verification, or boot/readiness infrastructure setup."
---

# Docs

Keep the repo legible to agents and humans.

## Principles

- Docs rot silently — every code change is a possible doc change
- Optimize for decision-relevant, repo-owned truth per token
- Describe current state; keep history only in migration notes, changelogs, and decisions
- Lead with supported capabilities and next actions
- Prefer short, task-shaped sections and structured facts over narrative prose
- Keep routing docs short and point to deeper docs instead of duplicating them
- Use repo-relative links for in-repo docs
- Keep repo docs, agent guidance, and work tracking linked but distinct

## Negative-state rule

When rewriting current-state docs, delete every absent, removed, or unprovisioned item, including specific names and category paraphrases, unless it changes a plausible current action. Prior curiosity, old tickets, and "agents ask about it" do not make an absence operational. If a limitation must stay, state one precise boundary and the supported path.

## Boundaries

Not docs work: tactical work planning; epic or tracker-ticket creation; boot/readiness setup; baseline PR, issue, contributor, or security policy templates; independent code review; runtime verification.

## Workflow

### 1. Audit the doc surface

Check the files agents and humans actually rely on:

- `AGENTS.md`
- `CLAUDE.md`
- `README.md`
- `CONTRIBUTING.md`
- `SECURITY.md`
- `docs/`
- durable specs, runbooks, and decision docs

Flag stale commands, dead paths, duplicate guidance, routing failures, narrative history, exhaustive negative inventories, and repo-internal details leaking into reader-facing docs.

Before editing:

- classify the target as repo docs, agent guidance, or a durable spec or decision
- identify the decision or task each section supports
- preserve commands, paths, invariants, boundaries, and recovery steps

Use [references/source-boundaries.md](references/source-boundaries.md) before writing cross-repo, private workspace, or local-machine facts into checked-in docs.

### 2. Update routing docs

Keep top-level docs terse and navigational.

- `AGENTS.md` should be a compact operating contract and map, not a wiki
- If the repo uses `AGENTS.md`, make `CLAUDE.md` a symlink or `@AGENTS.md` import instead of maintaining a second authored file
- `README.md` should lead with value, quick use, and links to deeper docs
- Refresh `CONTRIBUTING.md` and `SECURITY.md` when they already exist or when moving existing policy out of an overloaded `README.md`; do not invent baseline policy from scratch
- For workspace repos, keep one canonical setup doc and let `README.md` point to it
- Use the concrete top-level split and section order in [references/documentation.md](references/documentation.md)
- Use reader-facing labels in routing lists; use raw filenames only when the filename matters

### 3. Update deep docs and specs

Refresh the detailed documents that carry the knowledge.

- architecture and API docs
- task guides and runbooks
- durable feature specs and decision records
- readiness infrastructure docs after boot, smoke, observability, or isolation changes

Write each updated section as the reader's current source of truth.

For agent-facing or internal docs, follow the selection, structure, and progressive-disclosure guidance in [references/agent-first.md](references/agent-first.md).

When the user asks to save a durable rule, prompt, specification, or decision, choose its [durable home](references/source-boundaries.md#durable-homes).

Tactical implementation plans, epics, tracker tickets, and session handoffs are a separate work-tracking operation. When requested alongside docs cleanup, keep the surfaces linked and report the planning work as a distinct next action instead of inventing a checked-in plan home.

For durable feature contracts and decisions, use the selection rules and
templates in [references/specifications.md](references/specifications.md).

### 4. Clean up drift

- deduplicate repeated facts
- delete or archive stale docs
- fix cross-links and moved paths
- keep naming, labels, casing, commands, and section order consistent
- keep one canonical home for setup or install commands and replace copied command blocks with pointers
- prefer reader-facing link text over raw paths unless the path is the point
- apply the [agent-first cleanup filters](references/agent-first.md#select-information)

### 5. Validate reality

Verify prose against the repo.

Concrete checks:

- `rg -n "old/path|stale-command" AGENTS.md CLAUDE.md README.md docs/` when paths or commands moved
- `rg -n "<new command|new path|decision keyword>" AGENTS.md CLAUDE.md README.md docs/` to find duplicate or conflicting homes
- `test -e <path-from-docs>` before keeping a file reference
- `test ! -e AGENTS.md || { test -L CLAUDE.md && test "$(readlink CLAUDE.md)" = "AGENTS.md"; }` when normalizing agent entrypoints
- for claims sourced from outside the repo, cite or verify the upstream source before making the claim durable
- compare the before and after facts; every current command, invariant, boundary, and recovery path must remain represented
- apply the negative-state rule above to the final rewrite

## Output

After docs work, report a compact docs footer:

- files updated
- verified: command names or path checks, not output logs
- removed or rewritten: only if stale or duplicated docs changed
- gaps: remaining doc gaps, or `none`
- next: work planning, readiness setup, independent review, runtime verification, or `none`

Keep the footer to 5 labeled lines or fewer. List changed files once.

## References

- [references/agent-first.md](references/agent-first.md) — agent-first writing, positive state, progressive disclosure, AGENTS.md shape
- [references/documentation.md](references/documentation.md) — README, contributing, security, and repository-doc shapes
- [references/source-boundaries.md](references/source-boundaries.md) — durable ownership and private/local evidence boundaries
- [references/specifications.md](references/specifications.md) — when to create specs or decisions, compact templates, acceptance coverage, and drift
