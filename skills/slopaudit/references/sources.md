# Sources and measures

Stream and filter logs with small scripts; open databases read-only. Logs
hold secrets and private messages: quote short excerpts, redact tokens, and
describe confidential contexts generically.

## Session logs

| Harness | User prompts | Full sessions |
|---|---|---|
| Claude Code | `~/.claude/history.jsonl` (`display`, `project`, `timestamp`) | `~/.claude/projects/<cwd-slug>/<session>.jsonl`; subagents under `<session>/subagents/` |
| Codex | `~/.codex/history.jsonl` (`text`, `ts`) | `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl` (`response_item` messages) |
| T3 Code | — | `~/.t3/userdata/state.sqlite` (`projection_thread_messages`, `projection_threads`) |

Before counting:

- Keep only what the user typed: drop tool results, reminders, hook output,
  and skill bodies.
- Separate subagent sessions from top-level ones. Count skill loads for both,
  corrections for top-level only.
- Exclude eval and sandbox sessions (usually a working directory under a temp
  path). They can outnumber real ones.

Per week:

- sessions and prompts per harness, model, and repo;
- repeated-instruction clusters with counts and dated examples;
- corrections and frustration (profanity, "I told you", "why did you",
  interrupts, rejected tool calls) and their causes;
- turns ending in a question answered by a bare "yes" or "go", and
  status polling;
- skills loaded by the model, typed by the user, never loaded;
- permission denials and recurring tool or network failures.

## Friction

From full sessions, top-level and subagent, eval and sandbox excluded:

- searches before the first read of a file the session went on to change
  (by edit tool or shell); count sessions that changed nothing separately;
- files read repeatedly in a session, facts looked up again across sessions;
- docs or skills the agent followed and found wrong;
- tool output size by tool and MCP server: largest calls, totals, output
  truncated or refetched with a filter;
- guesses and questions a log, file, or read-only service could have
  answered.

## Forge

Search APIs share one rate limit across every agent on the account: batch,
sample, and stop before exhausting it.

- PRs opened, merged, and closed unmerged, per repo and area; agent tooling
  versus product work;
- default-branch commits: direct pushes versus merged PRs;
- churn: renames, files rewritten repeatedly, add-then-remove within days,
  reverts, dependency-bot volume;
- time to merge, bot threads opened after merge or left unresolved, merges
  with failing checks;
- tracker: stale, done-but-open, duplicate items.

Other people's projects: acceptance rate, time to merge, why PRs closed.

## Review lessons

Per agent-written PR in the window (sample if many):

- bot findings and whether each was fixed, rejected, or left open;
- human comments and requested changes, weighting maintainers of projects the
  user doesn't own highest;
- commits others pushed onto the branch, and maintainer edits between the
  last agent commit and the merge;
- follow-up fixes and reverts after merge.

Cluster by mistake, not reviewer. Count each cluster, link two or three
examples, and check whether an existing skill, rule, or check already claims
to prevent it. A written lesson that keeps recurring means the text doesn't
work.

Cost per merged PR: commits after opening, review rounds, CI runs and reruns,
CI minutes, time from first commit to merge and deploy. Name the costliest
and any still unmerged.

CI per repo: minutes and cost by runner and trigger (user, agents, dependency
bots); failure rate on default branches and PRs; reruns that passed unchanged
(flakes); push-to-green time and slowest jobs; runner or workflow changes
later reverted.

## Setup

- Always-loaded text per harness: global rules and what renders them, skill
  descriptions, MCP tool schemas, session-start hook output.
- Install sources and profiles: which machine gets which layer, and what
  reaches a public repo or another organization's machine.
- Per repo: guides, branch rules, required checks, CI, secrets handling,
  dependency automation.
- Agents that act for other people: tool permissions, who can reach them,
  what credentials they hold.
