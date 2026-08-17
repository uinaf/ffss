![slopguard — structured independent code review, as a CLI and an agent skill.](https://uinaf.dev/og/banner/slopguard.png)

# slopguard

`slopguard` is a Go CLI and agent skill for one structured, independent
code review through Codex CLI, Claude Code, Cursor Agent, or Grok Build. It freezes
an explicit Git target, scans the complete bundle for secrets, validates the
provider result locally, and emits a stable terminal or JSON report.

## Install

On macOS, install the signed CLI from the `uinaf/tap` Homebrew tap:

```bash
brew install --cask uinaf/tap/slopguard
slopguard --version
```

On Linux amd64 or arm64, install the latest released binary without Go,
Homebrew, `jq`, or `sudo`:

```bash
curl --proto '=https' --tlsv1.2 -fsSL \
  https://raw.githubusercontent.com/uinaf/ffsstack/main/cli/slopguard/install.sh | sh
~/.local/bin/slopguard --version
```

Pass `--version "$TAG"` or `--dest /chosen/bin` after `sh -s --` to pin a
release or override `${HOME}/.local/bin`. The installer downloads the archive
and `checksums.txt` from the same GitHub Release, verifies the exact SHA-256,
and atomically replaces the destination binary. Installer-managed binaries
upgrade in place with `slopguard selfupdate` (`--check` probes without
touching the binary); brew-managed installs upgrade through `brew upgrade`
instead. See
[Release verification](docs/RELEASES.md#linux-installer-trust-boundary) for the
HTTPS trust boundary and independent Cosign and GitHub attestation checks.

Consumers that prefer Go tooling can instead install with Go 1.26 or newer:

```bash
go install github.com/uinaf/ffsstack/cli/slopguard/cmd/slopguard@latest
slopguard --version
```

Go-built binaries track `main` and report `dev (unknown)`; `selfupdate`
refuses them. Signed, versioned builds come from the tap or the installer.

Runtime dependencies are Git 2.41 or newer, the `trufflehog` executable, and
the selected review harness available on `PATH`. Multiple supported harnesses
may be installed; `--engine` selects exactly one for each run.

## Quick use

Inspect the effective configuration and its sources, then review dirty local
changes:

```bash
prompt="Review this completed change against its acceptance criteria."
slopguard config --engine codex
slopguard review --mode local --engine codex \
  --prompt "$prompt"
```

Use `branch` for the complete merge-base-to-HEAD diff or `commit` for one
non-merge commit:

```bash
prompt="Review this completed change against its acceptance criteria."
slopguard review --mode branch --base origin/main --engine claude \
  --prompt "$prompt"

slopguard review --mode commit --commit HEAD --engine codex \
  --prompt "$prompt"
```

Slopguard never edits source, runs tests, commits, pushes, chooses a provider,
or falls back to another model. Builder verification happens before review.

## Machine output

Use `--output json` for the versioned result contract. The JSON document is the
only stdout value; progress and diagnostics use stderr.

```bash
prompt="Review this completed change against its acceptance criteria."
slopguard review --mode branch --base origin/main --engine codex \
  --output json --prompt "$prompt" > result.json
```

Exit 0 means a valid clean review, exit 1 means valid findings, and exit 2 means
no trustworthy review result.

The installed binary also exposes its exact canonical contracts:

```bash
slopguard schema review > review-v1.schema.json
slopguard schema result > result-v1.schema.json
```

`review` describes the structured output expected from a review provider;
`result` describes the CLI's final machine report. These are output contracts,
not a generic schema for CLI request parameters.

## Agent skill

The family [agent skill](../../skills/slopguard/SKILL.md) delegates one independent
review to the installed CLI. It does not contain a second runtime.

## Documentation

- [Configuration and isolation](docs/CONFIG.md)
- [Review engines](docs/engines/README.md)
- [Local, branch, and commit targets](docs/TARGETS.md)
- [Versioned result and exit contract](docs/RESULT_SCHEMA.md)
- [Release artifacts and verification](docs/RELEASES.md)

## Contributing

See [Contributing](CONTRIBUTING.md) for setup, release gates, and pull-request
expectations.

## License

This project succeeds OpenClaw's
[original slopguard agent skill](https://github.com/openclaw/agent-skills/tree/main/skills/slopguard).
The original workflow is MIT licensed and credited to OpenClaw. This project's
[MIT license](LICENSE) preserves the copyright notices for both OpenClaw and
uinaf.
