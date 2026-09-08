![ffss — the flipflopslopstack.](https://uinaf.dev/og/banner/ffss.png)

# uinaf/ffss

**ffss**, the flipflopslopstack. A reviewer that doesn't like the agent,
and skills so the agent finishes the job.
Ships slop; checks receipts.

## CLI

| CLI | What it does |
| --- | --- |
| [`slopguard`](cli/slopguard/) | Independent second-model code review with stable terminal and JSON output |

## Skills

One job each.

| Skill | Use it for |
| --- | --- |
| [`slopmachine`](skills/slopmachine/) | Completing an agreed plan through implementation, verification, review, and delivery |
| [`slopguard`](skills/slopguard/) | Reviewing one completed change with an independent model |
| [`slopcourier`](skills/slopcourier/) | Opening a change request for completed, verified work |
| [`slopnanny`](skills/slopnanny/) | Monitoring a change request through review, CI, and merge |
| [`slopclean`](skills/slopclean/) | Removing AI tells from prose, code, and tests |
| [`slopspec`](skills/slopspec/) | Saving agreed work as issues, epics, or durable plans |
| [`slopscriber`](skills/slopscriber/) | Auditing and updating repository documentation |
| [`slopprep`](skills/slopprep/) | Preparing repositories and runners for autonomous work |
| [`wat`](skills/wat/) | Rewriting rambling replies as terse status updates |

## Installation

### CLI

macOS, from the `uinaf/tap` Homebrew tap (signed):

```bash
brew install --cask uinaf/tap/slopguard
slopguard --version
```

Linux, or macOS without Homebrew (amd64 or arm64):

```bash
curl --proto '=https' --tlsv1.2 -fsSL \
  https://raw.githubusercontent.com/uinaf/ffss/main/cli/slopguard/install.sh | sh
~/.local/bin/slopguard --version
```

The installer verifies release checksums and defaults to `~/.local/bin`.
Verification details: [slopguard](cli/slopguard/README.md#install).

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
