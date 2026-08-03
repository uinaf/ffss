# Structuring

Lightweight file shapes for durable specifications and decisions. Templates, not ceremony.

## Directory Layout

```
docs/
├── specs/           # WHAT + WHY — long-lived contracts
└── decisions/       # WHY we chose X — ADR-lite
```

These directories are not a replacement for reader-facing docs like `README.md`,
`CONTRIBUTING.md`, architecture docs, or runbooks. Link them from routing docs when
useful. Keep tactical planning in the repository's preferred work tracker unless
the repository explicitly establishes a local plan directory as that tracker.

## Specs

Contracts that define "done." Stable after agreement. Change only when requirements change.

### Template

```markdown
# <feature-name>

## Problem
One paragraph. What's wrong and for whom.

## Requirements
- R1: <requirement>
- R2: <requirement>

## Acceptance Criteria
- AC1: <observable, testable outcome>
- AC2: <observable, testable outcome>

## Constraints
- <what must be true: perf, compat, security>

## Non-goals
- <what this explicitly does NOT cover>
```

### Naming
`specs/<feature-slug>.md` — e.g. `specs/password-reset.md`

### Lifecycle
Write → review → agree → implement. Update only when requirements change, not when the plan changes.

## Decisions

Why, not what. ADR-lite.

### Template

```markdown
# <NNNN>-<slug>

## Context
What forced the decision.

## Decision
What we chose.

## Consequences
What this enables and what it blocks.
```

### Naming
`decisions/NNNN-slug.md` — e.g. `decisions/0003-use-sqlite-not-json.md`

### Lifecycle
Append-only. New decisions get the next number. Add a new decision when you reverse course.

## Discovery

Agents find these through `AGENTS.md` pointers, not filesystem scanning.

When this workflow audits a repo and finds or creates specification or decision directories, it adds a routing entry to `AGENTS.md`:

```
## Docs
- Specs: docs/specs/
- Decisions: docs/decisions/
```

Keep AGENTS.md links shallow: directory plus one-line description, one pointer per directory.

## Rules

1. **One purpose per directory.** Keep specifications and decisions distinct from reader-facing guides.
2. **Specs define behavior.** Update them only when requirements change.
3. **Decisions explain consequential choices.** Keep them append-only and record reversals separately.
4. **Link canonical work.** Specifications and decisions may link to tracker work without copying its status or task list.
5. **Drift is a signal.** A specification change without corresponding acceptance coverage is a defect.
