# Cut the Wait on a Single-Package Verify Workflow

## Problem/Feature Description

A single-package TypeScript library runs the workflow below on every pull
request and push to `main`. The `verify` gate is the required check.
Contributors complain that a one-line fix waits over a minute for three
jobs whose commands finish in seconds. Nobody has proposed sharding; there
is one test file per module and the whole suite runs in four seconds.

```yaml
name: verify
on:
  pull_request:
  push:
    branches: [main]
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: ${{ github.event_name == 'pull_request' }}
permissions:
  contents: read
jobs:
  lint:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@<sha> # v5
        with:
          fetch-depth: 0
      - uses: pnpm/action-setup@<sha> # v4
      - uses: actions/setup-node@<sha> # v4
        with:
          node-version-file: .node-version
          cache: pnpm
      - run: pnpm install --frozen-lockfile
      - run: pnpm lint
  typecheck:
    runs-on: ubuntu-24.04
    steps: # identical checkout, pnpm, node, and install, then `pnpm typecheck`
  test:
    runs-on: ubuntu-24.04
    steps: # identical checkout, pnpm, node, and install, then `pnpm test`
  audit:
    runs-on: ubuntu-24.04
    steps: # identical checkout, pnpm, node, and install, then `pnpm audit --audit-level high`
  verify:
    if: always()
    needs: [lint, typecheck, test, audit]
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@<sha> # v5
      - run: node scripts/check-results.mjs '${{ toJSON(needs) }}'
```

Median timings from the last 30 successful runs, per job: runner start 2s,
checkout 1s, pnpm and Node setup with the cache 12s, install 9s. Work:
lint 3s, typecheck 6s, test 4s, audit 2s. The `verify` gate itself takes
14s, of which 12s is its own runner start and checkout. Repository history
is 400 commits; a full-history checkout of it takes 2s.

## Output Specification

Produce:

- `.github/workflows/verify.yml`: the reworked workflow with every action
  pinned to a full commit SHA and a version comment. Keep every check, the `verify` required-check name, the
  concurrency contract, and read-only permissions.
- `ci-plan.md`: for each change, the measurement that justifies it and what
  it removes from the required path; state explicitly which Linear-style
  techniques were considered and rejected for this repository and why.
