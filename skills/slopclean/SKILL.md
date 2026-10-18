---
name: slopclean
description: "Remove AI tells from prose, code, or tests; prune low-value tests from a suite; or fix docs and agent guides that drifted from their sources. Use when asked to unslop, humanize, or de-AI text, prune tests, or fix doc drift."
---

# Slopclean

Clean the artifact in place, in one pass. Keep meaning, facts, behavior, and
public API; house style wins. In docs, fix any claim its source contradicts.

Read the matching reference (source-and-test diffs need both):
[prose](references/prose.md), [code](references/code.md),
[tests](references/tests.md), [docs and agent guides](references/docs.md).

Every behavior keeps a test that proves it. Report real defects and contract
changes separately instead of fixing them in the pass.
