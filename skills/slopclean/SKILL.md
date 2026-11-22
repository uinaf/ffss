---
name: slopclean
description: "Remove AI tells from prose, code, or tests; prune low-value tests from a suite; or fix docs and agent guides that drifted from their sources or go stale on every change. Use when asked to unslop, humanize, or de-AI text, prune tests, fix doc drift, or cut a wall-of-text doc down."
---

# Slopclean

Clean the artifact in place. Prose and code take one pass; a suite prune
follows [its steps](references/tests.md#pruning-a-suite). Keep meaning, facts, behavior, and
public API; house style wins.

Read the matching reference (source-and-test diffs need both):
[prose](references/prose.md), [code](references/code.md),
[tests](references/tests.md), [docs and agent guides](references/docs.md).

Every contract keeps at least one automated proof the gate runs, which fails
when the contract breaks: a test at its strongest boundary, or a type,
schema, or lint check. Report real defects and contract changes separately.
