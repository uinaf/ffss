---
name: slopclean
description: "Strip AI tells from writing or from a diff: detect machine patterns (puffery, stock vocabulary, crutch punctuation, filler, narrating comments, defensive boilerplate), rewrite with a concrete human voice, and self-audit. Use when asked to unslop, de-slop, humanize, or clean up text, docs, commit messages, or a change before it ships. Do not use for correctness review (slopguard's lane), fact-checking, or translating."
---

# Slopclean

Make the artifact read like a person made it on purpose. Two modes, same
instinct: cut what a machine reflexively adds, keep what carries meaning.

## Process

1. Pick the mode from the artifact: prose mode for writing, code mode for
   diffs and source. Load the matching reference.
2. Scan for the patterns; rewrite in place. Preserve meaning and intended
   tone; this is a cleaning pass, never a rewrite of substance.
3. Add voice where the text is sterile: state opinions, vary rhythm, prefer
   the concrete mechanism or number over the vibe.
4. Self-audit: "what still makes this obviously machine-made?" Fix it.

## Prose mode

Load [prose.md](references/prose.md). Headline moves: cut puffery and
stock AI vocabulary, name sources or delete vague attributions, delete
every em dash (banned; banner alt text is the one exception), kill crutch punctuation and
inline-header lists, split long sentences, tighten verbose UI copy,
delete chatbot phrases and hedging stacks, and replace feeling-words
with mechanisms and numbers.

## Code mode

Load [code.md](references/code.md). Headline moves: delete comments that
narrate the next line or the change's own history, remove defensive
boilerplate nobody asked for, collapse abstraction layers with one caller,
drop dead config and unused escape hatches, and rename hedged identifiers
to say what the thing is.

## Boundaries

- Never change behavior, facts, or public API while cleaning; when a slop
  pattern hides a real defect, report it instead of polishing it.
- Match the repository's or document's established conventions over any
  rule here.
- One cleaning pass per request; do not loop until sterile; over-cleaned
  text is its own tell.
