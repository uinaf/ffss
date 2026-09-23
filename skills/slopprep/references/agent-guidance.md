# Agent Guidance Readiness

Audit `AGENTS.md` as an executable operating contract, not a style sample. It
is ready when a capable agent can identify the owner, choose scope, act within
authority, run the lifecycle, and report evidence from a cold start without
guessing.

## Canonical Shape

Audit the guide's shape against
[../../slopscriber/references/agent-first.md](../../slopscriber/references/agent-first.md):
one authored source with a `CLAUDE.md` symlink or import, root content versus
task-shaped pointers, the working model a product repository needs, abstract
preferences translated into decisions. This reference adds only owner context,
the cross-model contract, the audit procedure, and grade effects.

## Human and Owner Context

In a private owner workspace, a short human introduction improves judgment:
the person's work, priorities, collaboration style, a useful recurring failure
mode, and pointers to the canonical private or public profile sources. It is a
compass, not a synthetic persona or biography.

In a shared product repository, orient around the product, users, maintainer
contract, and ownership boundaries; no personal biography.

Keep sensitive identity, credentials, finance, machine state, and private
workspace facts behind task-relevant pointers so an agent opens the smallest
source needed.

## Proof Map

The global guide stays generic, so the repository guide owns its proof. Map
each change class the repository actually has to its check, runner, and
artifact, from what its surfaces and existing lanes already support:

- changes that need only static checks, such as docs, copy, or lint-only
  config
- component or visual changes: stories, visual diffs, or rendered captures
- user workflows: end-to-end or black-box checks against the running product
- device or platform behavior: simulator, emulator, or hardware runs with a
  recording and the commands that produced it
- deployed or provider-backed behavior: live checks and their scope limits
- pure logic with a wide input space: unit or property tests

Name where expensive or platform-specific lanes run, such as a declared
remote runner, and keep the command identical to the local one. Omit classes
the repository does not have; an invented lane makes agents run proof the
change cannot need.

## Cross-Model Contract

Write shared guidance as observable behavior that works across capable model
families:

- define deliverable, scope, authority, evidence, and terminal condition
- say when to act and when a decision genuinely needs the user
- require current evidence for progress and completion claims
- keep communication preferences concrete and non-repetitive
- state a recurring failure only when it changes the desired behavior
- preserve room for model judgment inside mechanically enforced boundaries
- give the reason behind a rule in plain words; capitalized or absolute
  pressure makes current models apply it too broadly
- keep exact step sequences for fragile operations; for judgment work, state
  the outcome and let the model choose the steps
- ask reviewers for every suspected finding and filter afterwards; severity
  thresholds in the request suppress real findings
- leave out inherited review-round caps, fix-commit limits, and context or
  token countdowns; agents read them as stop signals and end authorized work
  early. Keep explicit user limits and machine timeouts
- state each rule once; repeated "ask first" lines and conflicting rules cause
  needless stops
- state that user instructions outrank skill guidance, within the harness's
  instruction hierarchy
- name the early stops to avoid and the ones to keep, such as a decision only
  the user can make

Current models follow instructions literally, so guidance written to correct
an older model's failure tends to overcorrect. Measured tendencies also
differ: Opus 5 over-delegates small tasks, while GPT-6 Astra stops to ask
early. Shared text states the behavior each should reach, not a fix for one. When models
change, re-test inherited rules against first-party guidance
([Claude](https://platform.claude.com/docs/en/build-with-claude/prompt-engineering/claude-prompting-best-practices),
[GPT-6](https://developers.openai.com/api/docs/guides/latest-model)) and delete
those whose failure no longer reproduces.

Do not stack model-specific prompt fragments in the shared guide. Keep model
selection, reasoning effort, verbosity, tool policy, and harness-specific
invocation controls in their owning configuration. When a named-model guide is
required, verify its current first-party prompting documentation and model or
system card; record only a measured, stable difference that cannot live in the
harness.

## Audit Procedure

1. Read the root guide and every pointer the requested task class requires.
2. Resolve named commands through tracked task owners, delegated scripts,
   nested packages, and hidden CI; check what each gate covers.
3. Compare inspection or improvement authority with each command's
   prerequisites, credentials, cost, and state changes. Inspect runtime
   ownership and teardown before execution; a familiar command name is not a
   safe scope.
4. Exercise the authorized proof path on the declared runner. Report
   unavailable or unsafe paths as gaps; inspection does not authorize
   bootstrap or repairs.
5. Trace one expected failure from command to surfaced diagnostic and recovery.
6. Identify duplication, contradictions, volatile claims, hidden prerequisites,
   and facts that live only in chat or a person's memory.
7. Test with a representative handoff: can a fresh agent explain the product
   or system outcome at risk, place the change, enumerate its surfaces, and
   name the starting source, allowed actions, required proof, and stop
   condition? Include a docs-only change: the guide should let it skip
   runtime proof.

## Grade Effects

- Missing purpose, ownership, working model, or starting route caps Legibility
  at D when it affects the task class.
- A concise entrypoint with valid task-shaped pointers can reach C.
- B requires versioned acceptance, lifecycle, authority, proof map, and
  write-back contracts with current links and commands.
- A requires a maintenance loop that turns recurring failures, review feedback,
  model changes, and operational drift into simpler guidance or mechanical
  enforcement.
- A polished guide cannot raise Executability or Feedback when its commands do
  not work; documentation claims become evidence only after you exercise the
  declared path.

Route prose compression or restructuring that does not change autonomous
execution capability to documentation-focused cleanup.
