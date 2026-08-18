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

OpenCode has no compatible plugin format; its plugin API carries hooks and
tools, not skills. Expose the plugin's skills to its native discovery
instead, for example from the Claude Code checkout:

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

### Which skill owns what

Route by the artifact, not the verb:

| Artifact or task | Owner |
| --- | --- |
| README, AGENTS.md, docs/, specs: content, currency, house style | [`slopscriber`](skills/slopscriber/) |
| Prose voice, de-slopping a text or diff | [`slopclean`](skills/slopclean/) |
| Issues, epics, tracker tickets | [`slopspec`](skills/slopspec/) |
| Opening the change request | [`slopcourier`](skills/slopcourier/) |
| Getting an open change request to settled | [`slopnanny`](skills/slopnanny/) |
| Independent code review | [`slopguard`](skills/slopguard/) |
| Governed multi-unit run | [`slopmachine`](skills/slopmachine/) |
| Boot, gates, proof paths: can an agent verify here | [`slopprep`](skills/slopprep/) |
| Chat replies | [`wat`](skills/wat/) |
| PR/issue templates, `SECURITY.md`, rulesets, releases, GitHub shell | [gh-setup](https://github.com/uinaf/agent-skills/tree/main/skills/gh-setup), lives in agent-skills |

The one seam worth naming: `AGENTS.md` orientation and proof paths belong to
slopprep; its compression, currency, and style belong to slopscriber.

## Status

Live. Each CLI releases from this repo on its own tag (`slopmachine/vX.Y.Z`,
`slopguard/vX.Y.Z`); signing and verification are covered in each member's
release docs ([slopmachine](cli/slopmachine/docs/RELEASES.md),
[slopguard](cli/slopguard/docs/RELEASES.md)).

Next: [M3 dispatch + M4 learn](https://github.com/uinaf/ffss/issues/27).

## License

MIT; see [LICENSE](LICENSE); members carry their own copies.
