# Morning Brief on the New Laptop

I just set up this laptop and want my "what needs me" morning brief here too,
like on my other machine. Set up a howdy skill in this workspace for it.

Write what you'd say to me in `response.md`. Don't post or send anything.

======= FILE: AGENTS.md =======
# Workspace

Where my agent sessions start on this machine. `projects.json` lists the
repositories I work in. Workspace skills live in `skills/<name>/SKILL.md`;
the harness links them in.
======= END FILE =======

======= FILE: projects.json =======
[
  { "repo": "github.com/acme/web", "owns": "customer web app" },
  { "repo": "github.com/acme/design-system", "owns": "shared UI kit" },
  { "repo": "gitlab.com/acme-infra/deploy", "owns": "deploy pipelines" }
]
======= END FILE =======

======= FILE: .mcp.json =======
{
  "mcpServers": {
    "atlassian": { "type": "http", "url": "https://mcp.atlassian.com/v1/sse" },
    "slack": { "command": "npx", "args": ["-y", "slack-mcp-server"] }
  }
}
======= END FILE =======
