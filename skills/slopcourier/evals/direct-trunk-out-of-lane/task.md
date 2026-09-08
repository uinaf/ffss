# Deliver the Approved Docs Fix

## Problem/Feature Description

Use slopcourier if appropriate to deliver the completed work below. This is an
offline decision exercise: write the delivery route and next actions to
`delivery-status.md`; do not execute commands or invent delivery results.

## Input Files

=============== FILE: context.md ===============
User authorization: Fix the broken reference links and push directly to main
after the docs check. No PR is needed for this change.

Repository policy: routine docs-only maintenance may go directly to the
default branch. Feature work requires a PR. The default branch is main.

Current observations: the only changes fix the approved reference links. They
are committed locally on main as e26d401. The docs check passed for that commit.
The worktree is clean, origin/main is the direct parent of e26d401, and origin
is the intended repository. No PR exists. No other gate or review is required.
=============== END FILE ===============
