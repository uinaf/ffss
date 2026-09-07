![slopguard — structured independent code review, as a CLI and an agent skill.](https://uinaf.dev/og/banner/slopguard.png)

# slopguard

`slopguard` is a Go CLI and agent skill for one structured, independent code
review through Codex CLI, Claude Code, Cursor Agent, or Grok Build. It freezes
an explicit Git target, validates the provider result locally, and emits a
stable terminal or JSON report.

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
  release or override `${HOME}/.local/bin`.
- The installer downloads the archive and `checksums.txt` from the same GitHub
  Release, verifies the exact SHA-256, and atomically replaces the destination
  binary.
- Installer-managed binaries upgrade with `slopguard selfupdate` (`--check`
  probes without touching the binary); brew installs upgrade through
  `brew upgrade`.
- [Release verification](docs/RELEASES.md#installer-trust-boundary) covers the
  HTTPS trust boundary and independent Cosign and GitHub attestation checks.

With Go 1.26 or newer:

```bash
go install github.com/uinaf/ffss/cli/slopguard/cmd/slopguard@latest
slopguard --version
```

Go-built binaries track `main`, report `dev (unknown)`, and are refused by
`selfupdate`. Signed, versioned builds come from the tap or the installer.

Runtime dependencies: Git 2.41 or newer and the selected review harness on
`PATH`. Several harnesses may be installed; `--engine` selects exactly one.

## Quick use

Inspect the effective configuration and its sources, then review dirty local
changes:

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

Slopguard never edits source, runs tests, commits, pushes, chooses a provider,
or falls back to another model. Builder verification happens before review.

Telemetry is off by default. Enable one run with `--telemetry`; export the
bounded local spool with `slopguard telemetry export`. Reviews never upload
telemetry. See [Optional telemetry](docs/TELEMETRY.md).

## Machine output

`--output json` emits the versioned result contract as the only stdout value;
progress and diagnostics use stderr.

```bash
prompt="Review this completed change against its acceptance criteria."
slopguard review --mode branch --base origin/main --engine codex \
  --output json --prompt "$prompt" > result.json
```

Exit 0: valid clean review. Exit 1: valid findings. Exit 2: no trustworthy
review result.

The binary exposes its canonical contracts:

```bash
slopguard schema review > review-v1.schema.json
slopguard schema result > result-v1.schema.json
```

`review` is the structured output expected from a provider; `result` is the
CLI's final machine report. Neither describes CLI request parameters.

## Agent skill

The family [agent skill](../../skills/slopguard/SKILL.md) delegates one
independent review to the installed CLI; it contains no second runtime.

## Documentation

- [Configuration](docs/CONFIG.md)
- [Provider doctor](docs/DOCTOR.md)
- [Review engines](docs/engines/README.md)
- [Performance measurement](docs/PERFORMANCE.md)
- [Optional telemetry](docs/TELEMETRY.md)
- [Local, branch, and commit targets](docs/TARGETS.md)
- [Versioned result and exit contract](docs/RESULT_SCHEMA.md)
- [Release artifacts and verification](docs/RELEASES.md)

## Contributing

See [Contributing](CONTRIBUTING.md) for setup, release gates, and pull-request
expectations.

## License

This project succeeds OpenClaw's
[original slopguard agent skill](https://github.com/openclaw/agent-skills/tree/main/skills/slopguard),
MIT licensed and credited to OpenClaw. This project's [MIT license](LICENSE)
preserves the copyright notices for both OpenClaw and uinaf.
