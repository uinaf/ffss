# Planning result — SDK-41 retry policy

## Durable destination

**Linear, team `SDK`, issue [SDK-41](https://linear.app/acme/issue/SDK-41/retry-policy).**

| Evidence | Weight | Reading |
|---|---|---|
| `AGENTS.md:3` — "Linear team SDK is canonical for planned product work" | Repository default; overrides remote-host inference | Linear |
| Conversation began from `https://linear.app/acme/issue/SDK-41/retry-policy` | Prefer updating or attaching to that work | SDK-41 |
| `AGENTS.md:3` — "GitHub issues are used only for external bug reports" | Explicit exclusion for planned work | Not GitHub |
| `.git/config` remote `git@github.com:acme/sdk.git` | Recommendation signal only, and already overridden | Not GitHub |
| `gh` installed at `/opt/homebrew/bin/gh`; no `linear` CLI, no Linear connector | Access signal only; does not choose the destination | Not GitHub |

The GitHub remote is not a conflict to escalate — the repository declares the tracker and the source work item confirms it. No GitHub issue is created.

## Update, do not create

- **Update** SDK-41 in place. The two agreed changes are additional acceptance criteria on the same outcome, not new deliverables.
- **No new issue, epic, or project.** The work is one agent, one fresh context, one verifiable outcome. An epic to hold one issue is not warranted.
- **No `docs/plans/` or checked-in plan file.** This `planning-result.md` is the requested report, not the durable home.
- Keep the existing title `Add configurable retry policy` and the existing `## Outcome` text unchanged.

## Content that should change

Two checkboxes appended to `## Acceptance criteria`; existing default-attempts criterion retained and unmodified. Paste-ready SDK-41 body:

```markdown
## Outcome
SDK consumers can configure retry limits.

## Acceptance criteria
- [ ] The default remains three attempts.
- [ ] Configuration parsing rejects a negative retry count.
- [ ] Zero retries is documented as disabling retry behavior.
```

Diff against the current body: 2 lines added, 0 changed, 0 removed.

## Write path — blocked

Linear cannot be written from this session: no `linear` CLI on `PATH` and no authenticated Linear connector or MCP tool is exposed. The destination stays Linear; the block is access, not choice.

**Missing capability:** an authenticated Linear write path (connector, MCP server, or API token) for workspace `acme`, team `SDK`.

Until then, apply the body above to SDK-41 by hand, or authorize a Linear connector and this can be published directly.

## Verified resume point

```text
canonical: SDK-41 (update pending — not yet applied)
children: none
frontier: SDK-41 (no blockers)
gaps: no Linear write path; body above is unpublished
next: paste the body into SDK-41, then implement against its three acceptance criteria
```

**Verified:** all four input files read; `linear`/`gh`/`glab` presence checked on `PATH`. **Unverified:** SDK-41's live body — the local `CURRENT_ISSUE.md` snapshot is the only copy available, so confirm no one has edited the issue before pasting. No source paths are cited because none were inspected in this revision.
