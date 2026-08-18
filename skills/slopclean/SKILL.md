---
name: slopclean
description: "Strip AI tells from writing, a diff, or a test suite: detect machine patterns (puffery, stock vocabulary, crutch punctuation, filler, narrating comments, defensive boilerplate, tautological or overmocked tests, repetitive setup), rewrite with a concrete human voice, and self-audit. Use when asked to unslop, de-slop, humanize, or clean up text, docs, commit messages, tests, or a change before it ships. Do not use for correctness review (slopguard's lane), fact-checking, or translating."
---

# Slopclean

Make the artifact read like a person made it on purpose. Three modes, same
instinct: cut what a machine reflexively adds, keep what carries meaning.

## Process

1. Pick the mode from the artifact: prose mode for writing, code mode for
   diffs and source, test mode for test files and suites. Load the matching
   reference; a diff touching both source and tests loads both.
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

## Test mode

Load [tests.md](references/tests.md). Headline moves: delete or replace
tautological tests (asserting the code against itself, vacuous existence
checks, self-made snapshots), unmock everything that is not a process,
network, clock, or randomness boundary, collapse repetitive setup into one
fixture or a table, and cut coverage theater: N happy-path clones with no
boundary or failure case.

## Boundaries

- Never change behavior, facts, or public API while cleaning; when a slop
  pattern hides a real defect, report it instead of polishing it.
- In test mode, never weaken a real assertion or delete a behavior's only
  coverage; replace it in the same pass or report the gap.
- Match the repository's or document's established conventions over any
  rule here.
- One cleaning pass per request; do not loop until sterile; over-cleaned
  text is its own tell.
