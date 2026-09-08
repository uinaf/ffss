# Draft a Proven Configuration Change

Prepare one GitHub issue draft in `planning-result.md` for the agreed work
below. Do not publish or implement it. GitHub Issues is the repository's
canonical engineering tracker.

## Input Files

=============== FILE: agreement.md ===============
Rename CACHE_TTL_SECONDS to CACHE_MAX_AGE_SECONDS. Accept both names for one
release, emit the existing deprecation warning for the old name, and document
the new key. Add the relevant compatibility tests in the same change.

The configuration loader already supports aliases; the existing alias contract
tests passed on the current base. The maintainer settled the compatibility
window and precedence: the new key wins if both are supplied. There are no
unresolved feasibility questions. This is one independently landable change
that fits a fresh agent context.
=============== END FILE ===============
