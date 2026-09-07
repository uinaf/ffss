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

A build does not prove a browser flow. A screenshot does not prove an
end-to-end transition. Green CI does not prove a provider, account, device, or
deployed endpoint unless that exact surface ran.

## Task Instruments

Name the instrument missing from a proof claim. Build it before iterating when
improvement is authorized; during inspection, report the gap and its owner.

| Claim | Instrument |
| --- | --- |
| faster | benchmark harness with a recorded baseline on representative input |
| matches a design or reference | repeatable capture (screenshot, render, output dump) plus direct inspection of the rendered properties, compared against the source until no discrepancy remains |
| bug fixed | a regression test that fails before the fix or on revert |
| feature works | checks of the changed contract, including relevant failure paths |
| behavior preserved | the same behavioral or contract checks before and after the refactor |
| documentation corrected | checks against the owning source, links, or rendered output as applicable |
| lower cost, size, or token count | per-unit measurement on a real sample workload, with candidates that shrink the billed unit itself |
| best of several approaches | fixture set plus a scoring script that sweeps every candidate |

For optimization, record a representative baseline before editing, identify
the bottleneck and hypothesis, and attribute the measured change. Use a scored
sweep only when choosing among competing approaches. Report failed hypotheses
as well as gains. Reusable instruments belong in the existing task graph;
one-task experiments stay in attempt-scoped scratch.

## Repository Contract

Use the repository-owned verification surface shared by local work and CI.
For changed-code proof, prefer its affected lanes; expand for shared inputs,
uncertain coverage, or an explicit owner requirement. Repeat passing checks
only after relevant changes, failures, or unresolved concerns. The entrypoint
may be a manifest script, build task, framework command, or typed CLI; it
needs no wrapper file. Make it:

- run noninteractively with a finite bound
- preserve a primary failure or signal status and concise, inspectable output;
  make cleanup or absence-verification failure non-zero after primary success
- exercise the strongest cheap surface appropriate to the repository
- distinguish repository failures from missing runner capabilities
- release owned resources on every exit path per the ownership protocol in
  [setup-patterns.md](setup-patterns.md)
- emit task-and-attempt-scoped artifacts when evidence must survive the process

Don't create a parallel agent-only verification wrapper. Improve the ordinary
command contributors and CI already use.

## Real-Surface Evidence

Choose the smallest check set that can honestly disprove the claim:

- UI: navigate the changed flow, inspect interaction and console state,
  capture a labeled screenshot only as supporting evidence.
- API or service: start the real process, send representative success and
  error requests, inspect response plus structured logs.
- CLI: invoke the shipped or packaged entrypoint with representative arguments
  and inspect exit code, stdout, and stderr.
- State or config: prove write/read round trips, restart behavior, and invalid
  configuration handling.
- Deploy wiring: exercise the actual configured surface when the claim extends
  beyond local health.

Capture both the action and the resulting state; a command transcript without
its observed outcome is not real-surface evidence. Prefer integration,
contract, smoke, and end-to-end checks over mock-heavy unit tests at the seam
being claimed; mocked tests remain supporting evidence.

## Failure Quality

Exercise at least one representative failure when the task class touches
input, IO, authentication, network, configuration, or external dependencies.
Record the expected off-path behavior. Require:

- a non-zero or explicitly failed terminal state
- a stable error class, code, or machine-readable status when appropriate
- enough context to identify the failed boundary without exposing secrets
- a useful recovery action when the user or operator can act
- preserved artifacts or logs for unattended diagnosis

Swallowed errors, vague success, raw secret output, unbounded waits, cleanup
that only runs on success, global teardown that destroys unowned resources,
and owned resources left running without an explicit handoff are readiness
failures.

## Reporting

Report outcomes, not command theater:

- name the exact surface and revision exercised
- summarize passing checks by intent and result
- include exact commands and relevant output for failure reproduction
- label unavailable proof as unverified and name the missing repository or
  runner capability
- grade final state and side effects rather than trusting an agent's
  completion message

If the repository already provides this contract, use it during ordinary work;
don't start readiness work just to repeat the builder's final checks.
