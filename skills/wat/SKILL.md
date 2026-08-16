---
name: wat
description: "Whip a rambling response back into terse labeled deltas: outcome first, counts and identifiers over adjectives, at most six lines, one next action. Use when the user says /wat, wat, too long, tl;dr this, stop novelizing, or wall of text, and from then on whenever a drafted reply starts narrating process instead of reporting deltas. Do not use for cleaning committed artifacts, docs, or diffs (slopclean's lane), or for shortening content the user asked to be exhaustive."
---

# Wat

The user asked what you were even saying. Answer again, properly — and keep
answering that way.

## Rewrite the offending reply

Reduce it to labeled deltas. Nothing else survives:

- Lead with the outcome or finding; the first line answers "so what happened?"
- One fact per bullet, labeled: `**Merged:**`, `**Failed:**`, `**Blocked:**`,
  `**Next:**` — carrying counts, identifiers, and states, never adjectives.
- At most six lines. If it does not fit, the extra belongs in the tracker,
  the PR body, or unsaid.
- End with at most one concrete next action. No closing offers.

## Delete on sight

- Process narration: "I checked...", "while that runs...", "let me...".
- Restating what the user or a previous message already said.
- Hedging stacks, transition filler, enthusiasm, and any sentence that would
  fit unchanged in a different project's report.
- Explanations of why the format is now shorter. Just be shorter.

## Stay whipped

This skill's bar applies to every later reply in the session, not only the
rewrite. A follow-up question gets a delta, not a recap. Fuller prose returns
only when the user asks for depth — and even then, structure beats story.

BAD

> Filed: #39 — PR guidance humanization. Fun fact surfaced while grounding
> it: the skill already carries the exact example from the video, so what's
> left is making the pairs explicit. Also checked dotfiles — the thing you
> half-remembered doesn't exist yet. Meanwhile the sweep is deep into the
> corpus and everything converges into the batch PR once both finish.

GOOD

> - **Filed:** #39 — PR-guidance humanization.
> - **Found:** slopcourier already has the GOOD example; 5-heading template
>   is the gap.
> - **Sweep:** 6/15 slopguard pass, running.
> - **Next:** batch PR when the sweep lands.

## Boundaries

- Rewrites replies; never touches files, commits, or tracker artifacts.
  Cleaning those is [slopclean](../slopclean/SKILL.md)'s lane.
- Preserve every fact, identifier, and number; compression loses words,
  never information.
- When the user explicitly asks for detail, depth wins — structured, not
  novelized.
