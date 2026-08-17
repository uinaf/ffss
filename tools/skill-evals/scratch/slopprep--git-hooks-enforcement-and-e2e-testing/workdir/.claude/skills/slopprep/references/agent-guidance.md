# Agent Guidance Readiness

Audit `AGENTS.md` as an executable operating contract, not a style sample. A
guide is ready when a capable agent can identify the owner, choose the right
scope, act within authority, run the lifecycle, and report evidence from a cold
start without guessing.

## Canonical Shape

Keep one authored `AGENTS.md`. When a harness expects another filename, symlink
or import the canonical guide instead of maintaining parallel prose.

The root guide should stay short and progressively disclose deeper contracts:

1. **Orientation** — what this repository or workspace is for, who or what it
   serves, and where implementation belongs.
2. **Routing** — task-shaped pointers that say when to read each deeper source.
3. **Authority** — safe local actions, required approvals, destructive or
   external boundaries, and credential ownership.
4. **Workflow** — the smallest bootstrap, boot, verify, and teardown path plus
   where task acceptance comes from.
5. **Proof** — the canonical gate, task-relevant real surface, failure evidence,
   and what cannot be inferred from local checks.
6. **Write-back** — where durable decisions, operational state, and handoffs
   belong.

Do not duplicate architecture tours, API references, generated trees, complete
style manuals, or volatile inventories. Link them with a task or question that
justifies opening them.

## Product or System Working Model

A shared product repository needs more than routing. Give a fresh agent enough
of the system model to place a change and predict its blast radius:

- what the product does, who relies on it, and the outcomes that must not regress
- non-negotiable qualities paired with known failure modes or expected proof
- domain terms whose everyday meanings would cause ambiguity
- dangerous operations, live-state boundaries, and safe alternatives
- recurring change dimensions such as clients, providers, entry points,
  connection modes, contracts, or reverse states
- one compact architecture or data-flow explanation when it changes where code
  belongs

Inline only cross-cutting facts. Keep complete architecture, API, and subsystem
manuals behind task-shaped pointers. The goal is not a standard section list;
it is enough causal context for the agent to make the right local decision.

Abstract preferences are not operational until they predict behavior. Replace
`keep it simple`, `protect performance`, or `build clean UI` with a decision
rule, counterexample, known regression, or required observation. A short
maintainer note earns its place when it resolves real trade-offs rather than
adding personality alone.

[T3 Code's repository guide](https://github.com/pingdotgg/t3code/blob/main/AGENTS.md)
is one concrete example: product priorities are tied to regression modes, a
bounded coverage matrix, live-state hazards, architecture placement, and exact
proof rather than left as slogans.

## Human and Owner Context

For a private owner workspace, a short human introduction can improve judgment.
State the person's work, priorities, collaboration style, and a useful recurring
failure mode, then point to the canonical private or public profile sources.
Treat the introduction as a compass, not a synthetic persona or complete
biography.

For a shared product repository, orient around the product, users, maintainer
contract, and ownership boundaries. Do not add personal biography merely to make
the guide sound friendly.

Keep sensitive identity, credentials, finance, machine state, and private
workspace facts behind task-relevant pointers. An agent should open the smallest
source needed rather than loading every personal facet at session start.

## Cross-Model Contract

Write shared guidance as observable behavior that works across capable model
families:

- define the deliverable, scope, authority, evidence, and terminal condition
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

1. Read the root guide and every pointer required for the requested task class.
2. Resolve every named path and command against the checkout.
3. Compare declared authority with actual repository, runner, and credential
   boundaries.
4. Exercise the documented cold-start and proof path on the declared runner.
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
  not work. Documentation claims are evidence only after the declared path is
  exercised.

Use documentation-focused cleanup for prose compression or restructuring that
does not change autonomous execution capability.
