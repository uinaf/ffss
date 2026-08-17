# Repository Documentation

Use one clear responsibility per document. Order information by reader task,
use human-facing labels, and link to deeper detail instead of duplicating it.

## Top-Level Split

| File | Responsibility |
| --- | --- |
| `README.md` | product value, install, first successful use, deeper docs |
| `CONTRIBUTING.md` | contributor setup, validation, development and PR workflow |
| `SECURITY.md` | private reporting route, scope, and disclosure boundaries |
| `LICENSE` | legal terms only |

Use this split to redistribute current content from an overloaded README. Do
not invent contributor, security, issue, or pull-request policy when the
repository has not established it.

## README

Default order:

1. Name and one-sentence purpose
2. Fastest install or start path
3. One successful usage flow
4. Optional examples or variants
5. Compact links to deeper docs
6. Short contributing and license pointers

Package repos need install plus one minimal usage example. App repos keep
end-user use in the README and contributor setup elsewhere. A workspace or
coordination repo may replace usage with its ownership model, quick start, and
source-of-truth registry.

Do not mirror filesystem order. Put the most useful destination first and use
labels such as `Architecture`, `Release workflow`, or `Security` unless the
literal filename matters.

## Contributing

Update an existing contributor guide or move existing policy here in this
order:

1. Setup
2. Run locally
3. Validation
4. Repo-specific development notes
5. Pull-request expectations
6. Release notes only when contributors need them

Keep commands copyable and verified. Link release, deployment, architecture,
and lengthy policy instead of turning the guide into a handbook. If the repo
has no contributor policy, report the gap rather than drafting governance from
taste.

## Security

Keep an existing policy short and private-first:

1. Working private contact or reporting surface
2. Supported scope or versions
3. Information reporters should include
4. Disclosure boundaries

Tell reporters not to disclose vulnerabilities in public issues. Never guess a
contact, support promise, or private-reporting capability.

## Deep Docs

Use task-specific current-state guides for architecture, APIs, deployment,
operations, and recovery. Architecture docs should explain the system and
important boundaries, preferably with the smallest useful diagram. Decision
records explain why a consequential choice was made.

Keep one canonical navigation list. Do not repeat it across README sections,
contributor docs, and agent guidance.

## Documentation vs Work Artifacts

Reader-facing documentation describes the current product or repository.
Agent work artifacts coordinate how it changes:

- `AGENTS.md` and scoped guidance: repeated operating behavior
- `docs/specs/`: durable behavioral contracts
- `docs/decisions/`: durable choices and trade-offs
- the repository tracker: tactical execution and resumable status

Connect them with short links. Do not pour implementation plans, prompt text,
or unfinished reasoning into reader-facing docs. Promote an artifact only after
it becomes a stable contract the repository owns.

## Maintenance

- After behavior changes, check the reader and agent surfaces that describe it.
- After renames or deletions, search docs for stale paths and commands.
- After consequential choices, update the owning decision or specification.
- Treat doc drift as a failed repository contract, not cosmetic debt.
