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
parent and the proof of its own delivered behavior. Omit empty sections and
avoid an epic whose only purpose is holding one issue.

Slice through the layers needed to deliver behavior. Size work for one fresh
agent context, including verification. Do not split code, docs, and tests into
independent tickets when none can land alone. For broad migrations, expand,
migrate in reviewable batches, then contract when independent vertical slices
cannot remain valid.

A blocking edge means the dependent cannot start or verify without its blocker.
Shared topic or preferred order is insufficient. Keep independent work parallel.
Verify included paths and record the revision when drift matters.

A fresh agent must be able to identify scope, settled choices, completion proof,
finished work, blockers, and what is ready now. Keep current requirements and
status in the canonical body/fields; comments may retain progress history.
