![ffss — the flipflopslopstack.](https://uinaf.dev/og/banner/ffss.png)

# uinaf/ffss

**ffss**, the flipflopslopstack. A reviewer that doesn't like the agent,
and skills so the agent finishes the job.
Ships slop; checks receipts.

## Skills

One job each.

| Skill | Use it for |
| --- | --- |
| [`slopguard`](skills/slopguard/) | Reviewing one completed change with the other model family: `codex review` or Claude's `/code-review` |
| [`slopcourier`](skills/slopcourier/) | Delivering verified work as a change request and seeing it through review, CI, merge, and the runs the merge starts |
| [`slopclean`](skills/slopclean/) | Removing AI tells from prose, code, and tests; cleaning drifted docs and agent guides |
| [`slopaudit`](skills/slopaudit/) | Auditing an agentic setup, its recent use, and shipped work to decide what to prune, fix, or add |
| [`slopskills`](skills/slopskills/) | Selecting and installing repo-local skills for the stack and task |

Stack skills such as `react-ban-use-effect` live in
[uinaf/agent-skills](https://github.com/uinaf/agent-skills).

## Installation

Add this repository's marketplace:

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
