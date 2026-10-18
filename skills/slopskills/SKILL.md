---
name: slopskills
description: "Pick and install repo-local skills for a repository's stack. Use when setting up a repo, checking its skill coverage, or starting work that lacks stack guidance."
---

# Slopskills

Install the stack skills a repository needs into that repository.

## Steps

1. **Read the repo.** Agent guide, manifests, imports, and the task. In a
   monorepo, look at the affected package.
2. **Select** from the [catalog](references/catalog.md): the supported stack
   for a setup or coverage check, the task's subset for implementation work.
3. **Check what's there.** Reuse matching repo-local skills. Report name
   collisions and local edits before replacing anything. If the user chose a
   global install of a skill, skip the local copy unless they want the repo
   self-contained.
4. **Install** from the repo root with its existing installer and pin, else
   the skills CLI. List the source first; names change. Explicit skill names,
   only the repo's harnesses, project scope (no `--global` or `--all`).
   Examples and migration: [installation.md](references/installation.md).
5. **Point to them.** Add short task-to-skill pointers to the agent guide
   ("Query cache invalidation: tanstack-query"). Leave skill bodies alone.
6. **Verify** names, source records, paths, and referenced files. Check one
   task that should load the skill and one that shouldn't. Installed means
   available, not invoked; say which you tested.

For advice-only requests, list the source and compose the step 4 commands,
then stop before installing.

Report each skill with its stack evidence, its state (installed or already
there), and gaps.
