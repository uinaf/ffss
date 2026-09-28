# Add a Signed Homebrew Fallback for a Non-Go CLI

## Problem/Feature Description

Acme Tools publishes `envctl`, a non-Go CLI, and needs its release workflow to
update `acme-tools/homebrew-tap`. Its existing formula generator ends with an
ordinary local commit and push. The tap now requires verified commits, so a bot
name and noreply email are insufficient.

The generator has no native GitHub App-signed commit mode. Preserve its useful
formula-generation behavior, but prevent it from committing. After a release is
published, a dependent Linux job reads source state with the source
repository's read-only workflow token, then mints a separate short-lived App
token scoped only to the tap. It deterministically prepares only
`Formula/envctl.rb` and commits that path with the full-SHA-pinned
`pgaskin/push-signed-commits` action. Stage no other path, use the observed tap
checkout parent as the atomic expected head, and fail rather than overwrite if
the tap advances. Read back the resulting tap commit and fail if GitHub does
not report `verification.verified: true`.

The handoff must use durable state. It should run whenever the exact trusted
release tag is published and immutable but tap parity is missing, including a
later recovery run. Do not gate repair solely on semantic-release's
`new_release_published` output.

## Output Specification

Update `.github/workflows/release.yml` and write a short `SETUP.md`. Document
the release Environment's `RELEASE_APP_CLIENT_ID` variable and
`RELEASE_APP_PRIVATE_KEY` secret, the tap-only token scope, and
`contents: write`. Keep the source workflow token at `contents: read`. Do not
add a PAT, custom bot identity, ordinary `git push`, or a manual tap PR.

## Input Files

The current workflow publishes through semantic-release and exposes
`new_release_published` and `new_release_version` as job outputs. It currently
runs a formula generator directly after semantic-release using the default
repository token. Replace only that Homebrew handoff; preserve the existing
release system, but replace the transient output gate with exact release-state
discovery and an idempotent parity check.
