# Keep a Small Plan in One GitHub Issue

## Problem/Feature Description

The repository explicitly tracks engineering work in GitHub Issues. The user says: "Save this agreed change as an issue." The change renames one configuration key, supports the old key for one release with a warning, updates the configuration reference, and adds a compatibility test. The work fits one agent context and lands as one reviewable change.

Produce `planning-result.md` with the issue title and body shape that should be published. Do not implement the change.

## Input Files

=============== FILE: AGENTS.md ===============
# Agent guide

Engineering backlog: GitHub Issues in this repository. Apply the `ready` label only when acceptance criteria and verification are complete.
=============== END FILE ===============

=============== FILE: AGREED_CHANGE.md ===============
Rename CACHE_TTL_SECONDS to CACHE_MAX_AGE_SECONDS. Continue accepting the old name for one release and emit the existing deprecation warning. Update the configuration reference and add a compatibility test for both names.
=============== END FILE ===============
