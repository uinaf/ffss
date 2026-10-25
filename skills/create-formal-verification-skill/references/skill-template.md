# Skill template

Put the skill where the repo's harnesses already find skills. With none,
use `.agents/skills/formal-verification/` and link
`.claude/skills/formal-verification` to it when the repo uses Claude Code.
Keep `SKILL.md` near 40 lines. Add a reference only for a checker whose
gotchas don't fit there, named after the checker (`references/tlc.md`).

```markdown
---
name: formal-verification
description: "Model-check <repo>'s <races, retries, state machines, protocols> with <checkers>. Use when changing code a model maps, adding a model, or reading a counterexample."
---

# Formal verification

`<task command>` checks every model and its negative-control variant.
Models live in `<path>`; each spec's header maps it to code.

## When you change mapped code

1. Find the model: the code's comment names it.
2. Update the model, or state in the change request why it still holds.
3. Run `<task command>`. Both outcomes must hold: models pass, variants
   fail on their named invariant.

## Reading a counterexample

<Where the trace prints, how to map its steps to code through the header
table, how to reproduce one model alone.>

## Adding a model

Invariant in one sentence, code owner, checker, bound and why, mapping
header, and a variant with the known bug. Wire it into `<task command>`.

## Where it runs

<Local for the small bound; heavy runs go through <routing skill or
runbook>.> CI: <job link>, path-filtered on models and mapped code.

## Report

Per model run: checker version, bound, distinct states or depth, wall time,
variant result.
```
