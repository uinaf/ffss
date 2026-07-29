# Adding Enforceable React Guardrails

## Problem/Feature Description

A React application uses oxlint locally and GitHub Actions for pull requests.
Agents frequently introduce effect, accessibility, and architecture problems.
The team has a globally installed React guidance skill, but it is not invoked
reliably and cannot block a pull request. The repository's current `verify`
script runs oxlint and tests, while CI runs `npm run verify`.

Set up deterministic React Doctor coverage. Keep the existing linter and
canonical verification command, establish a safe baseline, and add a pull
request check that can be tightened after the team trusts the findings. Explain
whether the globally installed agent skill is required for enforcement.

## Output Specification

Produce the smallest coherent change set:

1. Update `package.json` and the lockfile with repo-owned React Doctor tooling.
2. Update `.oxlintrc.json` so React Doctor rules run through the existing lint
   command.
3. Update the canonical local gate only if the lint integration does not cover
   the required project-level checks.
4. Add or update `.github/workflows/react-doctor.yml` for pull requests.
5. Add `react-readiness.md` describing the baseline, rollout, permissions, and
   the role of optional agent instructions.

## Input Files

=============== FILE: package.json ===============
{
  "name": "react-dashboard",
  "private": true,
  "scripts": {
    "lint": "oxlint src",
    "test": "vitest run",
    "verify": "npm run lint && npm run test"
  },
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  },
  "devDependencies": {
    "oxlint": "^1.0.0",
    "vitest": "^3.0.0"
  }
}

=============== FILE: .oxlintrc.json ===============
{
  "rules": {
    "no-console": "error"
  }
}

=============== FILE: .github/workflows/verify.yml ===============
name: Verify
on: pull_request
permissions:
  contents: read
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@<full-sha>
        with:
          fetch-depth: 0
      - uses: actions/setup-node@<full-sha>
        with:
          node-version-file: .node-version
          cache: npm
      - run: npm ci
      - run: npm run verify
