![slopguard — structured independent code review, as a CLI and an agent skill.](https://uinaf.dev/og/banner/slopguard.png)

# slopguard

`slopguard` is a Go CLI and agent skill for one structured, independent code
review through Codex CLI or Claude Code. It freezes an explicit Git target,
validates the provider result locally, and emits a stable terminal or JSON
report.

## Install

macOS, signed CLI from the `uinaf/tap` Homebrew tap:

```bash
brew install --cask uinaf/tap/slopguard
slopguard --version
```

Linux, or a Mac without tap access, latest amd64 or arm64 release binary
without Go, `jq`, or `sudo`:

```bash
curl --proto '=https' --tlsv1.2 -fsSL \
  https://raw.githubusercontent.com/uinaf/ffss/main/cli/slopguard/install.sh | sh
~/.local/bin/slopguard --version
```

- Pass `--version "$TAG"` or `--dest /chosen/bin` after `sh -s --` to pin a
  release or change the destination.
- The installer checks the archive against the release's `checksums.txt`.
  Both come over the same HTTPS connection; for independent provenance,
  [verify the release](#verify-a-release).
- Upgrade with `slopguard selfupdate`, or `brew upgrade` for a tap install.

With Go 1.26 or newer:

```bash
go install github.com/uinaf/ffss/cli/slopguard/cmd/slopguard@latest
slopguard --version
```

Go-built binaries track `main`, report `dev (unknown)`, and can't
`selfupdate`.

Needs Git 2.45 or newer and the selected harness (Codex CLI or Claude Code) on
`PATH`.

### Verify a release

```bash
archive=slopguard_v3.0.0_darwin_arm64.tar.gz   # the release and target you downloaded

cosign verify-blob \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity \
    "https://github.com/uinaf/ffss/.github/workflows/release-slopguard.yml@refs/heads/main" \
  --certificate-oidc-issuer "https://token.actions.githubusercontent.com" \
  checksums.txt
grep "  ${archive}$" checksums.txt | shasum -a 256 -c -
gh attestation verify --owner uinaf "$archive"
```

## Quick use

Check the configuration and provider, then review local changes:

```bash
prompt="Review this completed change against its acceptance criteria."
slopguard config --engine codex
slopguard doctor --engine codex
slopguard review --mode local --engine codex \
  --prompt "$prompt"
```

`branch` reviews the merge-base-to-HEAD diff; `commit` reviews one non-merge
commit:

```bash
prompt="Review this completed change against its acceptance criteria."
slopguard review --mode branch --base origin/main --engine claude \
  --prompt "$prompt"

slopguard review --mode commit --commit HEAD --engine codex \
  --prompt "$prompt"
```

Slopguard never edits source, runs tests, commits, pushes, picks a provider,
or falls back to another model. Run the builder's checks before review.

`--output json` writes the [result contract](docs/RESULT_SCHEMA.md) to stdout.
Exit 0 is a clean review, 1 is findings, and 2 is no trustworthy result.

## Doctor

`slopguard doctor --engine <engine> [--json]` checks the harness, its version
and capabilities, and the web policy offline, without a model call or login.
Review never depends on it. Authentication is advisory: `ready` means a
credential variable is set; `delegated` means the provider session decides at
runtime.

| Exit | Meaning |
| --- | --- |
| 0 | `ready` |
| 1 | a valid `not_ready` diagnostic |
| 2 | configuration rejected, repository not resolvable, or no valid diagnostic |

## Documentation

- [Configuration, web and repository access, cache, telemetry](docs/CONFIG.md)
- [Targets and what leaves the machine](docs/TARGETS.md)
- [JSON result and exit contract](docs/RESULT_SCHEMA.md)
- [Agent skill](../../skills/slopguard/SKILL.md)
- [Contributing](CONTRIBUTING.md)

## License

[MIT](LICENSE), with copyright notices for uinaf and OpenClaw, whose
[original slopguard skill](https://github.com/openclaw/agent-skills/tree/main/skills/slopguard)
this succeeds.
