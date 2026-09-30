# Sources and measures

Read logs with small scripts that stream and filter; never load a whole log
into context. Open databases read-only. Logs hold secrets and private
messages: quote short excerpts, redact tokens, and describe confidential
contexts generically.

## Session logs

| Harness | User prompts | Full sessions |
|---|---|---|
| Claude Code | `~/.claude/history.jsonl` (`display`, `project`, `timestamp`) | `~/.claude/projects/<cwd-slug>/<session>.jsonl`; subagents under `<session>/subagents/` |
| Codex | `~/.codex/history.jsonl` (`text`, `ts`) | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` (`response_item` messages) |
| T3 Code | — | `~/.t3/userdata/state.sqlite` (`projection_thread_messages`, `projection_threads`) |

Other harnesses keep similar stores; find them under the harness's home
directory. Before counting:

- Keep only messages the user typed: drop tool results, injected reminders,
  hook output, and skill bodies.
- Separate subagent and orchestrated sessions from top-level ones; count skill
  loads for both, corrections for top-level only.
- Exclude eval and sandbox sessions, usually those with a working directory
  under a temp path. They can outnumber real sessions many times over.

Measure per week so trends show:

- sessions and prompts per harness, model, and repository;
- clusters of repeated instructions, with counts and a few dated examples;
- corrections and frustration (profanity, "I told you", "why did you",
  interrupts, rejected tool calls) and their root causes;
- turns ending in a question followed by a bare "yes" or "go";
- skills loaded by the model, typed by the user, and never loaded;
- permission denials by reason, and tool or network failures that recur.

## Forge

Use the forge CLI for the user's own account and organizations and for their
contributions elsewhere. Search APIs share one rate limit across every agent
on the account: batch queries, sample when volume is high, and stop before
exhausting it.

- pull requests opened, merged, and closed unmerged, per repository and area;
- commits on default branches, direct pushes versus merged pull requests;
- churn: renames, the same file rewritten repeatedly, things added and removed
  within days, reverts, dependency-bot volume;
- time from opening to merge, review-bot threads opened after merge or left
  unresolved, merges with failing checks;
- the user's tracker: stale, done-but-open, and duplicated items.

Contributions to projects the user doesn't own are the clearest outside
verdict on agent-written work:

- acceptance rate and time to merge, and why pull requests closed unmerged;
- what human reviewers asked to change, clustered: each cluster is a
  candidate rule or check;
- follow-up fixes and reverts of merged work.

CI usage, per repository, sampled from the forge's run history and billing:

- minutes and cost by runner type, and who triggered them: the user, agents,
  or dependency bots;
- failure rate on default branches and on pull requests, and reruns that
  passed without a change, which point to flaky tests;
- time from push to green, and the slowest jobs;
- runner or workflow changes that were made and then reverted.

## Setup

- Always-loaded text per harness: global rule files and what renders them,
  every skill description, MCP tool schemas, and session-start hook output.
- Install sources and profiles: which machine or context gets which layer,
  and what reaches a public repository or another organization's machine.
- Per repository: guide files, branch rules, required checks, CI workflows,
  secrets handling, and dependency automation.
- Machines and agents that act for other people: their tool permissions,
  who can reach them, and what credentials they hold.
