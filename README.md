![ffss — the flipflopslopstack.](https://uinaf.dev/og/banner/ffss.png)

# uinaf/ffss

**ffss**, the flipflopslopstack. A state machine that doesn't believe the
agent, a reviewer that doesn't like the agent, and skills so the agent behaves.
Ships slop; checks receipts.

## CLIs

| CLI | What it does |
| --- | --- |
| [`slopmachine`](cli/slopmachine/) | Evidence-gated implementation workflow with resumable state and forge-verified delivery |
| [`slopguard`](cli/slopguard/) | Independent second-model code review with stable terminal and JSON output |

## Skills

Each skill has one job.

| Skill | Use it for |
| --- | --- |
| [`slopmachine`](skills/slopmachine/) | Running an agreed plan through build, verification, review, and delivery |
| [`slopguard`](skills/slopguard/) | Reviewing one completed change with an independent model |
| [`slopcourier`](skills/slopcourier/) | Opening a change request for completed, verified work |
| [`slopnanny`](skills/slopnanny/) | Monitoring a change request through review, CI, and merge |
| [`slopclean`](skills/slopclean/) | Removing AI tells from prose, code, and tests |
| [`slopspec`](skills/slopspec/) | Saving agreed work as issues, epics, or durable plans |
| [`slopscriber`](skills/slopscriber/) | Auditing and updating repository documentation |
| [`slopprep`](skills/slopprep/) | Preparing repositories and runners for autonomous work |
| [`wat`](skills/wat/) | Rewriting rambling replies as terse status updates |

## Installation

### CLIs

On macOS, install both signed CLIs from the `uinaf/tap` Homebrew tap:

```bash
brew install --cask uinaf/tap/slopmachine uinaf/tap/slopguard
slopmachine version
slopguard --version
```

On Linux, or on macOS without Homebrew, install the latest amd64 or arm64
releases:

```bash
curl --proto '=https' --tlsv1.2 -fsSL \
  https://raw.githubusercontent.com/uinaf/ffss/main/cli/slopmachine/install.sh | sh
curl --proto '=https' --tlsv1.2 -fsSL \
  https://raw.githubusercontent.com/uinaf/ffss/main/cli/slopguard/install.sh | sh
~/.local/bin/slopmachine version
~/.local/bin/slopguard --version
```

The installers verify release checksums and use `~/.local/bin` by default.
See the detailed installation and verification guides for
[slopmachine](cli/slopmachine/README.md#install) and
[slopguard](cli/slopguard/README.md#install).

### Plugin

Install the skills through the ffss Agent Plugins marketplace:

```text
# Claude Code
/plugin marketplace add uinaf/ffss
/plugin install ffss@ffss

# Codex CLI
codex plugin marketplace add uinaf/ffss
codex plugin add ffss@ffss

# Cursor CLI (then install through /plugins)
cursor-agent plugin marketplace add https://github.com/uinaf/ffss

# Grok CLI
grok plugin install uinaf/ffss --trust
```

## License

MIT; see [LICENSE](LICENSE); members carry their own copies.
