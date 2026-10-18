# Docs and agent guides

Clean repository docs and agent guidance against their owning sources. Keep
what changes a capable agent's or reader's decisions; removing words helps only
if the required context stays discoverable.

## Select

- Keep canonical owners, non-obvious prerequisites, architectural boundaries,
  repo-specific conventions, verification scope, and recovery paths. Remove
  generic engineering advice, directory tours, duplicated instructions, and
  facts a cheap unambiguous lookup supplies.
- Code and structured config own the facts they express: link the owning file
  and its key, section, or symbol instead of keeping prose copies of
  inventories, defaults, schemas, or task lists. Entrypoints are the
  exception: README and `AGENTS.md` name the exact install, setup, run, and
  verify commands. Verify the source is readable by the intended reader before
  replacing prose with a link.
- Document what exists. Remove absent, retired, or never-deployed systems,
  their unset settings, history, and "there is no X" summaries. Keep a limit
  only when a reader would otherwise build or call the wrong thing, and state
  it beside the supported path.
- Checked-in docs describe contracts their repository owns. Cite upstream
  products by their maintained source, link other repositories, and keep
  private workspaces, local helpers, accounts, hosts, and one-off observations
  out unless the owner adopts them. Describe what you excluded by category,
  without repeating the sensitive values.

## Place

- `AGENTS.md`: a compact operating contract, not a contents page. System
  purpose, shared hazards, code-placement rules, exact lifecycle commands, a
  map from change class to its check, and the actions that need a user
  decision. Inline what affects many tasks or would cause a material error if
  missed; link deep architecture and scoped rules. Drop mandatory reading
  lists, repeated testing reminders, review checkpoints, and commit caps
  without a task-specific reason.
- One authored source: when a repository uses `AGENTS.md`, `CLAUDE.md` is a
  regular file containing only `@AGENTS.md`, not a copy or a symlink.
- README: purpose, install and start, first successful use, links to deeper
  docs. Contributor setup goes in CONTRIBUTING, the verified private reporting
  route in SECURITY; never invent contacts, support promises, or governance.
- A pointer says when to open its target and what question it answers. Split
  by task, not file length.

Check changed links, commands, and moved references against current sources.
Running a documented command is a separate action that still needs its own
authorization.
