# Releases

## Next release

- Provider processes retain bounded private stderr diagnostics without treating
  large non-fatal hook or progress output as `provider output_limit`.
- Codex CLI `0.153.2`, Claude Code `2.1.260`, Cursor Agent
  `2026.09.02-c22c1a3`, and Grok Build `1.0.13` are covered by compatibility
  fixtures.
- `slopguard doctor --json` is an alias for `--output json`.

`slopguard` evaluates a CLI release after every successful push to protected
`main`. Conventional Commits determine whether that evaluation publishes a
version; merges with no consumer-facing release type stop without a tag.

## Published artifacts

Each CLI release contains:

- macOS archives with Developer ID signed and notarized binaries for amd64 and
  arm64;
- Linux archives for amd64 and arm64;
- a SHA-256 `checksums.txt` manifest;
- a keyless Cosign bundle for that manifest; and
- GitHub build-provenance attestations for the archives, manifest, and bundle.

- The checksum signature protects every archive named by the manifest.
- GitHub attestations independently bind each uploaded artifact to the release
  workflow.
- macOS binaries are signed with hardened runtime and a secure timestamp
  before Apple accepts their notarization submissions.
- Their Apple signing ID is `slopguard`; managed execution controls can combine
  that ID with the expected Apple Team ID.
- Apple creates tickets for standalone binaries but does not support stapling
  tickets to them, so Gatekeeper retrieves the ticket online.

## Installer trust boundary

The installer script and the release tag, matching archive, and
`checksums.txt` it obtains all come over HTTPS from GitHub. The checksum
detects a corrupt or mismatched archive, but because installer, archive, and
checksum share one transport and hosting boundary, it is not independent
provenance verification. The installer never modifies shell startup files or
uses privilege escalation.

For independent verification, download the artifacts and use the Cosign
signature and GitHub build attestation workflow below before installing the
binary.

## Self-update

- `slopguard selfupdate` replaces the installed binary with the newest
  published release (or a pinned `--release vX.Y.Z`), verifying the platform
  archive against the release's `checksums.txt`.
- `--check` reports without touching the binary and exits 8 when an update is
  available.
- It shares the installer's trust boundary (checksum and archive over one
  HTTPS hosting boundary); use the Cosign and attestation workflow below for
  independent provenance.
- Homebrew-managed installs are refused: use `brew upgrade --cask slopguard`.
- Non-release builds refuse to self-update.

## Verify a release

Download one archive plus the manifest and signature bundle from the matching
GitHub Release, then verify the workflow identity and checksum:

```bash
archive=slopguard_v0.1.2_darwin_arm64.tar.gz

cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity \
    "https://github.com/uinaf/ffss/.github/workflows/release-slopguard.yml@refs/heads/main" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  checksums.txt

grep "  ${archive}$" checksums.txt | shasum -a 256 -c -
gh attestation verify --owner uinaf "$archive"
```

Replace the example archive version and target with the release being checked.

## Automation boundary

Pull-request jobs receive read-only repository access and no release secrets.
The main-only release job enters the protected `release` Environment only after
macOS/Linux verification and snapshot packaging pass. It mints a short-lived
`uinaf-releaser` installation token scoped to `slopguard` and `homebrew-tap`
with Contents write permission.

- The protected `release` Environment stores the Developer ID certificate,
  certificate password, and notary private key as secrets, and the Apple
  issuer, key, and Team IDs as environment variables.
- GitHub injects the three secrets only into the main-only publication step;
  pull requests cannot access them.
- The identifier variables also enter that step; the Team ID is reused by
  post-publication signature verification.
- Before publication, the credential verifier checks certificate trust, the
  matching signing private key, and identifiers, and rejects a certificate
  outside the Team ID pinned by `APPLE_TEAM_ID`.

- Semantic Release owns version selection, release notes, the Git tag, and a
  mutable draft GitHub Release.
- GoReleaser adopts that draft, Developer ID signs both macOS binaries, waits
  for Apple to accept both notarization submissions, uploads all archives, the
  checksum manifest, and the Sigstore bundle, then updates the Homebrew cask.
- The workflow verifies those outputs and provenance before publishing the
  draft; publication makes the release assets and tag immutable.
- Exact-tag release discovery fails closed instead of skipping publication.
- No release commit is pushed to `main`.

If publication fails after the tag is created, rerunning the failed workflow is
safe: a release tag at `HEAD` resumes the mutable draft without choosing a new
version. If publication succeeded and only the downstream Homebrew smoke
failed, the rerun detects the published release and skips every mutating
release step. Never delete or move a published tag to retry a release.

## Version tracks

CLI SemVer describes the Go executable contract. The skill ships with the
repo's agent plugin, carries no separate version, and documents its compatible
CLI requirement.

## Skill publication

The skill ships through this repo's agent-plugin marketplace
(`.claude-plugin/marketplace.json`): merging skill changes to `main` is the
publication, and installed plugins are SHA-versioned by the marketplace commit.
The root CI skills-lint job (`skillcheck lint`, via `tools/skill-evals`) gates
the package shape.
