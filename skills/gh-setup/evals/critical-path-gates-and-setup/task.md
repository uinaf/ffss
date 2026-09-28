# Shorten the Required Check on a pnpm Monorepo

## Problem/Feature Description

A pnpm workspace with packages `api`, `web`, and `shared` runs the workflow
below on every pull request. The required check is the `verify` gate. Median
wait is nine minutes; the actual test time inside the `api-test` shards is
under three. Engineers want the wait cut without dropping any check.

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
  changes:
    runs-on: ubuntu-24.04
    outputs:
      api: ${{ steps.filter.outputs.api }}
      migrations: ${{ steps.filter.outputs.migrations }}
    steps:
      - uses: actions/checkout@<sha> # v5
        with:
          fetch-depth: 0
      - uses: dorny/paths-filter@<sha> # v4
        id: filter
        with:
          filters: |
            api: ['api/**', 'shared/**', 'pnpm-lock.yaml']
            migrations: ['api/migrations/**']
  lint:
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@<sha> # v5
        with:
          fetch-depth: 0
      - uses: pnpm/action-setup@<sha> # v4
      - uses: actions/cache@<sha> # v4
        with:
          path: node_modules
          key: nm-${{ hashFiles('pnpm-lock.yaml') }}
      - run: pnpm install --frozen-lockfile
      - run: pnpm -r lint
  typecheck:
    runs-on: ubuntu-24.04
    steps: # identical checkout, cache, and install, then `pnpm -r typecheck`
  format:
    runs-on: ubuntu-24.04
    steps: # identical checkout, cache, and install, then `pnpm format:check`
  licenses:
    runs-on: ubuntu-24.04
    steps: # identical checkout, cache, and install, then `pnpm licenses:check`
  schema-check:
    runs-on: ubuntu-24.04
    needs: changes
    if: needs.changes.outputs.migrations == 'true'
    steps: # identical checkout, cache, and install, then `pnpm --filter api schema:check`
  api-test:
    runs-on: ubuntu-24.04
    needs: [changes, lint, typecheck, format, licenses]
    if: needs.changes.outputs.api == 'true'
    strategy:
      matrix:
        shard: [1, 2]
    steps:
      - uses: actions/checkout@<sha> # v5
        with:
          fetch-depth: 0
      - uses: pnpm/action-setup@<sha> # v4
      - uses: actions/cache@<sha> # v4
        with:
          path: node_modules
          key: nm-${{ hashFiles('pnpm-lock.yaml') }}
      - run: pnpm install --frozen-lockfile
      - run: sudo apt-get update && sudo apt-get install -y postgresql-client
      - run: pnpm --filter api test -- --shard=${{ matrix.shard }}/2
      - run: pnpm --filter api coverage:upload
      - run: pnpm --filter api test:cache-marker write
  verify:
    if: always()
    needs: [changes, lint, typecheck, format, licenses, schema-check, api-test]
    runs-on: ubuntu-24.04
    steps:
      - uses: actions/checkout@<sha> # v5
      - run: node scripts/check-results.mjs '${{ toJSON(needs) }}'
```

`coverage:upload` publishes the coverage report to an external service and
`test:cache-marker write` records that this input set passed; neither changes
a test result, and a failure in either is reporting-only. The `api` tests use
`psql` through `postgresql-client` for fixture loading.

Recent run data: each of `lint`, `typecheck`, `format`, `licenses`, and
`schema-check` spends 55 to 70 seconds on runner start, checkout, cache
restore, and install, then 5 to 20 seconds working. The `node_modules` cache
restore averages 28 seconds; a cold `pnpm install --frozen-lockfile` averages
19 seconds and a cold `pnpm install --filter api...` averages 9 seconds. Each
`api-test` shard spends about 95 seconds before the first test runs. A
teammate has proposed going to eight shards.

## Output Specification

Produce:

- `.github/workflows/verify.yml`: the reworked workflow with every action
  pinned to a full commit SHA and a version comment. Keep every check,
  the `verify` required-check name, the concurrency contract, read-only
  permissions, and the lane condition for `api-test` and `schema-check`.
- `ci-plan.md`: for each change, the measurement that justifies it, what
  moved off the critical path, and the shard decision with its reasoning.
