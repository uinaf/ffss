---
name: slopclean
description: "Remove AI tells from prose, code, or tests when asked to unslop or humanize an artifact; preserve its meaning and behavior."
---

# Slopclean

Clean the requested artifact in place while preserving meaning, facts, behavior,
and public API. Follow house conventions; one pass, not a sterilizing loop.

Read the matching reference; source-and-test diffs need both:

- [Prose](references/prose.md): cut puffery, vague claims, chatbot phrases, and
  repetitive scaffolding. Prefer concrete facts; preserve the author's voice.
- [Code](references/code.md): remove narrated comments, pointless forwarding,
  proven-unused private options, and misleading internal names. Call count alone
  does not make a useful boundary wasteful.
- [Tests](references/tests.md): replace tautologies, incidental change detectors,
  overmocking, and repetitive fixtures with checks of observable contracts.

Never weaken an assertion or delete the only coverage of a behavior; replace it
in the same pass or report the gap. Preserve public options even when their
implementation ignores them. Report real defects and contract changes separately.
Do not invent opinions or force stylistic variety to make prose seem human.
