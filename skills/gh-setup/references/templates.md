# Collaboration Files

Use when introducing or aligning GitHub-facing templates and contributor
policy. First inspect a public `<owner>/.github` defaults repository: local
files override shared defaults, and shared files are not copied into clones or
release archives.

## Ownership

- Use owner-level defaults for policy true across every target repository.
- The pull-request template and `SECURITY.md` live in the owner defaults.
- Delete repository-local copies of them unless a repository genuinely
  diverges.
- `CONTRIBUTING.md` stays repository-local because it carries environment
  setup and repo-specific workflow.
- Licenses remain repository-local.
- Add a code of conduct only when an actual enforcement and contact owner
  exists.
- Shared issue templates are risky because any repository-local issue
  configuration disables the shared set.

Changing or creating a public defaults repository is a public policy change;
obtain authorization first.

## Pull Requests and Issues

A good owner default gives the body three headings that are the
problem-first flow itself, with guidance in comments (example:
[uinaf/.github](https://github.com/uinaf/.github)):

```md
## Problem

<!-- as the requester stated it, not the mechanism -->

## Solution

<!-- a short problem-lead sentence, then labeled bullets; never a
     paragraph wall. Name a risk only when there is a real one. -->

## Proof

<!-- only what CI cannot show: a screenshot, before/after numbers.
     Upload media with `gh pr create --attach ./file.png` or
     `gh pr comment <n> --attach ./file.mp4`; never commit it to the branch.
     Delete this section when CI covers everything. -->
```

Prefer the shared owner default over repository-local copies; delete local
overrides unless the repository genuinely needs different fields. Title
the way the repository titles merged work, outcome over mechanism. No
implementation inventories, no ceremonial checklists, and no extra
headings that restate the diff. The comments are the verbosity control:
agents fill templates literally, so guidance written there is the one
place it reliably lands.

### Media proof

A template that asks for screenshots or recordings without naming an upload
route gets the asset committed to the branch. Name the route in the comment:
`gh pr create --attach ./file.png` and `gh pr comment <n> --attach` (gh 2.99+,
images and video, github.com and GitHub Enterprise Cloud; video goes on a bare
line and takes no alt text). When asked to align an owner default or a set of
repository-local templates, audit each proof or review-notes block for that
line and add it where missing. Leave templates that already name a working
upload route.

Create issue forms only when their fields improve triage. Common distinct
routes are bug, feature, and (only when supported) question. Vulnerabilities
always route to `SECURITY.md`, never a public issue form.

## Security and Contributing

`SECURITY.md` should say not to file public vulnerabilities, point to a private
route that works for the repository visibility, request affected surface,
impact, minimal reproduction, and mitigations, and avoid response promises the
maintainer cannot meet.

- Public repositories may use GitHub private vulnerability reporting only
  after the setting is enabled and verified.
- Private repositories route to an existing private maintainer channel; do not
  promise the public Security-tab workflow.

`CONTRIBUTING.md` owns setup, canonical validation, and branch/PR expectations.
Link deeper release or deploy runbooks instead of copying them into README,
templates, and agent guidance.

Repository descriptions and topics should help humans route the project using
its real purpose, artifact type, language or framework, and canonical public
URL. Never leak private client, organization, host, or adjacent-repo facts into
public metadata.
