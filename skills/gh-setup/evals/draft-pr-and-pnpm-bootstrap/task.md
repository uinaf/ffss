# Keep Draft Pull Requests Out of pnpm CI

## Problem

A repository's `.github/workflows/verify.yml` listens for pull-request activity types `opened`, `synchronize`, `reopened`, and `ready_for_review`. The maintainer expected that list to suppress draft pull requests, but opening or pushing to a draft still starts the verify job.

The repository has moved to pnpm 11 and declares Node through
`devEngines.runtime`, but its workflow still carries the older two-action
bootstrap. `actions/setup-node` asks for the pnpm store before
`pnpm/action-setup` has installed pnpm, so clean runners fail. The maintained
`pnpm/setup` successor can install the declared pnpm and Node runtime, restore
the pnpm store cache, and optionally run the install itself.

Make the smallest safe workflow correction. Draft pull requests must not
execute verification jobs, marking a PR ready must start verification, and the
pnpm 11 bootstrap and cache must work on a clean runner. Replace the legacy
two-action bootstrap with `pnpm/setup` pinned to the full commit for v2.0.2.
Keep the workflow on the unprivileged `pull_request` event.

## Input Files

The following files represent the current repository state. Extract them before beginning.

=============== FILE: .github/workflows/verify.yml ===============
name: Verify

on:
  pull_request:
    types: [opened, synchronize, reopened, ready_for_review]

permissions:
  contents: read

jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
      - uses: actions/setup-node@48b55a011bda9f5d6aeb4c2d9c7362e8dae4041e # v6.4.0
        with:
          node-version: 24
          cache: pnpm
      - uses: pnpm/action-setup@d15e628ca66d93ee5f352c71671a7bc6a97af5c9 # v6.0.8
        with:
          run_install: false
      - run: pnpm install --frozen-lockfile
      - run: pnpm test
=============== END FILE ===============

=============== FILE: package.json ===============
{
  "private": true,
  "packageManager": "pnpm@11.21.0",
  "devEngines": {
    "runtime": {
      "name": "node",
      "version": "24",
      "onFail": "download"
    }
  },
  "scripts": {
    "test": "vitest run"
  }
}
=============== END FILE ===============

## Output

Produce the corrected `.github/workflows/verify.yml` and a short explanation of why both fixes are necessary.
