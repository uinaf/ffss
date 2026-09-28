# Release Workflows

Use for GitHub Actions that version, tag, sign, publish, or distribute an
immutable artifact.

## Workflow Shape

- Pull requests and default-branch pushes run the repository's verification
  contract with read-only credentials.
- Release runs only from a trusted default-branch or validated protected-tag
  event after verification.
- Manual release or backfill inputs are validated in a secretless job and
  resolve to one immutable SHA or trusted tag before checkout or credentials.
- Release concurrency is non-cancellable and serialized for the publication
  boundary. Verification concurrency may remain cancellable.
- Use one release-state owner. Multiple tools must not race to create the same
  tag or GitHub Release.

Keep `persist-credentials: false` through checkout, install, build, pack, and
test. Introduce a scoped write identity only at the exact tag, release,
registry, or signed-writeback boundary.

## Permissions and Secrets

Start with `permissions: {}` or `contents: read`. Add only the job scopes the
selected target requires:

- `contents: write` for tags, releases, or source writeback
- `id-token: write` for trusted publishing, provider OIDC, or keyless provenance
- `attestations: write` for file attestations
- issue or pull-request write only when the configured release tool uses it

Use an approval-free `release` Environment when it is only a scoped credential
boundary. Keep deployment records for running-service deploys and custom
deployment-protection apps.

## Immutable Publication

Published GitHub Releases lock their tags and assets. Assemble the complete
transaction before publication:

```text
build -> verify payload -> create or resume draft -> attach complete manifest
      -> verify checksums, signatures, and provenance -> publish once
```

If semantic-release chooses the version and notes before another tool uploads
assets, configure it to create a draft. If all assets already exist, one
publisher may create the draft, upload, and publish atomically. Never append or
replace assets after publication.

A recovery reads the exact tag and Release state first. When a tag implies that
a Release should exist, use an authenticated exact-tag lookup that can see
drafts. A failed expected lookup is an error, not a successful no-op. Add a
bounded visibility retry only when the repository has demonstrated transient
lookup lag. Published means mutation is over; continue only missing downstream
reconciliation.

## Signed Writeback

An App token authenticates a write but does not sign a local `git commit`.
Prefer the selected release tool's native GitHub API writeback that lets GitHub
sign the commit. Use a generic API commit only when no native path exists and
it preserves the full tree plus an expected-head compare-and-swap.

Reject superseded runs before release analysis. Workflow concurrency and a
head preflight do not form an atomic branch lease; a source writeback that
cannot compare the expected head needs a real external lease or a different
writeback implementation.

For semantic-release-specific plugin order, version files, and dry-run checks,
read [semantic-release.md](semantic-release.md).

## Completion Proof

One real release for each distinct workflow shape must prove every applicable
boundary:

- published, non-draft, immutable GitHub Release
- release verification and exact asset manifest
- peeled tag resolves to the intended commit
- protected-branch writeback is verified and contained in the live default branch
- version files at the release commit match the published version
- registry, tap, moving action tag, or deploy pointer references the same
  version and payload digest
- a retry reaches the same state without mutating published assets

No-release and dry-run paths leave publication and downstream parity unverified.

## Partial-Failure Recovery

Reconcile durable state before choosing a repair:

| Durable state | Repair boundary |
| --- | --- |
| prepared signed commit, no tag | validate parent, tree, version, and signature; create the missing tag and run only missing publishers |
| tag exists, Release missing | create from the trusted tag; use a draft for assets |
| registry exists, Release missing | backfill the Release; never republish the registry version |
| tag exists, registry missing | publish the exact tagged package through a validated backfill |
| draft has partial assets | repair the draft, verify the complete manifest, publish once |
| immutable Release exists, downstream missing | run idempotent downstream reconciliation keyed by the tag |

- Backfills validate inputs before secrets, reread every durable boundary
  after repair, and never create another version bump.
- In an explicit recovery path where a missing Release is a valid state,
  accept only an unambiguous not-found response as absence.
- Authentication, authorization, rate-limit, network, and server failures
  remain errors.
- Backfill from the already-trusted tag, then reread the exact Release.
- A create conflict or duplicate exact-tag state is reconciliation work, not
  success.

## Supply-Chain and Handoff

- Secret-bearing jobs install fresh by default and do not consume caches
  populated by untrusted pull requests.
- Build, package, sign, and publish from the trusted release commit or tag.
- Prefer registry versions, release assets, provider packages, or image digests
  over GitHub Actions artifacts as the release-to-deploy boundary.
- Add finite timeouts and stable final checks for conditional or matrixed paths.
- Keep policy parsing and recovery logic in an existing structured tool or a
  tested typed module/local action, not inline workflow blocks or ad-hoc shell.
