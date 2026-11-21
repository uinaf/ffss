# Build My Morning Brief

You asked me the setup questions for my "what needs me" brief yesterday; my
answers are in `answers.md`. This sandbox can't reach my accounts, so I ran
the checks you'd run on my laptop and pasted the output in `probes.md`.

Build the howdy skill in this workspace now. Write what you'd say to me in
`response.md`. Don't post or send anything.

======= FILE: AGENTS.md =======
# Workspace

Where my agent sessions start on this machine. `projects.json` lists the
repositories I work in. Workspace skills live in `skills/<name>/SKILL.md`;
the harness links them in.
======= END FILE =======

======= FILE: answers.md =======
- Trackers: GitHub orgs `acme` and `acme-labs`; Jira project PLAT through the
  Atlassian MCP.
- Repos that count: everything in projects.json.
- Mail: Gmail through the `gog` CLI, account sam@acme.example.
- Chat: Slack through the Slack MCP; DMs and mentions only.
- Calendar: Google Calendar through `gog`, same account.
- Needs me: a review requested from me by name, my own PR with red CI, a
  Jira ticket assigned to me in "Ready for QA", a DM unanswered for over 4
  hours, a meeting today.
- Knowledge: Confluence space PLAT.
- Lanes: yes, I run parallel agent lanes on epics.
======= END FILE =======

======= FILE: projects.json =======
[
  { "repo": "github.com/acme/web" },
  { "repo": "github.com/acme/design-system" },
  { "repo": "github.com/acme-labs/prototypes" }
]
======= END FILE =======

======= FILE: probes.md =======
$ gh auth status
github.com
  ✓ Logged in to github.com account sam-acme (keyring)

$ gh search prs --state=open --owner=acme --owner=acme-labs -- user-review-requested:@me --limit 3 --json url
[{"url":"https://github.com/acme/web/pull/812"}]

$ gog gmail search 'in:inbox is:unread' --account sam@acme.example --max 3
ID                 FROM               SUBJECT
19a2c..            Priya N.           Q3 roadmap review

$ gog calendar events --account sam@acme.example --from today --to +7d
Error: googleapi: Error 403: Request had insufficient authentication scopes.

Slack MCP tools listed:
  slack_list_channels, slack_get_channel_history, slack_search_messages,
  slack_get_user_profile, slack_post_message
======= END FILE =======
