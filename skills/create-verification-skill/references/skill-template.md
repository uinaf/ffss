# Skill template

## Layout

Put the skill where the repo's harnesses already find skills. With none,
use `.agents/skills/verification/` and link `.claude/skills/verification` to
it when the repo uses Claude Code.

```text
verification/
├── SKILL.md
└── references/       # one file per lane family the repo has
    ├── devices.md    # emulators, simulators, physical devices
    ├── live.md       # test-account auth, fixtures, cleanup
    └── evidence.md   # capture, review, attach
```

Name references after the family, not the tool. Skip a family the repo
lacks. A formal-models lane links its own skill if the repo has one.

## Budget

- `SKILL.md`: about 60 lines, 80 at most. Every line routes, gates, or
  states a hazard.
- A reference: under 150 lines. A per-lane catalog longer than that belongs
  in code (test class docs, a task description, a `--list` flag). The
  reference then keeps the pattern and links the lookup.
- No copied flag tables, defaults, or task lists. Link the script or task
  and name the flag a reader needs.

## SKILL.md

```markdown
---
name: verification
description: "Prove a change in <repo>: gates, proof map, <lane families>. Use before claiming a change works, or when it needs a <device|live-account|evidence> lane."
---

# Verification

`<gate command>` is the merge gate; [<task file>](<link>) lists what it runs.

## Proof map

| Change | Proof | Negative control |
| --- | --- | --- |
| <kind of change> | <command or lane> | <how it goes red: fix reverted, injected failure, wrong fixture> |

## Where it runs

<Local: quick builds and single checks. Heavy: <work> goes through
<routing skill or runbook>; the same commands run there.>

## Hazards

- <shared device, shared account, destructive task, cost; one line each,
  stated as the safe action>

## Lanes

- <Family>: [<file>](references/<file>.md) when <trigger>.

## Report

What ran, failed, was skipped or unavailable, each with revision and
command. Label proof that made no live calls as synthetic.
```

## Description

The description is always loaded. Front-load "Prove a change in <repo>",
then one trigger per lane family. Leave out tool names a reader can't act on
from the description alone.
