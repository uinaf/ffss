# Release Troubleshooting

Use only after a concrete release failure. Durable tag, release, registry,
default-branch, signature, tap, and deployment state outrank workflow status.

## No or Wrong Version

- **No release:** inspect commits since the last reachable tag and dry-run the
  repo-pinned release launcher. Non-release commit types are normally no-ops.
- **Wrong version:** verify full history, reachable tags, analyzer rules, and
  matching analyzer/notes presets.
- **Recursive bump:** the writeback and every trigger path must use the same
  repository skip convention.

## Writeback or Tag Failure

- Confirm the selected GitHub App is installed for the repo, has the required
  content scope, and is allowed by effective branch rules.
- A token used by ordinary Git transport does not make a commit signed. Use the
  release tool's App-native signed-writeback path or a reviewed API path that
  preserves the required tree semantics.
- If logs name `github-actions[bot]`, find the remaining persisted checkout
  credential or legacy local push path.
- Race or dangling-tag failures require one non-cancellable release critical
  section plus an atomic expected-head/ref check where the tool mutates live
  branch or tag state.

## Partial Publication

If a tag or source bump exists but a registry, asset, release, tap, or deploy
step failed:

1. Read the exact durable state of every target.
2. Do not create another version, delete an immutable release, or assume a
   normal rerun resumes after an existing tag.
3. Use the state-specific backfill contract in
   [release workflows](release-workflows.md#partial-failure-recovery).
4. Re-prove parity after recovery.

If the exact tag says a Release should already exist, use an authenticated
exact-tag lookup and fail closed on lookup errors. Add a bounded retry only for
demonstrated transient visibility lag. In explicit backfill, distinguish a
confirmed not-found response from authentication, rate-limit, network, and
server failures; only confirmed absence may create the missing Release.

When Actions artifact quota blocks deploy after successful publication, remove
the temporary artifact dependency and promote from the durable release asset,
registry, image digest, or provider-native package.

## Target-Specific Checks

- **npm auth/403:** verify trusted-publisher owner, repository, workflow, and
  Environment match the actual job; require OIDC and a supported Node/npm
  toolchain. Use a narrow Environment token only when trusted publishing is
  unavailable. Public scoped packages need the correct access and repository
  metadata.
- **CocoaPods duplicate:** the version may already exist. Reconcile trunk state;
  never blindly republish the same version.
- **GoReleaser dirty tree:** find generated files written before release and
  keep outputs outside the source tree or use the repo's clean-release mode.
- **Unsigned Homebrew tap update:** use the selected GoReleaser version's
  GitHub App-native commit path, omit identity overrides that defeat signing,
  and read back signature verification.
- **Marketplace major tag stale:** update the intentional mutable major pointer
  only after selecting the highest eligible immutable stable release. Use an
  expected-old-OID compare-and-swap and verify the peeled remote target.
- **Immutable asset replacement:** a published release is not a scratch area.
  Skip mutation and resume only missing downstream parity, deploy, or smoke
  checks.

Exact action inputs and plugin options change. Inspect the repository-pinned
major and its upstream documentation before diagnosing an option-name error.
