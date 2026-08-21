# Fast Runner-Neutral Infrastructure Verification

## Problem

An infrastructure repository uses OpenTofu, shell checks, and TruffleHog. Local
verification is a custom `scripts/verify-changed.sh` program that parses Git
state and runs commands serially. GitHub Actions and GitLab CI each duplicate a
different subset of those commands. A clean local run takes 20 seconds, OpenTofu
downloads providers into a disposable data directory every time, and the secret
scanner walks `.git` and generated provider binaries.

Make verification fast for developers and agents without binding its contract
to a CI vendor. Preserve the existing `mise` tool owner. CI systems may select
lanes and restore caches, but local work, GitHub Actions, and GitLab CI must call
the same repository-owned tasks.

Keep OpenTofu backend isolation, credential handling, secret verification,
policy application, deployment, and live acceptance unchanged and exhaustive.

## Required Output

Produce:

1. `mise.toml` with explicit parallel verification lanes, affected inputs, and
   canonical fast and forced-full commands.
2. Thin `.github/workflows/verify.yml` and `.gitlab-ci.yml` adapters that invoke
   those tasks without copying validation policy.
3. Persistent provider and pinned-source build caches separated from disposable
   OpenTofu working state.
4. Updated scanner exclusions that omit `.git` and generated provider state
   without weakening detectors or verification.
5. Removal of `scripts/verify-changed.sh`.
6. `verification-results.md` containing unchanged, relevant-change, warm-full,
   cold-full, and slowest-lane measurements. This prompt provides no executable
   checkout, so label timings unverified instead of inventing them; include the
   exact benchmark cases to run against the implemented repository.

Do not add another task runner, a Git-aware shell selector, or CI-specific
validation commands. Cache misses must remain correct.
