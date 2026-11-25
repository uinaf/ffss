# Docs and agent guides

Check every claim against its owning source. Keep what changes a reader's
decisions.

## What stays

- **Keep:** owners, non-obvious prerequisites, boundaries, repo conventions,
  verification scope, recovery paths.
- **Cut:** generic advice, directory tours, and duplicates.
- **No mirrors:** a mirror restates what code, CI, or config already says:
  how a release, deploy, workflow, or tool works, which keys or tasks exist,
  what a job touches. Delete it and link the file and key. Behaviour specs,
  per-screen or per-feature walkthroughs, testing guides, and test or lane
  catalogs are mirrors too: a rule a test pins is documented by that test.
  Exception: README and `AGENTS.md` carry the exact install, setup, run, and
  verify commands.
- **Diagrams over walls of text:** a doc that grows with every feature change
  is a mirror. Cut it to what changes only with the architecture: module
  boundaries, main flows, lifecycles, and state machines. Draw each as
  Mermaid (`flowchart`, `sequenceDiagram`, `stateDiagram-v2`) in place of the
  prose it covers, from the code. Name only modules,
  states, and events the code has. Screen and per-change detail stays in
  code, tests, and design boards. No exported images: they can't be diffed.
- **Automated work:** cut from agent guides and repo skills what CI/CD,
  hooks, Renovate, or bots do on their own: deploy and release steps,
  upgrade runbooks, what a hook or job runs, recovery for automated deploys.
  Keep the gates an agent runs, actions that need a user decision,
  never-rules that stop an agent breaking the automation, and steps an agent
  performs that nothing automates.
- **Current state only:** cut retired or absent systems, their unset
  settings, history, and "there is no X" sections. Keep a limit only when a
  reader would otherwise build or call the wrong thing, and put the supported
  path beside it. ADRs, decision logs, and hand-written changelogs are history
  too: commits and change request bodies own the why.
- **Repo-owned only:** private workspaces, local helpers, accounts, hosts,
  internal URLs, and one-off observations stay out unless the owner adopts
  them. Describe what you left out by category, without the values, and note
  that promoting it needs the maintainer. Public repositories stay
  self-contained: never move their docs to a private wiki or link one.
- **Shipped plans go:** delete research, plan, spec, and benchmark docs once
  their subject ships. Don't move them elsewhere.
- **One mode per doc:** tutorial, how-to, reference, or explanation
  ([Diátaxis](https://diataxis.fr/)). Move reference tables out of a
  tutorial, background out of a how-to, and argument out of reference into
  the doc that owns that mode, and link it.

## Where it goes

- **`AGENTS.md`:** purpose, shared hazards, where code goes, exact lifecycle
  commands, which check proves which kind of change, and actions that need a
  user decision. Link deeper docs, saying when to open each. No mandatory
  reading lists, test-after-every-edit reminders, review checkpoints, or
  commit caps.
- **`CLAUDE.md`:** when the repo uses `AGENTS.md`, a regular file containing
  only `@AGENTS.md`.
- **`docs/architecture.md`,** or a README section in a small repo: one
  sentence on what the system does, then its diagrams. It changes only when
  a boundary, flow, or state changes.
- **README:** purpose, install and start, first use, links. Contributor setup
  goes in CONTRIBUTING, private reporting in SECURITY. Don't invent contacts
  or promises.
