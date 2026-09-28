# Actions Security

Use when a workflow executes project code or loads publish, signing, deploy, or
other privileged credentials.

## Trust Boundary

- Never use `pull_request_target` to check out, install, build, test, package,
  or otherwise execute pull-request code.
- Fork and pull-request jobs use `pull_request`, read-only permissions, and no
  delivery secrets.
- Secret-bearing work runs only on trusted branches, protected tags, or a
  validated manual dispatch.
- Manual inputs are validated in a secretless job, refs resolve to one immutable
  SHA, and downstream jobs consume only sanitized outputs.

## Permissions and Credentials

Default workflow permissions to `contents: read` or `{}`. Grant write, OIDC,
attestation, or pull-request permissions only to the job that needs them.
Monitoring and notification jobs stay read-only.

Use `persist-credentials: false` through checkout, install, build, pack, and
test in privileged workflows. Add write credentials only at the narrow delivery
boundary. Fetch full history only when tags, history, or affected detection
requires it.

## Dependencies, Caches, and Logic

- Pin high-trust remote Actions to reviewed full SHAs and keep an automated
  update path. Repository-level SHA enforcement is useful only after the
  current allowlist and updater contract are understood.
- Never write a SHA you have not read from the source. Without network access,
  leave an explicit placeholder such as `owner/action@<sha> # v4.2.1` naming the
  exact version, and list resolving it as a required follow-up
  (`gh api repos/<owner>/<action>/commits/<tag> --jq .sha`).
- Run `actionlint`, `zizmor`, and appropriate secret scanners. Use supported
  configuration instead of shell glue that merely silences them. Keep zizmor
  at 1.28.0 or newer: 1.27.0 logs its parsed config, `GH_TOKEN` included,
  under verbose output (GHSA-f42p-wjw5-97qh).
- When the owner maintains many repositories, ship the scan once as a
  composite action in the owner's `.github` repository, with every image and
  Action digest-pinned there, and call it as the last step of each
  repository's `verify` job ([security baseline](security-baseline.md)).
  Where full-SHA pins are enforced, tag releases there and pin callers to the
  release commit with the tag as the version comment (`@<sha> # v1.2.0`); a
  branch annotation such as `# main` fails zizmor's `ref-version-mismatch`
  audit once the branch moves.
- Never share package caches from untrusted pull requests with privileged
  publish, signing, release, or deploy jobs.
- Keep workflow YAML orchestration-thin. Prefer maintained Actions and the
  repository's existing typed validation/task surfaces. When custom parsing,
  ref policy, summaries, provider branching, or security-sensitive logic is
  unavoidable, use a tested typed module or local action with explicit inputs
  and outputs. Do not grow inline shell or move the same spaghetti into a new
  `.sh` file; shell may only dispatch a few already-defined commands.
- Workflow files carry only pin annotations (`# v1.2.3`) and scanner
  suppressions (`# zizmor: ignore[...]`, `# shellcheck disable=...`).
  Rationale lives in the README or the owning doc.

## Payloads and Artifacts

Actions artifacts are temporary same-run storage, not a durable release or
recovery boundary. A later run or recoverable deploy should consume an
immutable GitHub Release asset, registry version, image digest, provider-native
package, or signed archive with checksum/provenance.

Do not rebuild after verification unless the provider is intentionally the
builder and records equivalent provenance. Keep secret-bearing delivery jobs
non-cancellable and make retries reconcile durable state.
