# Set Up Automated Release Pipeline for npm Library

## Problem/Feature Description

Fieldstone Labs maintains `@fieldstone/form-validator`, a TypeScript library published to npm. A developer currently runs `npm version`, pushes a tag, and publishes by hand. Two releases shipped without a changelog update, and one local publish used stale dependencies.

The team wants to automate this using GitHub Actions and semantic-release, so that every conventional commit pushed to `main` that warrants a release (feat, fix, or breaking change) automatically: runs the test suite, bumps the version, updates the changelog, publishes to npm through npm Trusted Publishing/OIDC, creates a GitHub Release, and commits the version bump back to the repo. They want protection against two releases accidentally racing each other, and they want the version bump commit to never retrigger CI.

The organization requires verified signatures on `main`. The release
Environment provides `RELEASE_APP_CLIENT_ID` and `RELEASE_APP_PRIVATE_KEY` for
an installed GitHub App that can write this repository. The source writeback
must use GitHub's App-signed commit path; a configured bot name or noreply email
is not a signature. A superseded-run preflight and Actions concurrency are not
an atomic branch lock. Use plugin v1.0.1 only if the solution also names and
uses a concrete external branch lease that blocks every merge and direct push
from before semantic-release starts release analysis through the plugin's API
ref update. Otherwise use a full-SHA-pinned App-signed API integration that
sends the analyzed SHA as its expected head and fails closed on mismatch. If
plugin v1.0.1 is selected, immediately restore `origin` to a credential-free
URL in an `if: always()` step.

The organization also enforces immutable GitHub Releases. This package has no
post-publication release assets, so semantic-release may publish the metadata-
only release directly. The workflow must then prove `immutable: true`, run
`gh release verify`, resolve and peel the remote release tag, and require that
commit to be the verified App-signed writeback. Read `package.json` and
`CHANGELOG.md` from that immutable commit for npm parity; check the live default
branch separately only to prove it contains the writeback commit. Retries must
inspect the existing release and registry state instead of
creating another bump or trying to mutate a published release. Include a
validated backfill path for an existing trusted tag when npm or the GitHub
Release was published but the other boundary is missing; a normal
semantic-release rerun is not sufficient recovery.

## Output Specification

Produce the following files in the workspace:

- `.github/workflows/ci.yml`: the complete GitHub Actions workflow with verify and release jobs
- `.releaserc.json`: the semantic-release configuration file

Both files should be ready to commit to the repo root. You may create a `package.json` stub if needed to illustrate the configuration, but it is not required.

## Input Files

The following files represent the current state of the repository. Extract them before beginning.

=============== FILE: package.json ===============
{
  "name": "@fieldstone/form-validator",
  "version": "2.3.1",
  "description": "TypeScript form validation library",
  "main": "dist/index.js",
  "types": "dist/index.d.ts",
  "scripts": {
    "build": "tsc",
    "test": "vitest run",
    "lint": "eslint src",
    "typecheck": "tsc --noEmit",
    "verify": "npm run lint && npm run typecheck && npm run test && npm run build"
  },
  "devDependencies": {
    "typescript": "^5.4.0",
    "vitest": "^1.6.0",
    "eslint": "^8.57.0"
  },
  "repository": {
    "type": "git",
    "url": "git+https://github.com/fieldstone/form-validator.git"
  },
  "publishConfig": {
    "access": "public"
  }
}
=============== END FILE ===============

=============== FILE: CHANGELOG.md ===============
# Changelog

## 2.3.1

- Existing release history.
=============== END FILE ===============

=============== FILE: .node-version ===============
24
=============== END FILE ===============
