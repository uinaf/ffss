# Agent-first documentation

Keep guidance that changes a capable agent's decisions. Removing words helps
only if the required context stays discoverable.

## Select information

Keep canonical owners, non-obvious prerequisites, architectural boundaries,
repo-specific conventions, verification scope, and known recovery paths.
Remove generic engineering advice, directory tours, duplicated instructions,
and facts a cheap unambiguous lookup supplies. Link maintained scripts,
configs, schemas, and tests instead of translating them into prose. Preserve
rationale the source cannot explain.

Describe current capabilities. Keep a limitation only when it changes an action,
with the supported alternative; see the [negative-state rule](../SKILL.md#negative-state-rule).

## Retrieval

A pointer says when to open its target and what question it answers. Keep
common requirements near the entrypoint and specialized rules near their
scope. Split by task, not file length: another link adds a lookup, so do not
fragment material the same task always needs together.

For skills, keep discovery descriptions short and specific to the requested
capability; broad triggers pull routine work into a specialized workflow. Keep
essential constraints in `SKILL.md`; route setup, recovery, and mode-specific
detail to references when not needed on every use.

Use task-shaped headings and stable terms. Preserve literal commands, paths,
and diagnostic identifiers for search. Runbooks need observable outcomes and
recovery guidance, not the operator's intentions.

## AGENTS.md

Put guidance at its owner:

- Global: identity, workspace routing, and harness policy.
- Repository: system purpose, critical outcomes, shared hazards, code-placement
  rules, exact lifecycle commands and their proof limits, and durable write-back.
- Scoped: package, language, or subsystem rules needed only there.

The root is a compact operating contract, not a contents page. Inline facts
that affect many tasks or would cause a material error if missed; link deep
architecture and local conventions. A completeness matrix earns its space when
omissions across clients, providers, or modes recur.

Make completion and authority concrete: name the requested outcome and the
actions that need a user decision. Preserve existing authorization across
checks, fixes, and delivery; avoid mandatory reading lists, repeated testing
reminders, and review checkpoints without a task-specific reason. Keep shared
guidance model-neutral and revisit constraints whose rationale no longer holds.

Keep one authored source. When a repository uses AGENTS.md, use a CLAUDE.md
symlink or supported `@AGENTS.md` import, not a copy. Preserve an existing
valid import. Check current harness documentation before changing hierarchy,
filenames, or import behavior: [Codex](https://developers.openai.com/codex/guides/agents-md)
and [Claude Code](https://code.claude.com/docs/en/memory).

## Capture proven workflows

Save a reusable workflow as an owning skill or runbook after it has worked,
with its task, prerequisites, useful decisions, proof, and recovery. Do not
formalize a first attempt or keep a session transcript as instructions.
