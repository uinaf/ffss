---
name: slopaudit
description: "Audit a person's whole agent setup: loaded rules and skills, session logs, shipped work, and the repos, machines, MCP servers, and pipelines agents touch. Use for a periodic retro, or when asked where agents struggle or what to prune, fix, or add."
---

# Slopaudit

Find what to prune, fix, or add so the user can tell agents less and trust
them more. The audit is read-only and ends in a discussion.

## Scope

Default: this machine, every harness with local history, the last 30 days.
A narrower ask runs only the lanes that answer it. Ask about other machines,
harnesses to include, and contexts to keep out (employer or client work).
Keep notes and lane reports in a scratch directory outside every repo.

## Lanes

Run independent lanes in parallel. Each writes a report with evidence and
returns a short summary. Where to look and what to measure:
[sources.md](references/sources.md).

1. **Inventory.** Always-loaded rules, skills, plugins, MCP servers, hooks,
   and permissions per harness and machine, with a token estimate.
2. **Usage.** What the user repeats, corrects, rubber-stamps, and never uses.
3. **Friction.** Where agents searched long, followed wrong docs, pulled
   oversized output, or guessed.
4. **Shipped work.** Output, churn, and review outcomes on the forge,
   including other people's projects.
5. **Review lessons.** Mistakes reviewers keep catching, clustered by
   mistake, and the cost per merged PR from first commit to production.
6. **Setup.** Duplicates, contradictions, no-op instructions, stale
   references, competing skills, noisy checks, context leaks, and guardrails
   on agents that serve other people.
7. **External** (when asked). Popular skill collections: adopt, adapt, or
   skip.

Done when every claim cites a file and line, a count with its query, or a
link.

## Validate

Check each finding against the current source yourself. Drop or narrow what
doesn't hold. A behavior the user set on purpose is not a defect.

## Report

Lead with a one- or two-sentence diagnosis, then ranked findings: evidence,
smallest fix, owner. End with the user's decisions as a short numbered list.

**Owner ladder** for a lesson, strongest that fits:

1. Remove the wrong way to do it.
2. A check that fails on that exact mistake (type, lint rule, schema, test,
   local gate, then CI or a hook), with an error naming the right way.
3. The skill or playbook for that kind of work.
4. A global rule.

Friction gets a pointer, a fixed doc, leaner output, or read access. Prefer
deleting instructions and wiring existing checks over adding new ones.

**Baseline:** corrections per completed outcome, searches before the first
read of a changed file, always-loaded tokens, top repeated instructions.

Save the report and baseline where the user keeps knowledge, next to earlier
runs. File follow-ups in their tracker. Delete raw log excerpts from scratch.

## Apply

Once the user approves a finding, apply it in its owning repo with that
repo's gates and delivery, then re-sync installed copies. A new check must
fail on a real past instance of its mistake.
