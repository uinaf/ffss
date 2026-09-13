---
name: slopscriber
description: "Update repository documentation and agent guidance against current sources when fixing drift or cleaning up docs."
---

# Slopscriber

Keep documentation accurate, findable, and worth reading. Edit the affected
surfaces; a local change does not require a repository-wide audit.

Verify claims against their owning code, configuration, or cited source.
Preserve current commands, invariants, boundaries, and recovery paths; remove
stale facts, repetition, and history that belongs elsewhere. Keep one canonical
home per contract and link to it with a task-shaped label.

## Code and config own implementation facts

Treat code and structured config as documentation for the facts they express.
Link to the owning file and identify the relevant key, section, or symbol;
do not maintain prose copies of inventories, defaults, schemas, or task lists.
For example, link to an inventory group instead of listing every host and
repeating the same command for each one. A value change should not require
a matching Markdown edit just to keep a second copy current.

Keep what the source does not explain: intent, tradeoffs, non-obvious operating
constraints, recovery procedures, and minimal examples needed to use it.
Verify the source is accessible to the intended reader before replacing prose
with a link. When a standalone reference is required, prefer generating it
from the owner over maintaining it by hand.

## Negative-state rule

Remove absent or retired capabilities unless the limitation changes a current
action. Keep actionable limits precise and pair them with the supported path.

## Choose the reference

- Agent guidance or retrieval structure: [agent-first.md](references/agent-first.md).
- README, contributor, security, or deep-doc placement: [documentation.md](references/documentation.md).
- Private, cross-repo, or machine-local evidence; saving a durable rule:
  [source-boundaries.md](references/source-boundaries.md).
- Long-lived feature contracts or decisions: [specifications.md](references/specifications.md).
- Writing defaults when the owner has no stronger convention:
  [style.md](references/style.md).

Do not invent contributor/security policy, create a second backlog, implement
missing readiness infrastructure, or turn docs cleanup into runtime
verification. When the request includes those tasks, keep their ownership
distinct and continue separately authorized work.

Check changed links, commands, and moved references against current sources.
Use the repository's doc checks; running a documented command is a separate
action whose scope and side effects still need authorization. Report what
changed, what was verified, and any remaining gap. Do not create a report file
unless requested or required by the repository.
