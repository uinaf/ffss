---
name: wat
description: "Whip a rambling reply into terse labeled deltas: outcome first, counts over adjectives, six lines max. Use on /wat, tl;dr, or wall-of-text complaints, then keep applying; not for committed artifacts (slopclean's lane)."
disable-model-invocation: true
---

# Wat

The user asked what you were even saying. Answer again, properly, and keep
answering that way for the rest of the session.

## Rewrite

- First line answers "what happened?"
- One labeled fact per bullet (`**Merged:**`, `**Failed:**`, `**Next:**`)
  with counts and identifiers, never adjectives.
- At most six lines; end with at most one next action, no closing offers.
- Delete process narration, restatement, hedging, filler, and any sentence
  that would fit unchanged in another project's report.

BAD

> Filed: #39. PR guidance humanization. Fun fact surfaced while grounding
> it: the skill already carries the exact example from the video. Also
> checked dotfiles. The thing you half-remembered doesn't exist yet.
> Meanwhile the sweep is deep into the corpus and everything converges into
> the batch PR once both finish.

GOOD

> - **Filed:** #39, PR-guidance humanization.
> - **Found:** slopcourier already has the GOOD example; 5-heading template
>   is the gap.
> - **Sweep:** 6/15 slopguard pass, running.
> - **Next:** batch PR when the sweep lands.

## Boundaries

- Replies only; files, commits, and tracker artifacts are
  [slopclean](../slopclean/SKILL.md)'s lane.
- Drop what the tracker or PR already records, never a decision-relevant
  fact, identifier, or number. If those truly exceed six lines, the cap
  yields; doubt "truly" first.
- When the user asks for depth, depth wins. Keep it structured, not novelized.
