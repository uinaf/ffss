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
- clean up owned processes, ports, test state, temporary credentials, and
  heavyweight runtimes such as simulators, emulators, virtual machines,
  containers, browsers, services, and databases on success, failure, timeout,
  and cancellation
- preserve pre-existing resources, record exact IDs for resources created or
  acquired by the attempt, track an owned process tree when a launcher can spawn
  descendants, and verify final state rather than trusting cleanup exit status
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

Prefer integration, contract, smoke, and end-to-end checks over mock-heavy unit
tests at the seam being claimed. Mocked tests remain useful supporting evidence.

## Failure Quality

Exercise at least one representative failure when the task class touches input,
IO, authentication, network, configuration, or external dependencies. Require:

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
