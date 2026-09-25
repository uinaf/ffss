# Verification Contract

You own the infrastructure for proving work. That does not replace
self-checking each completed change or grant an independent ship decision.

## Proof Layers

Keep these claims separate:

| Layer | What it proves |
| --- | --- |
| deterministic guardrails | the checkout passes its repository-owned static and test gates |
| focused regression | the changed behavior and an adjacent failure or edge case were exercised |
| real surface | the shipped CLI, API, browser flow, worker, device, or simulator actually worked |
| CI | configured remote checks passed for the relevant revision |
| live or deploy | the configured production-like surface was exercised |

Green CI does not prove a provider, account, device, or deployed endpoint
unless that exact surface ran.

## Task Instruments

Name the instrument missing from a proof claim. Build it before iterating when
improvement is authorized; during inspection, report the gap and its owner.
A human asked to confirm by eye ("still feels fast", "looks right") is a
missing instrument. Non-obvious cases:

| Claim | Instrument |
| --- | --- |
| matches a design or reference | repeatable capture plus direct inspection of the rendered properties, compared against the source until no discrepancy remains |
| lower cost, size, or token count | per-unit measurement on a real sample workload, with candidates that shrink the billed unit itself |
| best of several approaches | fixture set plus a scoring script that sweeps every candidate |

Reusable instruments belong in the existing task graph; one-task experiments
stay in attempt-scoped scratch.

## Repository Contract

Use the repository-owned verification surface shared by local work and CI.
For changed-code proof, prefer its affected lanes; expand for shared inputs,
uncertain coverage, or an explicit owner requirement. The entrypoint may be a
manifest script, build task, framework command, or typed CLI; it needs no
wrapper file. Make it:

- run noninteractively with a finite bound
- preserve a primary failure or signal status; make cleanup or
  absence-verification failure non-zero after primary success
- distinguish repository failures from missing runner capabilities
- release owned resources on every exit path per the ownership protocol in
  [setup-patterns.md](setup-patterns.md#runtime-resource-ownership)
- emit task-and-attempt-scoped artifacts when evidence must survive the process

Don't create a parallel agent-only verification wrapper. When slow or
platform-specific lanes run on a remote runner, the same command runs there;
declare the runner and its credential boundary in the repository guide's
[proof map](agent-guidance.md#proof-map).

## Failure Quality

Exercise at least one representative failure when the task class touches
input, IO, authentication, network, configuration, or external dependencies:
non-zero terminal state, stable error class or code, the failed boundary
without secrets, a recovery action, preserved logs.

Swallowed errors, cleanup that only runs on success, global teardown that
destroys unowned resources, and owned resources left running without an
explicit handoff are readiness failures.

If the repository already provides this contract, use it during ordinary work;
don't start readiness work just to repeat the builder's final checks.
