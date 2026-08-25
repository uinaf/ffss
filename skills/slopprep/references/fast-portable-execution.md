# Fast Portable Execution

The repository owns verification. CI providers only provision a checkout,
restore safe caches, inject scoped runner capabilities, select
repository-owned lanes, invoke them, and aggregate results. Never let a
provider, local agent, or developer shell carry separate validation logic.

## Task graph

- Extend the existing task graph or manifest before adding a runner or
  wrapper. For heterogeneous repositories, `mise` tasks with explicit
  dependencies and sources are a suitable surface; use another established
  runner when it already owns the graph.
- Keep each command, dependency set, and test-file list in one live task
  owner. Compatibility scripts delegate to that task; contract tests inspect
  or execute the task the graph actually uses, never an unused wrapper.
- Give independent checks separate tasks and run them in parallel, output
  attributable per lane, every failure preserved. Tasks that write the same
  generated directory or cache are not independent; isolate that state or
  encode an ordering edge.
- Model generated artifacts as outputs of one task and dependencies of every
  consumer. Unless the runner hashes dependencies, each cached consumer must
  also fingerprint the generated files it reads. Prove the graph once without
  pre-existing generated output; a warm checkout hides missing edges behind
  stale files.

## Selection and caching

- Select affected work from explicit inputs, and exercise every change case
  the local contract claims to support, including deletions, renames, and
  untracked files. When freshness cannot safely represent a case, keep a
  forced full command and make merge-diff CI authoritative for it.
- Treat task-runner freshness as an optimization, never as Git-equivalent
  affected detection.
- Keep affected-input policy in one repository-owned map. When a CI adapter
  cannot consume it, run the exhaustive gate there instead of copying path
  filters into provider configuration.
- Keep one exhaustive, non-cached path. Selection and caches optimize proof;
  they never redefine which changes require proof.
- Include the task-graph definition and runtime and toolchain pins in the
  inputs of every cached lane they can affect.
- Key persistent caches by the relevant lockfile or source revision plus
  operating system, architecture, and toolchain; a cache miss must remain
  correct. Separate ephemeral working state from those caches.
- Fingerprint environment variables that can change results. Leave shell
  bookkeeping such as `SHLVL` and `INIT_CWD` untracked only after proving
  they do not affect output; otherwise identical work in a new shell becomes
  a false cache miss.
- With Vite+ task graphs, keep package-script caching disabled unless every
  script is pure; a global `run.cache: true` also caches deploy, publish,
  and migration scripts.
- Never share caches from untrusted change execution with privileged
  deploy, publish, signing, or secret-bearing jobs.

## Guardrails

- Don't narrow detectors or disable verification just to improve timing.
- Check task-runner install behavior in clean CI: cache the auto-install
  once, or disable it and install the selected lane's declared tools.
- Keep policy application, deployment, release, migration, and live
  acceptance exhaustive unless their owning contract independently proves
  safe selection.
- Before pushing verification-only work, inspect automatic release
  classification; use a non-releasing commit type unless a product release
  is authorized.

Record four timings after a material change: unchanged selection, one
relevant change, warm full, and cold full. Report the slowest lane and
separate task time from provisioning, tool install, cache restore, and
runner queue time. Optimize measured ownership boundaries, not total
duration by guesswork.
