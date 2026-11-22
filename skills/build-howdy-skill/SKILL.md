---
name: build-howdy-skill
description: "Build or maintain a private `howdy` skill: a workspace-local attention brief over this machine's trackers, mail, chat, and calendars. Use when asked to set up howdy, a morning brief, or \"what needs me\" on a machine, or when an existing howdy brief shows errors, empty sections, dead sources, renamed tools, or missing repos."
---

# Build howdy skill

Write a `howdy` skill for one user on one machine, from sources this machine
can reach. The generated skill holds the user's accounts and projects, so it
lives only in their workspace. This skill stays generic; so does anything you
write outside that workspace.

| Ask | Mode |
| --- | --- |
| no howdy skill yet, or "start over" | [Build](#build) |
| a howdy skill exists | [Maintain](#maintain) |

## Build

1. **Locate.** Find the workspace and its skills directory from its agent
   guide, else the harness's project skills directory. Target
   `<skills>/howdy/SKILL.md`. If the workspace is pushed anywhere public,
   use a path `git check-ignore` confirms, or one outside the repository,
   and say which.
2. **Inventory.** Read what the machine and workspace show before asking:
   the project or repo list, configured MCP servers (`.mcp.json`, the
   harness's MCP list), and CLIs on `PATH` (`gh`, `glab`, `jira`, `gog`, mail
   and calendar tools). Each question's options come from this inventory.
3. **Interview.** One question at a time: the decision, options from the
   inventory, and a recommendation. Ask the first now and hold the rest
   until it's answered; write no skill on guessed sources. Skip what the
   inventory already settles and state it instead. In order:
   1. Trackers: GitHub, GitLab, Jira or Linear, and through which CLI or MCP.
   2. Scope: which owners, repos, and projects count.
   3. Mail and chat: Gmail, HEY, Slack, others; which folders or channels.
   4. Calendars.
   5. "Needs me": what makes an item the user's move (review requested by
      name, own red CI, a ticket state, an unanswered DM, an event today).
   6. Knowledge: where docs and decisions live.
   7. Lanes: does the user run parallel agent lanes on epics?
4. **Probe.** Run one read-only call per source: a search or list, never a
   send, post, comment, label, or mark-read. Record the exact command or MCP
   tool name that worked. Done when every source has a working call, or the
   user decided on its failure: fix now, drop, or keep marked unverified. A
   source checked only by the user counts when they pasted its output.
5. **Write** the [generated skill](#generated-skill).
6. **Report**: the path, each source with its probe result, and what stays
   unverified.

## Maintain

1. Read the generated skill, then redo Inventory and Probe for every source,
   including the ones nobody complained about.
2. Classify each difference as **drift**:
   - dead source: auth expired, MCP server or CLI gone;
   - renamed tool, flag, owner, or repo;
   - scope change: a repo or project added or archived in the workspace list;
   - a new source on the machine the skill doesn't use.
3. Fix mechanical drift in place (renames, scope changes) and keep the
   user's own edits. Take a dead source or a new one to the user, one
   [Interview](#build) question each; until answered, mark the row
   unavailable rather than delete it.
4. Report one line per drift: what changed and what you did. Done when every
   row in the source table has a fresh probe result or a pending question.

## Generated skill

- Frontmatter: `name: howdy` and a `description` that leads with what it
  does, then its triggers ("howdy", "what needs me", "the brief").
- **Sources**: one table row per source with the exact command or MCP tool
  and its filters. Read only. A source that fails at run time becomes one
  line in the brief, and the sweep continues.
- **Brief**: lead with the count that needs the user, then:
  1. **Needs you**: the user's own definition from the interview.
  2. **This week**: dated items and replies due in 7 days.
  3. **In flight**: open work, and who holds the next move.
  4. **Since last time**: shipped, resolved, newly opened.
  One fact and one link per line.
- **Lanes**, when the user runs them: the Needs you view opens with a lane
  board in the shape of [slopflow's board](../slopflow/SKILL.md#board).
- **Done**: every non-empty source lands in exactly one section or is named
  as a failed source.
- **Upkeep**: one line saying that a failing source, a renamed tool, or a
  changed repo list means re-running this generator in Maintain mode.

Mail appears as sender and subject; bodies stay unread unless a subject
can't say what is owed.
