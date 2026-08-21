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

## Fast Portable Execution

The repository owns verification. CI providers only provision a checkout,
restore safe caches, inject scoped runner capabilities, select repository-owned
lanes, invoke them, and aggregate their results. GitHub Actions, GitLab CI,
Buildkite, local agents, and developer shells must not carry separate validation
logic.

- Extend the existing task graph or manifest before adding a runner or wrapper.
  For heterogeneous repositories, `mise` tasks with explicit dependencies and
  sources are a suitable existing surface; use another established project
  runner when it already owns the graph.
- Give independent checks separate tasks and run their dependency graph in
  parallel. Keep output attributable to each lane and preserve every failure.
  Tasks that write the same generated directory, compiler state, or local cache
  are not independent; isolate that state or encode an ordering edge.
- Select affected work from explicit inputs. Exercise added, modified, deleted,
  renamed, staged, unstaged, and untracked cases that the local contract claims
  to support. When local freshness cannot safely represent one case, retain a
  forced full command and make merge-diff CI authoritative for it.
- Include the task-graph definition itself in each cached lane's inputs. A
  changed command, dependency edge, source map, or cache policy must invalidate
  every lane whose meaning changed. Include runtime and toolchain pin files in
  every cached lane they can affect.
- Treat task-runner source/output freshness as an optimization, not as
  Git-equivalent affected detection. Preserved timestamps, deletion, and rename
  handling must be proven before freshness can represent those cases.
- Keep affected-input policy in one repository-owned map. If every CI adapter
  cannot consume that map, run the exhaustive repository gate in CI instead of
  copying path filters into provider configuration.
- Keep one exhaustive, non-cached path. Selection and caches optimize proof;
  they never redefine which changes require proof.
- Model generated artifacts as outputs of one task and dependencies of every
  consumer. A dependency edge may only order execution; unless the runner
  explicitly includes dependency hashes, each cached consumer must also
  fingerprint the generated files it reads. Prove the graph once without
  pre-existing generated output; a warm checkout can hide a missing edge behind
  stale files.
- Separate ephemeral working state from persistent download and build caches.
  Key provider, compiler, dependency, and generated-tool caches by the relevant
  lockfile or source revision plus operating system, architecture, and toolchain
  when those affect compatibility. A cache miss must remain correct.
- Exclude `.git`, dependency directories, build output, generated state, and
  large binaries from filesystem scans unless the scanner's policy explicitly
  owns them. Do not narrow detectors or disable verification merely to improve
  timing.
- Check task-runner install behavior in clean CI. Some runners auto-install every
  declared tool before one selected task; either cache that installation once or
  disable task auto-install and install the selected lane's declared tools.
- Treat environment tracking as cache policy. Fingerprint variables that can
  change results, and explicitly leave shell bookkeeping such as `SHLVL` and
  package-manager launch metadata such as `INIT_CWD` untracked only after proving
  they do not affect output. Otherwise identical work in a new shell becomes a
  false cache miss.
- With Vite+ task graphs, keep package-script caching disabled unless every
  script is pure. A global `run.cache: true` also caches deploy, publish, and
  migration scripts; prefer the default task-only cache or opt safe tasks in
  individually.
- Do not share caches from untrusted change execution with privileged deploy,
  publish, signing, or secret-bearing jobs.
- Keep policy application, deployment, release, migration, and live acceptance
  exhaustive unless their owning contract independently proves safe selection.

Record four timings after a material change: unchanged selection, one relevant
change, warm full verification, and cold full verification. Also report the
slowest lane and distinguish task time from provisioning, tool installation,
cache restore, and runner queue time. Optimize measured ownership boundaries,
not the workflow's total duration by guesswork.

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
