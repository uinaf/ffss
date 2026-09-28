# Automate Binary Releases for a Go CLI Tool

## Problem/Feature Description

Redwood Systems ships `vaultctl`, a Go command-line tool for secrets rotation used by their infrastructure teams. The project has grown from an internal tool to one adopted by a handful of partner companies, and the team wants to provide polished distribution: pre-built binaries for Linux/macOS/Windows, a Homebrew cask so Mac users can simply `brew install`, and signed build attestation for supply-chain compliance.

Releases depend on someone remembering to cut one. The process does not maintain a consistent changelog or build binaries, and the Homebrew cask in the separate `redwood-systems` tap repository is months out of date. Conventional commits on `main` should determine the version, create a GitHub Release, build cross-platform binaries, and update the cask without a manual release command.

The cross-repo Homebrew update needs credentials beyond the default GitHub
token, which already covers source-repository Release operations. Mint a
short-lived GitHub App installation token scoped only to the tap and use
GoReleaser's native GitHub App commit-author support so that update is signed
without granting the tap publisher source write access or adding a second
commit job. Both repositories require verified commits. The team also wants
immutable GitHub Releases and build provenance attestation for the assets.

Release recovery must be state-specific: create or resume the mutable draft for
the exact trusted tag when publication is incomplete, but skip every asset
mutation when that Release is already published and immutable. A missing draft
must not make an existing tag unrecoverable, and every recovery rereads parity.
Enumerate every tag pointing at `HEAD`, filter by the configured stable release
tag format, and require exactly one eligible tag; do not let `git describe`
choose among multiple tags. Pass that selected exact tag as
`GORELEASER_CURRENT_TAG` to every GoReleaser invocation so a co-located tag
cannot redirect publication. Resolve and peel the remote tag, require its commit
OID to equal `HEAD` before any backfill or build, and reread the same remote OID
immediately before immutable publication and again at completion. Protect the
release-tag namespace against updates/deletion and restrict creation to the
release actor. Resolve the remote ref through a tested repo-owned helper using
the source-repository workflow token and authenticated GitHub Git Refs/Tags
APIs; the checkout keeps persisted credentials disabled. The separate tap App
token must never be used for source reads or writes.
Use an authenticated exact-tag lookup that can see drafts. When the current run
already created or identified an expected draft, lookup failure must fail closed
rather than reporting absence or success. Add a bounded visibility retry only
if the design demonstrates a transient lookup-lag requirement. In explicit
recovery, accept only an unambiguous not-found response as a missing Release;
authentication, authorization, rate-limit, network, and server failures remain
errors. Backfill a confirmed missing Release from the trusted tag and reread
exact state; create conflict or duplicate exact-tag state must not become a
green no-op.
Reject a prerelease for the stable tag. Before attestation or publication,
require the resumed draft's complete asset names and SHA-256 digests to equal
the current build manifest; missing, extra, or mismatched assets fail closed.

## Output Specification

Produce the following files:

- `.github/workflows/ci.yml`: complete GitHub Actions workflow with verify and release jobs
- `.releaserc.json`: semantic-release configuration
- `.goreleaser.yaml`: GoReleaser configuration including Homebrew cask automation
- `scripts/resolve-remote-tag-oid` and focused tests: authenticated annotated-tag peeling with fail-closed errors

Include a brief `SETUP.md` at the repo root documenting the release Environment App credentials (`RELEASE_APP_CLIENT_ID` / `RELEASE_APP_PRIVATE_KEY`) and the explicit repository scope required for the minted token.

## Input Files

The following files represent the current repository state. Extract them before beginning.

=============== FILE: go.mod ===============
module github.com/redwood-systems/vaultctl

go 1.22
=============== END FILE ===============

=============== FILE: Makefile ===============
.PHONY: verify build

verify:
	go vet ./...
	go test ./...
	golangci-lint run

build:
	go build -o bin/vaultctl ./cmd/vaultctl
=============== END FILE ===============
