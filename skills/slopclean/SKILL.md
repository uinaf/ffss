---
name: slopclean
description: "Remove AI tells from prose, code, or tests when asked to unslop or humanize an artifact; preserve its meaning and behavior."
---

# Slopclean

Clean the requested artifact in place. Meaning, facts, behavior, and public API
do not change; house conventions win. One pass, not a sterilizing loop.

Read the matching reference; source-and-test diffs need both:
[prose](references/prose.md), [code](references/code.md),
[tests](references/tests.md).

Never weaken an assertion or delete the only coverage of a behavior; replace it
in the same pass or report the gap. Report real defects and contract changes
separately instead of fixing them in the pass.
