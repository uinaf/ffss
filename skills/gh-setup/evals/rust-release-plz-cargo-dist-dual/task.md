# Automate a Dual crates.io and Binary Rust Release

## Problem/Feature Description

Keystone Systems ships `glyph`, a Rust crate that is both a library on crates.io
and a cross-platform CLI distributed through an immutable GitHub Release and a
Homebrew formula. Use release-plz for the Release PR, checked-in Cargo version
and changelog, crates.io publication, and version tag. Use cargo-dist's
generated tag workflow as the sole GitHub Release and binary-asset owner.

The release-plz configuration must disable its GitHub Release publisher while
keeping tag creation enabled. Both release-plz jobs must use a short-lived,
repository-scoped GitHub App token so the automated Release PR receives CI and
the created tag actually triggers cargo-dist; the default repository
`GITHUB_TOKEN` is insufficient for those follow-up workflow events. crates.io
uses trusted publishing/OIDC rather than a long-lived registry token.

The source and Homebrew tap default branches require verified commits.
Cargo-dist generates the formula but must not use its unsigned built-in
Homebrew publisher. A custom post-announce reusable workflow verifies the
published immutable Release and commits only the formula through a pinned
App-signed API action. Recovery reconciles crates.io, tag, GitHub Release,
assets, and tap state without republishing immutable boundaries. If a published
immutable Release has an incomplete asset set, fail closed and roll forward to
a new version rather than attempting asset repair.
The tap write stages only the generated formula, uses the observed tap checkout
parent as the atomic expected head, and fails if the tap advances.

## Output Specification

Produce:

- `.github/workflows/release-plz.yml` for Release PR and crates.io/tag release
- `release-plz.toml`
- `dist-workspace.toml` and cargo-dist's generated tag workflow
- `.github/workflows/publish-homebrew.yml` with workflow-call and validated
  manual recovery entrypoints
- `SETUP.md` documenting trusted publishing plus separately minted source-only
  release-plz and tap-only Homebrew App tokens

## Input Files

=============== FILE: Cargo.toml ===============
[package]
name = "glyph"
version = "1.3.0"
edition = "2024"

[lib]
path = "src/lib.rs"

[[bin]]
name = "glyph"
path = "src/main.rs"
=============== END FILE ===============

=============== FILE: Cargo.lock ===============
# Existing committed lockfile.
=============== END FILE ===============

=============== FILE: CHANGELOG.md ===============
# Changelog

## 1.3.0

- Existing release history.
=============== END FILE ===============
