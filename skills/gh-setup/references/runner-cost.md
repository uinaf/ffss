# Runner Cost

Runner minutes are billed compute except on standard GitHub-hosted runners
for public repositories, where only latency and the concurrent-job cap cost
([billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)).
Every trigger, runner size, and rerun is a cost or latency decision; default to
the cheapest shape that still proves the contract.

- Follow the target owner's runner policy. Compare live pricing, included
  minutes, repository visibility, and runner availability before choosing a
  provider or size. Preserve each job's required OS and architecture; reusable
  workflows must support the caller's policy and platform needs.
- Use Linux for portable checks. macOS and other large runners are reserved for
  platform-bound jobs (native apps, Darwin-only APIs, Homebrew taps) and are
  gated at job level or restricted to `pull_request` + `workflow_dispatch`;
  macOS bills about ten times Linux on GitHub-hosted runners and twenty
  2-vCPU minutes per minute on Blacksmith, whose smallest macOS size is 6 vCPU
  ([rates](https://docs.github.com/en/billing/concepts/product-billing/github-actions),
  [Blacksmith](https://docs.blacksmith.sh/blacksmith-runners/overview)). Runner changes preserve required proof, scan coverage,
  triggers, permissions, and Environments.
- Public repositories stay on standard GitHub-hosted runners, where minutes are
  free. Private repositories draw on the owner's included minutes; once those
  are spent, GitHub-hosted jobs run as paid usage, and GitHub refuses to
  dispatch them when no budget covers the overage or a payment has failed.
  ([billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions));
  the owner's runner policy says whether they move to a third-party runner. A
  repository declaring a non-GitHub runner label lists it under
  `self-hosted-runner.labels` in `.github/actionlint.yaml`
  ([actionlint](https://github.com/rhysd/actionlint/blob/main/docs/config.md)).
- Provider requirements override that policy: npm trusted publishing supports
  GitHub-hosted runners only
  ([npm](https://docs.npmjs.com/trusted-publishers)), so a repository on
  third-party runners keeps that job GitHub-hosted. Use the
  [npm publish contract](release-targets.md#npm).
- Scans follow the [security baseline](security-baseline.md): steps at the
  end of the existing `verify` job on push. Keep required-check names stable;
  a job skipped by `if:` reports success, while a workflow skipped by path
  filters leaves a required check pending.
- Every verification workflow declares workflow-level concurrency:
  `group: ${{ github.workflow }}-${{ github.ref }}`,
  `cancel-in-progress: ${{ github.event_name == 'pull_request' }}`. Release,
  publish, and deploy critical sections keep their own non-cancellable keys.
- A workflow triggered on both `push: [main]` and `pull_request` pays twice per
  merged change. Where direct pushes are allowed, `verify` runs on both, since
  the push run is the only check a direct push gets and carries the scan. Keep
  every other job on one trigger, and let a push-triggered release that calls
  `verify` through `workflow_call` stand in for a separate push trigger.
- Jitter cron minutes away from the top of the hour, where GitHub drops queued
  scheduled jobs under load
  ([schedule](https://docs.github.com/en/actions/reference/workflows-and-actions/events-that-trigger-workflows)).
  A schedule names its threat and weekly minute cost.
- Expensive-per-run jobs (simulators, cross-compiles, e2e) sit behind
  `dorny/paths-filter` lanes or `workflow_dispatch`, with an `always()` result
  job that fails on any result other than success or an expected skip when
  branch protection needs a stable check.
- Watch failure rates: a failed run bills its full minutes and a rerun bills
  them again
  ([billing](https://docs.github.com/en/billing/concepts/product-billing/github-actions)).
  Fix or gate flaky jobs instead of rerunning them.

## Critical Path

The slowest chain of required jobs sets the wait. Where a change-detection
job gates other jobs, its duration is paid on every run
([Linear](https://linear.app/blog/ci-bottleneck-reworked), gate 94s to 20s).
In a small repository the verify job is the whole chain, and runner start,
checkout, and toolchain install are most of it; measure that share before
choosing a technique below.

- Fetch only what the job reads. Verification, lint, and build jobs keep the
  default depth of one
  ([actions/checkout](https://github.com/actions/checkout/blob/main/README.md));
  full history is for release version analysis, signed writeback, and manual
  history scans. The saving is proportional to history size, so on a small repository
  it is seconds.
- A paths filter on `pull_request` events lists files through the API, so the
  job needs `pull-requests: read` and no checkout step; on `push` events it
  fetches the base commit by SHA into a default-depth checkout, and on
  `merge_group` events it needs a checkout and reads the event SHAs
  ([paths-filter](https://github.com/dorny/paths-filter/blob/ceb8a2b8f2d89434be7ff52d3de7ec3738c5cc9d/README.md)).
  That fetch runs without credentials when the checkout persists none, so a
  private repository in that state keeps `fetch-depth: 0` on every non-PR
  event. The [uinaf changes action](https://github.com/uinaf/.github/blob/main/.github/actions/changes/action.yml)
  packages these rules; private callers pass `full-history: "true"`. The API
  returns at most 3,000 files and the action reports no truncation
  ([REST](https://docs.github.com/en/rest/pulls/pulls?apiVersion=2022-11-28)),
  so a job that skips lanes adds a catch-all `'**'` filter, compares its
  count with the pull request's `changed_files`, and runs everything when
  they differ.
- In a monorepo, affected-package detection checks out with
  `filter: blob:none` and a small `fetch-depth`, then deepens until the merge
  base resolves; a clone that is too shallow marks every package changed
  ([Turborepo](https://turborepo.dev/docs/reference/run)), and `fetch-depth: 2`
  is not enough in the general case.
- A job whose work is shorter than the measured runner start, checkout, and
  install on its runner shape is a candidate to merge into a sibling job on
  the same runner and trust level, with the tasks run concurrently. The
  overhead is real but unpublished by GitHub; measure it per runner shape
  ([Depot](https://depot.dev/blog/reducing-queue-time-with-cached-schemas),
  runner init p99 39s). Concurrent tasks share one runner's cores and memory,
  so compare the batched job with the parallel jobs before keeping it, and
  say whether latency or runner minutes is the target. Run the batched tasks
  concurrently through a runner that collects every exit status (the package
  manager's parallel run, or a tested repository script), not as sequential
  steps and not as backgrounded commands joined by a bare `wait`. Keep separate jobs for
  different runners, trust boundaries, or multi-minute work.
- Write setup and install seconds to `$GITHUB_STEP_SUMMARY` on every required
  job, so cache, batching, and shard decisions rest on recorded numbers.
- Measure caches before keeping them. Record hit or miss, restore, install,
  and save seconds in the step summary; a dependency cache stays only when its
  expected cost from those numbers beats always installing cold. Published
  numbers conflict by an order of magnitude
  ([BuildPulse](https://buildpulse.io/blog/github-actions-cache-optimization-benchmarks),
  [Linear](https://linear.app/blog/ci-bottleneck-reworked)). Try the
  package-manager store cache first; pnpm itself says it is not guaranteed
  faster ([pnpm](https://pnpm.io/continuous-integration)). Keep a
  `node_modules` cache only where the arithmetic favours it. In a monorepo,
  measure a filtered install of the affected packages against restoring
  everything.
- When sharding, wall time cannot drop below one setup plus the largest
  shard, and every added shard bills one more setup; Vitest and Playwright
  split by file or test count, never by duration
  ([Vitest](https://vitest.dev/guide/cli.html),
  [Playwright](https://playwright.dev/docs/test-sharding)). State the measured
  setup time and both figures before proposing shards, and cut setup first
  when it dominates.
- Work that gates nothing (cache markers, coverage upload, summaries,
  notifications) moves to a job after the required check when latency is the
  target; the second job pays another runner start
  ([GitHub](https://github.blog/engineering/infrastructure/making-github-ci-workflow-3x-faster/),
  deferred compliance).
- Speedups that reduce test isolation (shared module state, reused containers,
  skipped teardown) are opt-in per project or file with written eligibility
  rules ([Vitest isolate](https://vitest.dev/config/isolate.md)), and the
  test-writing guidance carries those rules so new tests comply by default.

Read [merge queue](merge-queue.md) only when the repository runs one.
