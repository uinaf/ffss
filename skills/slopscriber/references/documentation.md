# Repository Documentation

Use one clear responsibility per document. Route readers to deeper detail without duplicating it.

## Contents

- [Top-Level Doc Split](#top-level-doc-split)
- [README.md](#readmemd)
- [CONTRIBUTING.md](#contributingmd)
- [SECURITY.md](#securitymd)
- [Docs Section](#docs-section)
- [Architecture Docs](#architecture-docs)
- [Repo Docs vs Agent Work Artifacts](#repo-docs-vs-agent-work-artifacts)
- [Keep Docs Alive](#keep-docs-alive)

## Top-Level Doc Split

Use a small default top-level set with one responsibility per file:

- **`README.md`** — what the project is, how to install it, how to use it
- **`CONTRIBUTING.md`** — contributor setup, validation commands, branch/PR workflow
- **`SECURITY.md`** — private-first vulnerability reporting path and boundaries
- **`LICENSE`** — legal terms, not contributor instructions

Split the four responsibilities across focused docs unless the repo is tiny enough that one file is clearer.

Use this split to clean up current docs, redistribute content from an overloaded
README, or keep existing files accurate. Creating baseline contributor,
security, issue, and pull request templates from scratch is repository
governance setup, because those files encode collaboration policy.

General doc-surface rules:

- Visible labels should describe purpose, not filenames. Prefer `Contributing`, `Release workflow`, or `Architecture` over `CONTRIBUTING.md` or `docs/RELEASE.md`
- Use consistent casing for visible prose and navigation. Reserve all-caps names or extension-bearing filenames for literal file references only
- Order sections and link lists by reader task flow or importance, not by directory layout or implementation order
- Apply the same rule to agent-facing docs and internal guides, not just user-facing README copy

## README.md

Use this default order unless the repo gives you a strong reason not to:

1. **Hero** — name + one-sentence purpose
2. **Install** — fastest path to getting it running or consuming it
3. **Quick usage** — one first successful flow
4. **Optional examples / variants / integration notes**
5. **Docs** — compact navigation to deeper material
6. **Contributing** — short pointer to `CONTRIBUTING.md`
7. **License** — short pointer to `LICENSE`

Guidance:

- **Lead** with one sentence: what the project is and why it exists
- **Put the fastest path to value near the top**: install, quickstart, docs, or demo
- **Link out** to deeper docs instead of duplicating them
- Keep contributing and license sections short
- For package repos, show install plus one short usage example
- For app repos, keep end-user usage in `README.md` and move contributor setup to `CONTRIBUTING.md`
- Use human-facing labels in README navigation. Prefer `Contributing`, `Architecture`, `Distribution`, or `Security` over raw filenames or paths like `CONTRIBUTING.md` or `docs/RELEASE.md`
- Order navigation for readers, not for the filesystem. Put the most useful docs first instead of mirroring path order
- Apply the same rule to section titles and callouts around the README, not just the bullet links

### Workspace or coordination repos

For workspace, meta, or coordination repos, keep the README even more routing-oriented:

1. **Hero** — what the workspace owns
2. **Quick start** — shortest path into an already prepared machine or environment
3. **Workspace model** — layout, naming, source-of-truth registry
4. **Key docs** — compact links to setup, strategy, and agent guidance
5. **Skills** — short pointer to the canonical setup doc if shared skill installation exists
6. **Contributing** — short pointer to `AGENTS.md` or `CONTRIBUTING.md`

Guidance:

- Keep one canonical setup doc for machine bootstrap or shared install commands
- Let `README.md` point to that setup doc instead of duplicating full bootstrap flows
- Keep peer-repo or multi-repo layout explanations short and source-of-truth oriented
- Put cross-repo reference lists in deeper docs or in the peer repos when they are not part of the top-level user path

### Shape selection

- **Minimal product**: short value prop, one docs link
- **CLI/package**: install first, then quickstart, then docs links
- **Product + contributor**: short intro, install, usage, docs, contributing
- **With navigation/examples**: TOC, visual demo, usage examples

## CONTRIBUTING.md

Use this section when updating an existing contributor guide or moving current
contributor instructions out of an overloaded README. If the repo has no
contributor policy yet, flag the gap instead of inventing one from scratch.

Use this default order unless the repo gives you a strong reason not to:

1. **Setup**
2. **Run locally**
3. **Validation**
4. **Development notes**
5. **Pull request expectations**
6. **Release notes** only if contributors genuinely need them

Guidance:

- Put environment bootstrap first
- Keep commands copy-pastable and verified against the repo
- Include only contributor-facing commands here: install toolchain, install dependencies, run locally, run checks
- Keep repo-specific development notes only when they materially help contributors
- Link deeper docs instead of letting `CONTRIBUTING.md` turn into a handbook
- Keep visible labels and headings contributor-facing rather than file-oriented here too. For example, prefer `Pull request expectations` over a heading or bullet that just echoes a filename

## SECURITY.md

Use this section when updating an existing security policy or moving current
vulnerability-reporting instructions out of an overloaded README. If the repo
has no private reporting route, flag the gap instead of guessing one.

Keep it short and private-first.

Default shape:

1. **Contact**
2. **Scope**
3. **Guidelines**
4. **Supported versions**
5. **Disclosure**

Guidance:

- Tell reporters not to open public issues for vulnerabilities
- Use the repo's real security contact; do not guess
- Link from `README.md` only when it helps navigation instead of crowding the user flow

## Docs Section

When `README.md` has a `Docs` section, keep it compact and canonical.

- Link to deeper docs without dumping their contents into the README
- Common links: About, Guides, Architecture, Deployment, Security
- Treat it as reader navigation. Use human labels, not raw filenames or paths
- Keep `Contributing` and `License` in their own sections when the README already has those sections
- Put agent-only or generated docs last, or move them into a small `Repo Internals` section when that reads better
- Keep the same navigation list in one canonical place
- Keep it skimmable
- Carry the same ordering rule into other visible lists and sections across the doc surface. If a sequence reads like filesystem order instead of reader priority, rewrite it

## Architecture Docs

- `docs/ARCHITECTURE.md` — diagram-first system view and important boundaries
- task-specific current-state references such as API, deployment, operations, and guides
- decision docs when the architectural boundary depends on why a choice was made

## Repo Docs vs Agent Work Artifacts

Do not treat every markdown file as the same kind of documentation.

Repo docs describe the current project for readers, users, and contributors:

- `README.md`, `CONTRIBUTING.md`, `SECURITY.md`
- architecture, API, deployment, operations, and runbook docs
- setup and validation commands that are true for this repo now

Agent work artifacts coordinate how agents change the repo:

- `AGENTS.md` and scoped agent guidance for repeated behavior
- `docs/specs/` for durable behavioral contracts
- `docs/decisions/` for durable choices and trade-offs
- `docs/plans/` for tactical execution that should disappear when done
- handoff prompts only when they help future agents resume work

Keep the layers connected with short links. Do not pour implementation plans, prompt text, or unfinished reasoning into reader-facing docs. Promote an artifact into general docs only after it becomes a stable contract the repo owns.

## Keep Docs Alive

- After implementing a feature → check if AGENTS.md, README, or architecture docs need updating
- After renaming/moving/deleting code → grep docs for stale references
- After a design decision → record it in a decision doc before moving on
- Treat doc drift the same as test failure — it degrades every future agent's performance
