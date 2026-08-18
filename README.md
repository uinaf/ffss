![ffss — the flipflopslopstack.](https://uinaf.dev/og/banner/ffss.png)

# uinaf/ffss

**ffss**, the flipflopslopstack. A state machine that doesn't believe
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
/plugin marketplace add uinaf/ffss
/plugin install ffss@ffss

# Codex CLI
codex plugin marketplace add uinaf/ffss
codex plugin add ffss@ffss

# Cursor CLI (then /plugins in interactive mode to install)
cursor-agent plugin marketplace add https://github.com/uinaf/ffss

# Grok CLI
grok plugin install uinaf/ffss --trust
```

OpenCode [plugins](https://opencode.ai/docs/plugins/) are JS hooks and custom
tools, not a skill carrier; its [agent skills](https://opencode.ai/docs/skills/)
are a separate native feature discovered from `~/.config/opencode/skills`.
Link the plugin's skills there, for example from the Claude Code checkout:

```text
mkdir -p ~/.config/opencode/skills
ln -sfn ~/.claude/plugins/marketplaces/ffss/skills/* ~/.config/opencode/skills/
```

- The plugin is SHA-versioned; updates follow this repo's `main`.
- Skills marked `disable-model-invocation` load only on explicit `/name`
  invocation in harnesses that support the flag; Codex mirrors that policy
  with `policy.allow_implicit_invocation: false` in each matching
  `agents/openai.yaml`.
- The repo root [`plugin.json`](plugin.json) is the shared manifest from the
  [Agent Plugins standard](https://agent-plugins.org/).

One skill per lane; route by the artifact, not the verb:

| Skill | Owns |
| --- | --- |
| [`slopmachine`](skills/slopmachine/) | The governed multi-unit run, driving the slopmachine CLI |
| [`slopguard`](skills/slopguard/) | One independent code review, through the slopguard CLI |
| [`slopcourier`](skills/slopcourier/) | Opening the change request, with the visual-evidence ladder |
| [`slopnanny`](skills/slopnanny/) | Walking an open change request to a settled outcome |
| [`slopclean`](skills/slopclean/) | AI tells: stripping slop from writing, a diff, or a test suite |
| [`slopspec`](skills/slopspec/) | Tracker artifacts: issues, epics, durable work plans |
| [`slopscriber`](skills/slopscriber/) | Repo docs: README, AGENTS.md, docs/, specs; content, currency, house style |
| [`slopprep`](skills/slopprep/) | Agent readiness: boot, gates, proof paths |
| [`wat`](skills/wat/) | Chat replies: terse labeled deltas |

The GitHub shell (PR/issue templates, `SECURITY.md`, rulesets, releases) is
[gh-setup](https://github.com/uinaf/agent-skills/tree/main/skills/gh-setup)'s
lane, in agent-skills. The one seam worth naming: `AGENTS.md` orientation and
proof paths belong to slopprep; its compression, currency, and style belong
to slopscriber.

Reference bindings elsewhere in the family:
[slopzapper](https://slopzapper.uinaf.dev) (forge review bot),
slopscouter (QA, planned),
[slopbench](https://github.com/uinaf/slopbench) (evals), and
[slopwake](https://github.com/uinaf/slopwake).

## License

MIT; see [LICENSE](LICENSE); members carry their own copies.
