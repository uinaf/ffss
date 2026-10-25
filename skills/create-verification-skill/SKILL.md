---
name: create-verification-skill
description: "Generate a repo-local verification skill that tells agents how to prove a change in that repository: gates, proof map, device and live-account lanes, fixtures, evidence, and negative controls. Use when setting one up, or when long harness or testing docs should become a skill."
---

# Create verification skill

Write a short repo-local `verification` skill that an agent loads before it
proves a change. Scripts, build tasks, CI, and `--help` stay the source of
truth; the skill is a **cache** of what lookup can't find. It replaces the
harness prose it absorbs. Installing published stack skills and fixing
ordinary doc drift are out of scope.

## Steps

1. **Inventory.** Read the agent guide, README, testing and harness docs, CI
   workflows, build tasks, and every proof script's header and `--help`.
   Where to look: [inventory.md](references/inventory.md). Done when each
   command, lane, device, credential path, fixture rule, and evidence rule is
   a row with its owner (file and line, task, or flag).
2. **Bin each row.**
   - **Lookup:** a script, task, flag, or `--help` already states it. Link
     the owner.
   - **Cache:** true, needed, and stated nowhere mechanical: hazards,
     unwritten conventions, gotchas, why a lane exists. It goes in the skill.
   - **Move:** a fact a script should own. Put usage text into the script's
     header or `--help`. A new script check (preflight, refusal) is a
     proposal, not part of this run.
   - **Drop:** history, duplicates, one-off observations, retired lanes.

   Done when every row has a bin.
3. **Interview for gaps.** Ask one batch of questions, only for answers the
   sources lack that would change the skill. Typical gaps are in
   [inventory.md](references/inventory.md#gaps). Without an answer, write the
   safe default, mark it `TODO(owner)`, and list it in the report. Invent no
   account, host, or device name.
4. **Write the skill.** Layout, `SKILL.md` template, and length budget:
   [skill-template.md](references/skill-template.md). One reference per lane
   family; what each must answer: [lanes.md](references/lanes.md). Done when
   every Cache row sits in exactly one file and every proof-map row names
   its negative control.
5. **Slim the old docs.** Delete what the skill or a script now owns. Leave a
   one-line pointer where people land (README, agent guide proof map).
   Product rules stay in their owning doc. Fix links to removed anchors.
6. **Prove it.**
   - The repo's Markdown and link checks pass.
   - Each command in the skill runs, or its `--help` or dry run exits 0.
     Skip anything that mutates shared state.
   - **Cold read:** give a fresh agent only the new skill and two or three
     recent changes from history, one of them docs-only. It names the gate,
     lane, evidence, and negative control for each. Compare with what those
     changes ran.

   Done when the cold read matches, or the skill is fixed until it does.

## Report

Skill tree, lines added and removed, Move proposals, `TODO(owner)` gaps,
and each check with its command and result.
