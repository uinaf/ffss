---
name: slopskills
description: Select and install repository-local skills for the current stack and task. Use when setting up a repo, checking skill coverage, or preparing work with missing stack guidance.
---

# Slopskills

Give the target repository the skills its work needs. Keep this workflow global;
install the selected stack skills in the consuming repository.

## Select

Read the target's agent guide, worktree state, package manifests, and relevant
source imports. Identify the requested task and harness. In a monorepo, inspect
the affected package before treating a dependency elsewhere as relevant.

Use the [recommended catalog](references/catalog.md) to match evidence to skills.
For repository setup or a coverage audit, consider the supported stack; for an
implementation task, select the subset needed for that work. A recommendation
is not a reason to introduce a library, migrate tests, or change architecture.

Inspect existing repo-local skills and their source records first. Reuse a
matching installation; report name collisions or local edits before replacing
them. Global availability does not make a repository self-contained.
Honor explicitly chosen global skills and skip duplicate local installs unless
the user requests a self-contained repository.

If asked only for advice or inspection, report the selection and exact install
commands without making changes. A request to equip or set up repo skills
authorizes the relevant local installations. Otherwise report missing skills
and continue the authorized task without silently changing its setup.

## Install

Use the repository's existing installer and version pin when available;
otherwise use the skills CLI. Check its help and list the selected source's
skills before installation, since names and package layouts can change.

Run from the target repo root. Select explicit skill names and only the requested
or repository-configured harnesses. For example, with Codex:

```sh
npx skills add Effect-TS/skills --list
npx skills add Effect-TS/skills --skill effect-ts --agent codex --yes
```

Use project scope, never `--global` or `--all`. Do not commit absolute links
into a home directory or plugin cache, or prune global or unrelated skills as
a side effect. Harness paths, lockfiles, and global-to-local migration:
[installation.md](references/installation.md).

## Make discoverable and verify

For an authorized setup, add concise task-to-skill pointers to the target's
existing agent guide. Use concrete triggers such as Query cache invalidation,
Swift actor isolation, or React effect changes; link installed entrypoints.
Keep upstream skill bodies intact and avoid copying their instructions into
`AGENTS.md`. Load the selected skill before continuing work it governs.

Verify installed names, source records, harness paths, and referenced files.
Check one representative task against the routing and one unrelated task that
should not load the skill. Installation proves availability, not automatic
invocation; distinguish static checks from a fresh-session activation test.

Report selected skills with stack evidence, installed or already-present state,
and unresolved gaps. Stop after the requested repository coverage; do not scan
or install across other repositories unless asked.
