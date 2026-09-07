# Autonomy Evidence

Readiness infrastructure and observed autonomy are different claims. This
scale shows how strongly you exercised a repository and runner grade.

## Trace the unattended workflow

Trace the applicable stages of
`triage → dispatch → provision → execute → prove → submit → reconcile → complete`.
Record input, output, owner, and terminal condition at each stage, including
recovery to retry, escalation, or failure. No-diff QA declares its result,
evidence, target, and allowed side effects; it need not create a branch.

For repeated trials, record task class, scenario, result, human interventions,
duration, retries, failure class, and artifacts as JSON. Aggregate success,
intervention, duration, resource, retry, and failure metrics for autonomy
claims; exercise parallel isolation and crash/stall recovery where claimed.

## Evidence Levels

E0 through E2 are defined in [grading.md](grading.md); this file covers
designing and reporting the top two.

### E3: Representative

Run a small suite of representative tasks over multiple trials with outcome
graders. Report success rate, human interventions, duration, retries, resource
or cost class, and failure taxonomy. Start with real recurring work and
failures; expand the suite as changes become harder to distinguish.

### E4: Operational

Long-running or parallel work has survived real stalls, crashes,
cancellations, credential denial, CI or review feedback, and recovery. Collect
this over time, track its freshness, and turn failures into harness or eval
updates.

## Representative Task Design

Choose tasks from the automation path and actual workload, for example:

- triage an issue into an owned task with unambiguous acceptance criteria
- bootstrap a cold isolated workspace on the declared runner
- implement and prove a narrow bug fix on a real surface
- QA an existing build and submit a manifest of screenshots, recordings, logs,
  traces, JSON output, and scenario results without a code change
- handle missing or denied credentials without exposing values
- produce a branch or PR handoff and respond to a failing CI check
- submit a QA report and reconcile requested reruns or acceptance feedback
- resume after process loss without repeating unsafe side effects
- run two tasks concurrently without resource or delivery collisions

Cover the task classes the grade claims; a dependency bump does not generalize
to UI work, incident response, schema migration, or release automation.

## Graders

Grade final state first:

- repository diff, tests, build artifact, database state, or provider state
- required and forbidden side effects
- branch, PR, CI, review, QA submission, acceptance, or terminal task state
- artifact presence, manifest entries, provenance, hashes, secret redaction,
  and required formats

Use transcripts for efficiency, policy, and diagnosis: turns and tool calls;
retries, stalls, and recovery decisions; human questions or approvals; token,
duration, and resource or cost class.

An agent's completion statement is not proof. Keep tasks
implementation-agnostic unless the implementation is the contract. Give every
task an unambiguous specification and a known-valid solution or setup that
proves the grader can pass.

## Reliability Profile

Report the smallest useful set:

```text
task suite: 6 task classes, 3 trials each
trial success: 15/18
consistent success: 4/6 tasks passed every trial
human interventions: 1 credential-scope escalation
duration: p50 18m, p95 44m
recovery: 2/2 injected stalls recovered
false success: 0
evidence: runner image, model, harness revision, date
```

Use `pass@1` for the probability that one attempt succeeds and `pass^k` when
every one of `k` attempts must succeed; name the metric rather than an
ambiguous "pass rate." For long-horizon work, report both a typical and a
higher-reliability threshold when enough trials exist.

## Eval Maintenance

- Seed the suite from manual release checks, bug reports, review failures, and
  automation incidents.
- Inspect failed and suspiciously easy transcripts; distinguish agent failure
  from an ambiguous task or broken grader.
- Reject hidden implementation requirements the task contract never stated.
- Keep reference solutions and verify they still pass after toolchain changes.
- Add harder or broader tasks before the suite saturates.
- Track evidence age; rerun after material model, harness, runner, dependency,
  or repository architecture changes.
