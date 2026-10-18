---
name: slopclean
description: "Remove AI tells from prose, code, or tests when asked to unslop or humanize an artifact; prune low-value tests from a suite; or clean up drifted docs and agent guides against their sources. Preserves meaning, behavior, and coverage."
---

# Slopclean

Clean the requested artifact in place. Meaning, facts, behavior, and public API
do not change; house conventions win. One pass, not a sterilizing loop;
a suite prune stops at its measured target. Docs are the exception on facts:
correct a claim that its owning source contradicts.

Read the matching reference; source-and-test diffs need both:
[prose](references/prose.md), [code](references/code.md),
[tests](references/tests.md), [docs and agent guides](references/docs.md).

Never weaken an assertion or delete the only coverage of a behavior; replace it
in the same pass or report the gap. Report real defects and contract changes
separately instead of fixing them in the pass.
