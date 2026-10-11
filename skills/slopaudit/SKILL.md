---
name: slopaudit
description: "Audit a person's whole agentic setup: loaded rules and skills, how they are actually used in session logs, recent shipped work, and the repositories, machines, MCP servers, and pipelines agents touch. Use for a periodic retro, or when asked where agents struggle or what to prune, fix, or add in an agent workflow; not for reviewing one change or auditing one codebase."
---

# Slopaudit

Find what to prune, fix, or add so the user can tell agents less and trust
them more. The audit is read-only and ends in a discussion; apply a finding
only after the user approves it.

## Scope

Default to this machine, every harness with local history, and the last 30
days. A narrower ask, such as one repository's last ten sessions, runs only
the lanes that answer it. Ask only for what cannot be discovered: other
machines, which harnesses to include, and contexts to keep out, such as an
employer's or a client's work. Write notes and delegate reports to a scratch
directory outside every repository, and keep a running notes file there so the
discussion can resume.

## Lanes

Run independent lanes in parallel, delegating when the harness allows. Each
lane writes a full report with evidence and returns a short summary.
Where to look and what to measure: [sources.md](references/sources.md).

1. **Inventory.** Every always-loaded instruction and installed skill, plugin,
   MCP server, hook, and permission rule per harness: its source, how it is
   installed, and which machines or profiles get it. Estimate always-loaded
   tokens.
2. **Usage.** From session logs: repeated instructions, corrections and
   frustration, bare approvals after the agent asked, status polling, skills
   invoked by the model versus typed by the user, never-loaded skills, and
   permission denials.
3. **Friction.** From the agents' own tool calls: where they searched long
   before finding the right file or fact, followed a doc the code
   contradicts, fetched oversized or repeated tool output, or guessed
   because a log, service, or fact was out of reach.
4. **Shipped work.** From the forge: output by area, time spent on agent
   tooling versus product, churn, merges that beat review, and unresolved
   review threads after merge. For contributions to other people's projects,
   what maintainers accepted and rejected.
5. **Review lessons.** The mistakes agents keep making, as caught by the
   people and review bots who read their pull requests: findings, requested
   changes, and commits reviewers or maintainers pushed onto the branch.
   Cluster them; each recurring cluster is a lesson with a proposed owner.
   Also measure the cost from first commit to production per merged pull
   request: rework commits, review rounds, CI runs, reruns, and minutes.
   Report it per pull request, naming the costliest and any still unmerged.
6. **Setup.** Duplicated rules, contradictions, instructions a capable model
   already follows, stale references, skills competing for one request, checks
   and review rules that mostly fire on correct work, context leaking into
   places it doesn't belong (public repositories, another employer's machine),
   and the guardrails around repositories, pipelines, and agents that serve
   other people.
7. **External**, when asked: compare popular skill collections against the
   user's own and say what is worth adopting, adapting, or skipping.

A lane is done when its report cites a file and line, a count with its query,
or a link for every claim.

## Validate

Delegate reports overstate. Before a finding reaches the user, check its
evidence yourself against the current source, and drop or narrow what doesn't
hold. A behavior the user set on purpose is not a defect; note it once and
move on.

## Report

Lead with the diagnosis in a sentence or two, then ranked findings. For each:
the evidence, the smallest fix, and its owner. For a lesson, name one owner,
the strongest that fits: a change that removes the wrong way to do it, then a
type, lint rule, or CI check that fails on that exact mistake, then the skill
or playbook for that kind of work, then a global rule
([enforcement](../slopprep/references/setup-patterns.md#mechanical-enforcement)).
For friction, the fix is a pointer from a file agents already read, a
corrected doc, leaner tool output, or read access, not a new rule. Prefer
deleting or narrowing an instruction to adding one, and prefer the user's
existing mechanisms, including checks that exist but aren't wired in or
silently pass, to new hooks, scripts, or infrastructure. Group decisions the
user must make into a short numbered list.

Record a baseline for next time from the lanes that ran: corrections per
completed outcome, searches before the first read of a changed file, the
always-loaded token count, and the top repeated instructions. Save the report
and baseline where the user keeps knowledge, next to earlier runs so the trend
shows; file follow-up work in their tracker. Raw log excerpts stay in scratch
and are deleted after the run.

## Apply

Apply approved findings in their owning repositories, with each repository's
own gates and delivery rules, and re-sync the installed copies. Verify each
installed result, not only the source change; a new check must fail on a real
past instance of its mistake. Keep the notes file current with what shipped,
its commit, and its proof.
