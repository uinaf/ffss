# Verification Contract

Readiness owns the infrastructure that lets builders and evaluators prove work.
It does not replace the ordinary responsibility to self-check each completed
change, and it does not make an independent ship decision.

## Proof Layers

Keep these claims separate:

| Layer | What it proves |
| --- | --- |
| deterministic guardrails | the checkout passes its repository-owned static and test gates |
| focused regression | the changed behavior and an adjacent failure or edge case were exercised |
| real surface | the shipped CLI, API, browser flow, worker, device, or simulator actually worked |
| CI | configured remote checks passed for the relevant revision |
| live or deploy | the configured production-like surface was exercised |

A build does not prove a browser flow. A screenshot does not prove an end-to-end
transition. Green CI does not prove a provider, account, device, or deployed
endpoint unless that exact surface ran.

## Task Instruments

A builder that must ask a human to check its work is missing an instrument;
treat the request as a readiness gap, name the missing tool, and provide it
before iterating.

| Claim | Instrument |
| --- | --- |
| faster | benchmark harness with a recorded baseline on representative input |
| matches a design or reference | repeatable capture (screenshot, render, output dump) plus direct inspection of the rendered properties, compared against the source until no discrepancy remains |
| correct behavior | a test that fails before the fix and fails again on revert |
| lower cost, size, or token count | per-unit measurement on a real sample workload, with candidates that shrink the billed unit itself |
| best of several approaches | fixture set plus a scoring script that sweeps every candidate |

- Measure the baseline before the first edit; without one, "improved" is an
  adjective.
- Diagnose before editing: rank the measured bottlenecks and write a
  cause-and-fix hypothesis for each.
- Outside a scored sweep, change one variable at a time and attribute each
  gain; report before/after numbers.
- Commit each verified improvement separately, so a failed hypothesis reverts
  cleanly and wins survive on their own.
- When no best path is obvious, define the quality score first, sweep real
  samples, and pick by score; report losing candidates and failed hypotheses,
  not only the winner
  ([laboratory pattern](https://brianlovin.com/writing/give-your-agent-a-laboratory-pt-ii-KjFnCW9)).
- Refine the winning parameter to the quality frontier: push until the score
  degrades and keep the last value that holds it.
- Build, preview, fixture, rollback, and observation paths are readiness
  capabilities; grade their absence.
- An instrument contributors and CI will reuse belongs in the repository's
  task graph; a one-task lab stays attempt-scoped scratch, never committed
  leftovers.

## Repository Contract

Prefer one repository-owned verification entrypoint reused by local work and CI.
That entrypoint may be a manifest script, build task, framework command, or
typed CLI; it does not need a wrapper file.
It should:

- run noninteractively with a finite bound
- preserve a primary failure or signal status and concise, inspectable output;
  make cleanup or absence-verification failure non-zero after primary success
- exercise the strongest cheap surface appropriate to the repository
- distinguish repository failures from missing runner capabilities
- release owned resources on every exit path per the ownership protocol in
  [setup-patterns.md](setup-patterns.md)
- emit task-and-attempt-scoped artifacts when evidence must survive the process

Do not create a parallel agent-only verification wrapper. Improve the ordinary
command used by contributors and CI.

## Real-Surface Evidence

Choose the smallest check set that can honestly disprove the claim:

- UI: navigate the changed flow, inspect interaction and console state, capture
  a labeled screenshot only as supporting evidence.
- API or service: start the real process, send representative success and error
  requests, and inspect response plus structured logs.
- CLI: invoke the shipped or packaged entrypoint with representative arguments
  and inspect exit code, stdout, and stderr.
- State or config: prove write/read round trips, restart behavior, and invalid
  configuration handling.
- Deploy wiring: exercise the actual configured surface when the claim extends
  beyond local health.

- Real-surface proof exercises the real user path and captures both the action
  and the resulting state; a command transcript without its observed outcome
  is not real-surface evidence.
- Prefer integration, contract, smoke, and end-to-end checks over mock-heavy
  unit tests at the seam being claimed.
- Mocked tests remain useful supporting evidence.

## Failure Quality

Exercise at least one representative failure when the task class touches input,
IO, authentication, network, configuration, or external dependencies. Prefer a
worst-case input over a mild one, and record the intended off-path behavior,
not only that a failure occurred. Require:

- a non-zero or explicitly failed terminal state
- a stable error class, code, or machine-readable status when appropriate
- enough context to identify the failed boundary without exposing secrets
- a useful recovery action when the user or operator can act
- preserved artifacts or logs for unattended diagnosis

Swallowed errors, vague success, raw secret output, unbounded waits, cleanup
that only runs on success, global teardown that destroys unowned resources, and
owned resources left running without an explicit handoff are readiness failures.

## Reporting

Report outcomes, not command theater:

- name the exact surface and revision exercised
- summarize passing checks by intent and result
- include exact commands and relevant output for failure reproduction
- label unavailable proof as unverified and name the missing repository or
  runner capability
- grade final state and side effects rather than trusting an agent's completion
  message

If a repository already provides this contract, use it during ordinary work;
do not invoke readiness work merely to repeat the builder's final checks.
