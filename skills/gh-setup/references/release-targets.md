# Publish Targets

Load only the selected target. Shared trust, immutable-publication, recovery,
and completion rules live in [release-workflows.md](release-workflows.md).

## Target Matrix

| Target | Version owner | Publish boundary | Required proof |
| --- | --- | --- | --- |
| npm package | semantic-release, changesets, or repo-selected manager | npm trusted publishing or scoped token fallback | packed contents, registry version, provenance, tag/Release parity |
| SwiftPM | Git tag | trusted Git tag | resolved package revision and release-tag parity |
| CocoaPods | prepared podspec plus Trunk | CocoaPods token | Trunk version plus tag, podspec, and Release parity |
| Go binary | semantic-release or repo-selected tag owner plus GoReleaser | draft GitHub Release, then publish | complete assets, checksums, attestation, immutable Release |
| Rust library | release-plz or repo-selected Cargo manager | crates.io trusted publishing | crate version, tag, provenance, changelog or manifest parity |
| Rust binary | cargo-dist or equivalent single asset owner | draft GitHub Release | generated workflow, installers, complete immutable assets |
| GitHub Action | release tag plus moving major pointer | Git refs and Release | committed bundle, immutable version tag, monotonic major tag |
| Homebrew | source publisher plus tap writer | signed tap commit | formula/cask digest, audit, verified tap commit |

Do not combine two version managers or two GitHub Release owners for the same
artifact.

## npm

Use GitHub-hosted Actions for npm trusted publishing, including private
repositories: [npm trusted publishing supports GitHub-hosted runners
only](https://docs.npmjs.com/trusted-publishers/), so the release job stays
GitHub-hosted in repositories that otherwise run on Blacksmith. Configure the package
for the exact repository, workflow file, and Environment; grant
`id-token: write`; remove `NPM_TOKEN`. Use a granular package-scoped token on
the release Environment only when trusted publishing is unavailable.

Before enabling automation, prove the package already exists or perform the
explicitly authorized one-time bootstrap publication. Verify `npm pack` output,
public scoped-package access, repository metadata, CLI `bin` contents, and the
published registry version.

## SwiftPM and CocoaPods

SwiftPM publishes through the tag. CocoaPods adds a separate immutable Trunk
boundary. Prepare version files deterministically, sign any protected-branch
writeback through the selected API path, then publish the podspec with the
Environment-scoped token.

If only one boundary succeeds, backfill the missing boundary from the exact
trusted tag. Never republish an existing pod version or create a second bump as
generic retry behavior.

## Go and GoReleaser

Use one tool to choose/create the tag and GoReleaser to build the complete
asset set. Keep the GitHub Release as a draft while GoReleaser uploads; compare
the exact draft asset names and digests with the current build manifest before
publishing.

Bind every GoReleaser invocation to the selected exact tag. Resolve and peel
the remote tag, require it to match the intended commit, and reread it before
immutable publication. If GoReleaser updates Homebrew, prefer its native
GitHub-App commit path and omit custom author or committer fields.

## Rust

Choose one version owner:

- Binary without crates.io: a tag/version manager prepares the Cargo version
  and tag; cargo-dist alone creates the GitHub Release and assets.
- Library or crates.io distribution: release-plz owns Cargo versions and
  publication. For dual binary distribution, disable its GitHub Release
  creation and let cargo-dist own assets.

Prefer crates.io trusted publishing when available. Do not mix release-plz
with a semantic-release Cargo writeback. Commit `Cargo.lock` for binaries unless
the repository has an explicit contrary contract.

## Homebrew

Treat the tap as a separate signed-write destination. The source workflow token
cannot write a sibling tap. Prefer an organization-owned GitHub App and mint a
tap-only installation token.

Use the publisher's native GitHub-App commit support when available. Otherwise
generate the formula or cask deterministically and use a narrow API commit that
checks the tap head observed before generation. Read back signature
verification and run the tap's applicable `brew audit` path.

Compute URLs and checksums from the exact immutable source release. A tap update
is downstream reconciliation and must be repairable without mutating that
release.

## GitHub Actions

Build and commit the action bundle during the pull request. Verification should
delete or rebuild the generated surface and fail on any changed, missing,
stale, or untracked output before tagging.

Version tags are immutable; a major tag such as `v1` is an intentionally
mutable compatibility pointer. Update it only to the highest eligible published
stable release in that major, using peeled Git-ref commit identities and an
expected-old-or-absent compare-and-swap. Reread the pointer after mutation.

## Monorepos

Choose coordinated or independent versions deliberately. Independent packages
need collision-free tag formats and per-package working directories. For
coordinated releases, use the repository's established workspace release tool
instead of parallel semantic-release jobs invented during GitHub setup.
