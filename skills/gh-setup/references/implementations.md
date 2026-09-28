# Maintained Implementations

Use only when creating or materially rewriting GitHub workflow code. These are
working public examples, not universal templates. Read each repository's guide,
manifest, scripts, and live GitHub settings; reuse the named contract while
adapting triggers, gates, identities, targets, and current dependency pins.
An example does not authorize adopting its owner's policy or credentials.

| Contract | Code to inspect |
| --- | --- |
| Pull-request verification plus npm trusted publishing, App-signed version writeback, and immutable release readback | [`uinaf/workspace-kit` verify](https://github.com/uinaf/workspace-kit/blob/main/.github/workflows/verify.yml), [release](https://github.com/uinaf/workspace-kit/blob/main/.github/workflows/release.yml), and [semantic-release config](https://github.com/uinaf/workspace-kit/blob/main/.releaserc.json) |
| Draft-first binary release, checksums, provenance attestations, immutable publication, and downstream Homebrew update | [`uinaf/slopwake` CI/release](https://github.com/uinaf/slopwake/blob/main/.github/workflows/ci.yml) and [semantic-release config](https://github.com/uinaf/slopwake/blob/main/.releaserc.json) |
| Shared npm release (`workflow_call`) with Environment-bound App token, and a change-detection composite with per-event checkout rules | [`uinaf/.github` release](https://github.com/uinaf/.github/blob/main/.github/workflows/release-npm.yml) and [changes action](https://github.com/uinaf/.github/blob/main/.github/actions/changes/action.yml); caller: [`uinaf/design` release](https://github.com/uinaf/design/blob/main/.github/workflows/release.yml) |
| Organization-level collaboration defaults | [`uinaf/.github`](https://github.com/uinaf/.github) |

Before reuse, confirm the linked repository is still public and active, open the
current source rather than relying on this summary, and preserve the target
repository's own lifecycle and policy. If no example matches, implement the
smallest tested module or local action in the repository's primary language
instead of growing inline workflow or shell logic.
