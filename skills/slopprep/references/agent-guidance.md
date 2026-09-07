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

## Cross-Model Contract

Write shared guidance as observable behavior that works across capable model
families:

- define deliverable, scope, authority, evidence, and terminal condition
- say when to act and when a decision genuinely needs the user
- require current evidence for progress and completion claims
- keep communication preferences concrete and non-repetitive
- state a recurring failure only when it changes the desired behavior
- preserve room for model judgment inside mechanically enforced boundaries

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
   condition?

## Grade Effects

- Missing purpose, ownership, working model, or starting route caps Legibility
  at D when it affects the task class.
- A concise entrypoint with valid task-shaped pointers can reach C.
- B requires versioned acceptance, lifecycle, authority, proof, and write-back
  contracts with current links and commands.
- A requires a maintenance loop that turns recurring failures, review feedback,
  model changes, and operational drift into simpler guidance or mechanical
  enforcement.
- A polished guide cannot raise Executability or Feedback when its commands do
  not work; documentation claims become evidence only after you exercise the
  declared path.

Route prose compression or restructuring that does not change autonomous
execution capability to documentation-focused cleanup.
