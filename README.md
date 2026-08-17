![ffsstack — the flipflopslopstack.](https://uinaf.dev/og/banner/ffsstack.png)

# uinaf/ffsstack

**ffsstack**, the flipflopslopstack. A state machine that doesn't believe
the agent, a reviewer that doesn't like the agent, and some skills so the
agent behaves. Ships slop; checks receipts.

## CLIs

| CLI | What it is |
| --- | --- |
| [`slopmachine`](cli/slopmachine/) | The deterministic state machine: Go CLI + SQLite spine gating intake → release → build → verify → review → deliver → watch, with forge-verified evidence |
| [`slopguard`](cli/slopguard/) | Pre-ship second-model review closeout CLI |

## Skills

Install all nine as one Agent Plugins package. Claude Code, Codex, Cursor,
and Grok read the same marketplace and the portable root manifest:

```text
# Claude Code
/plugin marketplace add uinaf/ffsstack
/plugin install ffsstack@ffsstack

# Codex CLI
codex plugin marketplace add uinaf/ffsstack
codex plugin add ffsstack@ffsstack

# Cursor CLI (then /plugins in interactive mode to install)
cursor-agent plugin marketplace add https://github.com/uinaf/ffsstack

# Grok CLI
grok plugin install uinaf/ffsstack --trust
```

OpenCode has no compatible plugin format; its plugin API carries hooks and
tools, not skills. Expose the plugin's skills to its native discovery
instead, for example from the Claude Code checkout:

```text
mkdir -p ~/.config/opencode/skills
ln -sfn ~/.claude/plugins/marketplaces/ffsstack/skills/* ~/.config/opencode/skills/
```

The plugin is SHA-versioned; updates follow this repo's `main`. Skills marked
`disable-model-invocation` load only on explicit `/name` invocation in
harnesses that support the flag. Codex mirrors that policy with
`policy.allow_implicit_invocation: false` in each matching `agents/openai.yaml`.
The repo root [`plugin.json`](plugin.json) is the shared manifest from the
[Agent Plugins standard](https://agent-plugins.org/).

| Skill | What it does |
| --- | --- |
| [`slopmachine`](skills/slopmachine/) | Drive the slopmachine CLI through a governed run |
| [`slopguard`](skills/slopguard/) | Run one independent review through the slopguard CLI |
| [`slopcourier`](skills/slopcourier/) | Deliver finished work as a change request on the repo's forge, with the visual-evidence ladder |
| [`slopnanny`](skills/slopnanny/) | Babysit a delivered change request through review and CI to a settled outcome |
| [`slopclean`](skills/slopclean/) | Strip AI tells from writing or a diff |
| [`slopspec`](skills/slopspec/) | Turn agreed work into durable tracker plans |
| [`slopscriber`](skills/slopscriber/) | Audit and rewrite repo docs and agent guidance |
| [`slopprep`](skills/slopprep/) | Make a repository agent-ready |
| [`wat`](skills/wat/) | Whip rambling replies back into terse labeled deltas |

Reference bindings elsewhere in the family: slopzapper (forge review bot),
slopscouter (QA, planned), slopbench (evals), slopwake.

## Status

Live. Both CLIs release from this repo on per-member tags
(`slopmachine/vX.Y.Z`, `slopguard/vX.Y.Z`): signed and notarized macOS
builds, Cosign-signed checksums, GitHub build attestations, Homebrew casks
in [`uinaf/homebrew-tap`](https://github.com/uinaf/homebrew-tap), Linux
installers, and in-place `selfupdate`. CI runs each member's own gate
(`mise run verify` inside `cli/slopmachine/` and `cli/slopguard/`) plus a
skills lint. Next:
[M3 dispatch + M4 learn](https://github.com/uinaf/ffsstack/issues/27).

## License

MIT; see [LICENSE](LICENSE); members carry their own copies.
