# Agent Guidance Readiness

Audit `AGENTS.md` as an executable operating contract, not a style sample. A
guide is ready when a capable agent can identify the owner, choose the right
scope, act within authority, run the lifecycle, and report evidence from a
cold start without guessing.

## Canonical Shape

For the shape of the guide itself, follow
[../../slopscriber/references/agent-first.md](../../slopscriber/references/agent-first.md):
the single authored source with a `CLAUDE.md` symlink or import, what belongs
at the root versus behind task-shaped pointers, the working model a product
repository needs, and abstract preferences translated into decisions. Do not
restate those rules here; audit against them. This reference adds only what
readiness checks on top: owner context, the cross-model contract, the audit
procedure, and grade effects.

## Human and Owner Context

In a private owner workspace, a short human introduction improves judgment.
State the person's work, priorities, collaboration style, and a useful
recurring failure mode, then point to the canonical private or public profile
sources. Treat the introduction as a compass, not a synthetic persona or
complete biography.

In a shared product repository, orient around the product, users, maintainer
contract, and ownership boundaries. Do not add personal biography merely to
make the guide sound friendly.

Keep sensitive identity, credentials, finance, machine state, and private
workspace facts behind task-relevant pointers, so an agent opens the smallest
source needed instead of loading every personal facet at session start.

## Cross-Model Contract

Write shared guidance as observable behavior that works across capable model
families:

- define the deliverable, scope, authority, evidence, and terminal condition
- say when to act and when a decision genuinely needs the user
- require current evidence for progress and completion claims
- keep communication preferences concrete and non-repetitive
- state a recurring failure only when it changes the desired behavior
- preserve room for model judgment inside mechanically enforced boundaries

- Do not stack model-specific prompt fragments in the shared guide.
- Keep model selection, reasoning effort, verbosity, tool policy, and
  harness-specific invocation controls in their owning configuration.
- When a named-model guide is required, verify its current first-party
  prompting documentation and model or system card; record only a measured,
  stable difference that cannot live in the harness.

## Audit Procedure

1. Read the root guide and every pointer required for the requested task class.
2. Resolve named commands through the tracked task owners, delegated scripts,
   nested packages, and hidden CI configuration; check what each gate covers.
3. Compare inspection or improvement authority with the command's prerequisites,
   credentials, cost, and state changes. Inspect runtime ownership and teardown
   before execution; a familiar command name does not establish a safe scope.
4. Exercise the authorized proof path on the declared runner. Report unavailable
   or unsafe paths as gaps; inspection does not authorize bootstrap or repairs.
5. Trace one expected failure from command to surfaced diagnostic and recovery.
6. Identify duplication, contradictions, volatile claims, hidden prerequisites,
   and facts that live only in chat or a person's memory.
7. Test the guide with a representative task handoff: can a fresh agent explain
   the product or system outcome at risk, place the change, enumerate its relevant
   surfaces, name the starting source, allowed actions, required proof, and stop
   condition?

## Grade Effects

- Missing purpose, ownership, product or system working model, or starting route caps
  Legibility at D when that information affects the task class.
- A concise entrypoint with valid task-shaped pointers can reach C.
- B requires versioned acceptance, lifecycle, authority, proof, and write-back
  contracts with current links and commands.
- A requires a maintenance loop that turns recurring failures, review feedback,
  model changes, and operational drift into simpler guidance or mechanical
  enforcement.
- A polished guide cannot raise Executability or Feedback when its commands do
  not work. Treat documentation claims as evidence only after you exercise the
  declared path.

Route prose compression or restructuring that does not change autonomous
execution capability to documentation-focused cleanup.
