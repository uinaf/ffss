# Artifact shapes

Use the repository's templates first. Otherwise choose the least hierarchy that
keeps work independently verifiable and resumable.

| Work | Shape |
| --- | --- |
| One cohesive outcome | One issue or work item |
| Several independently landable slices | Parent with child work items |
| Multiple teams or planning horizons | Existing project/initiative convention |
| Tracker unavailable | Paste-ready draft for that tracker |

A canonical item carries the outcome, supporting evidence, settled decisions,
acceptance, boundaries, relevant verification, and material risks or stop
conditions. A parent adds child outcomes and dependencies; each child names its
parent and the proof of its own delivered behavior. Omit empty sections; no
epic exists to hold one issue.

When an unverified assumption could invalidate dependent work, block only that
work on a bounded validation and record the result that permits or stops it.

Slice through the layers needed to deliver behavior. Size work for one fresh
agent context, including verification. Do not split code, docs, and tests into
tickets when none can land alone. For broad migrations where vertical slices
cannot stay valid: expand, migrate in reviewable batches, then contract.

A blocking edge means the dependent cannot start or verify without its blocker.
Shared topic or preferred order is insufficient. Keep independent work parallel.
Verify included paths and record the revision when drift matters.

Keep current requirements and status in the canonical body or fields;
comments may hold progress history.
