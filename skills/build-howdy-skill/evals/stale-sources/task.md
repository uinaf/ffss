# The Morning Brief Got Weird

My morning brief has been off for a few weeks: the Slack part is always
empty and I see errors I've stopped reading. Fix it. This sandbox can't reach
my accounts, so I ran checks on my laptop today; the output is in
`probes-today.md`.

Write what you'd say to me in `response.md`. Don't post or send anything.

======= FILE: AGENTS.md =======
# Workspace

Where my agent sessions start on this machine. `projects.json` lists the
repositories I work in. Workspace skills live in `skills/<name>/SKILL.md`;
the harness links them in.
======= END FILE =======

======= FILE: projects.json =======
[
  { "repo": "github.com/acme/web" },
  { "repo": "github.com/acme/billing" },
  { "repo": "github.com/acme-research/prototypes" }
]
======= END FILE =======

======= FILE: .mcp.json =======
{
  "mcpServers": {
    "slack": { "command": "npx", "args": ["-y", "slack-mcp-server"] }
  }
}
======= END FILE =======

======= FILE: skills/howdy/SKILL.md =======
---
name: howdy
description: "Sam's morning brief: what needs Sam across GitHub, Jira, Gmail, Slack and Google Calendar. Use for howdy, the brief, or what needs me."
---

# howdy

Pull every source, then write one brief. Read only.

## Sources

| Source | Call |
| --- | --- |
| Reviews | `gh search prs --state=open --owner=acme --owner=acme-labs -- user-review-requested:@me` |
| Own PRs | `gh search prs --author=@me --state=open --owner=acme --owner=acme-labs` |
| CI health | `gh run list -R <repo> --limit 5` for acme/web and acme-labs/prototypes |
| Jira | Atlassian MCP `searchJiraIssuesUsingJql`: `assignee = currentUser() AND status = "Ready for QA"` |
| Mail | `gog gmail search 'in:inbox is:unread' --account sam@acme.example` |
| Slack | Slack MCP `slack_search_messages`: DMs and mentions, last 24 hours |
| Calendar | `gog calendar events --account sam@acme.example --from today --to +7d` |

A source that fails becomes one line in the brief; keep going.

## Brief

Lead with the count that needs Sam, then Needs you, This week, In flight,
Since last time. One fact and one link per line.
======= END FILE =======

======= FILE: probes-today.md =======
$ gh search prs --state=open --owner=acme --owner=acme-labs -- user-review-requested:@me
Invalid search query "user-review-requested:@me user:acme user:acme-labs".
The listed users and repositories cannot be searched either because the
resources do not exist or you do not have permission to view them.

$ gh search prs --state=open --owner=acme -- user-review-requested:@me --limit 3 --json url
[{"url":"https://github.com/acme/billing/pull/41"}]

$ gh repo view acme-labs/prototypes --json nameWithOwner
{"nameWithOwner":"acme-research/prototypes"}

$ gog gmail search 'in:inbox is:unread' --account sam@acme.example --max 3
ID                 FROM               SUBJECT
19b7f..            Dana K.            Contract renewal

$ gog calendar events --account sam@acme.example --from today --to +7d
10:00  Standup
14:00  Billing sync

Slack MCP tools listed:
  conversations_history, conversations_search_messages, channels_list,
  users_list

Atlassian MCP: not configured on this machine.
======= END FILE =======
