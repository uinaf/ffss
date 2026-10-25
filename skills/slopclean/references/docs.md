# Docs and agent guides

Check every claim against its owning source. Keep what changes a reader's
decisions.

## What stays

- **Keep:** owners, non-obvious prerequisites, boundaries, repo conventions,
  verification scope, recovery paths.
- **Cut:** generic advice, directory tours, duplicates, and facts a quick
  lookup gives.
- **No mirrors:** a mirror restates what code, CI, or config already says:
  how a release, deploy, workflow, or tool works, which keys or tasks exist,
  what a job touches. It goes stale on the next change. Delete it and link the
  file and key. Behaviour specs, architecture overviews, testing guides, and
  test or lane catalogs are mirrors too: a rule a test pins is documented by
  that test. Docs keep only what a reader must do or must not break that no
  file, test, or check states.
  Exception: README and `AGENTS.md` carry the exact install, setup, run, and
  verify commands.
- **Current state only:** cut retired or absent systems, their unset
  settings, history, and "there is no X" sections. Keep a limit only when a
  reader would otherwise build or call the wrong thing, and put the supported
  path beside it.
- **Repo-owned only:** private workspaces, local helpers, accounts, hosts,
  internal URLs, and one-off observations stay out unless the owner adopts
  them. Describe what you left out by category, without the values, and note
  that promoting it needs the maintainer.
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
- **README:** purpose, install and start, first use, links. Contributor setup
  goes in CONTRIBUTING, private reporting in SECURITY. Don't invent contacts
  or promises.

Check changed links, commands, and moved references against current sources.
