# Specifications and Decisions

Use a durable specification when acceptance is ambiguous, spans several
modules or owners, or cannot be inferred from existing tests. Skip it for small
fixes, dependency updates, exploratory prototypes, and changes whose current
contract is already executable.

## Ownership and Layout

```text
docs/
├── specs/       long-lived behavior and acceptance
└── decisions/   why a consequential choice was made
```

These do not replace README, contributor, architecture, API, or operations
docs. Tactical execution belongs in the repository's preferred tracker unless
the repository explicitly uses a local plan directory.

## Specification Shape

```markdown
# <feature>

## Problem
What is wrong and for whom.

## Requirements
- R1: Observable requirement.

## Acceptance criteria
- [ ] Testable outcome.

## Constraints
- Compatibility, performance, privacy, or security boundary.

## Non-goals
- Explicit exclusion.
```

Name files `docs/specs/<feature-slug>.md`. Keep them concise enough to review
as one contract. Implementation may reveal ambiguities; record the decision,
update the spec, and update acceptance coverage together.

## Decision Shape

```markdown
# <NNNN>-<slug>

## Context
What forced the decision.

## Decision
What was chosen.

## Consequences
What this enables and prevents.
```

Name files `docs/decisions/<NNNN>-<slug>.md`. Record reversals as new decisions
that supersede the old one; do not rewrite history silently.

## Acceptance Coverage

Prefer executable acceptance, contract, or conformance checks when stable
inputs and outputs can express the behavior. They should:

- derive from requirements rather than implementation details
- map each case to an observable rule
- accept equivalent implementations
- include important edge and failure cases
- run through the repository's ordinary verification surface

Not every feature needs a language-neutral fixture. Existing integration or
end-to-end tests are enough when they already express the durable contract.

## Drift and Discovery

Add one task-shaped `AGENTS.md` pointer when agents need to find the spec or
decision directories. Do not copy their contents into the root guide.

Before calling work complete, reconcile specification, decisions, behavior,
and acceptance coverage. A spec that cannot be kept current is overhead: fix
its maintenance path, reduce it to the stable contract, or retire it.
