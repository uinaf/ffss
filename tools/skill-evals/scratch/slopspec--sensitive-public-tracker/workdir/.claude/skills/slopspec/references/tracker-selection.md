# Tracker Selection

Choose the repository's actual work system, not the easiest available tool.

## Evidence Precedence

| Evidence | Meaning |
|---|---|
| User names a tracker, project, team, epic, or issue | Authoritative for this request |
| Current work began from a tracker URL or key | Prefer updating or attaching to that work |
| `AGENTS.md`, `CLAUDE.md`, `CONTRIBUTING.md`, or repo docs declare work tracking | Repository default; overrides remote-host inference |
| Repeated issue keys or links in branches, commits, and pull requests | Strong established-workflow signal |
| Issue templates, labels, or tracker-specific config | Strong provider and artifact-shape signal |
| Git remote host | Recommendation signal, not proof of the tracker |
| Installed CLI or connector | Access signal only; it does not choose the destination |

When strong evidence conflicts, report the conflict and ask one question with a recommended answer. Do not create artifacts in both places.

## Common Signals

### GitHub

- GitHub remote plus active repository issues
- `.github/ISSUE_TEMPLATE/` or issue-form configuration
- `owner/repo#123` references and GitHub issue URLs
- `gh` or an authenticated GitHub connector as the write path

### GitLab

- GitLab or self-hosted GitLab remote
- `.gitlab/issue_templates/`
- project or group issue references and GitLab issue URLs
- `glab` or an authenticated GitLab API/connector as the write path

### Jira

- Repository instructions naming the Jira site and space/project
- repeated keys such as `ABC-123` in branches, commits, pull requests, or docs
- an existing epic or work item named by the user
- an authenticated Jira connector or API as the write path

### Linear

- Repository instructions naming the workspace and team
- repeated team keys such as `ENG-123` with Linear URLs or integration evidence
- an existing project or parent issue named by the user
- an authenticated Linear connector or API as the write path

### Local or another tracker

Use a local directory, Notion database, Asana project, or another system only when the user or repository establishes it as the work tracker. Preserve its native vocabulary and hierarchy instead of translating everything into GitHub terms.

## When to Ask

Ask where to save the plan when:

- the user requested planning but did not authorize publication
- no repository preference is discoverable
- multiple plausible trackers conflict
- the provider is known but the project, team, or parent is ambiguous
- the plan would expose sensitive information to a public or broadly visible tracker

Lead with one recommendation:

> Save this as a parent issue in `<destination>` with `<N>` child tickets? Recommended because `<repo evidence>`.

Do not ask when the user already named the destination or requested publication into an unambiguous repository-owned tracker.

## Access and Failure

Before writing, verify the target, authenticated identity or profile when relevant, repository or project visibility, and permission to create or update the intended artifact type. Do not install integrations, change authentication, create tracker projects, invent labels, or broaden visibility merely to publish a plan.

When native hierarchy or dependency operations are unavailable, retain the chosen tracker and use explicit parent and blocker links in item bodies. When no write path is available, produce a paste-ready draft and name the exact missing capability.
