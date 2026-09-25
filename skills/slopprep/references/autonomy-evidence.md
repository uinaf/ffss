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
duration, retries, failure class, and artifacts as JSON. Exercise parallel
isolation and crash/stall recovery where claimed.

## Evidence Levels

E0 through E2 are defined in [grading.md](grading.md); this file covers
designing and reporting the top two.

- **E3, representative:** a small suite of representative tasks from the real
  workload over multiple trials with outcome graders. Report success rate,
  human interventions, duration, retries, resource or cost class, and failure
  taxonomy.
- **E4, operational:** long-running or parallel work has survived real stalls,
  crashes, cancellations, credential denial, CI or review feedback, and
  recovery. Track its freshness and turn failures into harness or eval updates.

Cover the task classes the grade claims; a dependency bump does not generalize
to UI work, incident response, schema migration, or release automation. Include
denied-credential handling, resume after process loss without repeating unsafe
side effects, and two concurrent tasks when those capabilities are claimed.

Grade final state and required or forbidden side effects; use transcripts only
for efficiency, policy, and diagnosis. Give every task a known-valid solution
that proves the grader can pass.

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
ambiguous "pass rate." Rerun after material model, harness, runner,
dependency, or repository architecture changes.
