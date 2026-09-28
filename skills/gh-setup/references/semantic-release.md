# Semantic Release

Use only after the repository has selected semantic-release and Conventional
Commits as its version contract. Inspect the repo-pinned tool and action
versions before changing configuration.

## Version Decision

- `feat` produces a minor, `fix` a patch, and an explicit breaking change a
  major unless repository release rules say otherwise.
- Analyzer and notes generator must use the same preset and release rules.
- Squash-merge repositories should validate PR titles; direct-push paths should
  validate commit subjects. Local hooks are feedback, not enforcement.
- Fetch full tag history in release jobs and in any verification job that
  runs the release dry-run; plain verification keeps the default depth.

Dry-run from the intended release branch before the first real publication and
inspect both the computed version and notes. Preserve the repository-owned
launcher and pinned versions rather than introducing `npx ...@latest`.

## Plugin Order

Order plugins by lifecycle:

1. analyze commits
2. generate notes
3. prepare versioned files or changelog
4. publish registries or assets
5. perform any signed source writeback
6. create or finalize the GitHub Release

A source-writeback plugin must run after every file-preparation plugin and
before the release tag is finalized. List writeback files explicitly. Do not
use a tree-limited API plugin for symlinks, executable files, deletions, or
generated trees it cannot faithfully preserve.

For protected branches, prefer the selected release tool's GitHub App-native
signed writeback. An App token used by ordinary `git commit` or `git push` does
not itself produce a verified signature. The App must also be allowed by the
effective branch rules.

## Concurrency and Branches

- Keep one non-cancellable release critical section per release branch.
- Do not assume Actions concurrency is an atomic branch lease. A tool that
  writes against the live branch head must reject unexpected head movement or
  run behind an external control that prevents it.
- Configure only branches and prerelease channels the repository actually
  publishes.
- A release writeback should not recursively trigger verification or another
  release; use the repository's established skip mechanism consistently.

## Publication and Recovery

Build and verify all assets before making an immutable GitHub Release public.
If the release tool creates the release early, keep it as a draft until assets
and downstream registries are ready.

Semantic-release is not a transaction across GitHub, registries, taps, or
deployments. After a partial failure, inspect tag, release, registry, default
branch, and signature state before retrying. A rerun may stop at an existing
tag and never invoke the failed publisher. Follow the durable-state recovery
rules in [release workflows](release-workflows.md#partial-failure-recovery).

Completion requires the peeled tag to resolve to the intended signed version
commit, the live default branch to contain it when source writeback is part of
the contract, every registry to expose the same version, and the GitHub Release
to have the intended immutable state and assets.
