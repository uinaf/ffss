# Plan Parallel Invitation Work in Jira

## Problem/Feature Description

The repository tracks work in Jira project `TEAM`. The user approved a parent epic and three independently reviewable end-to-end slices:

1. Create and send an invitation from the admin UI through the API.
2. Accept an invitation and join the team; this requires invitation creation.
3. Revoke a pending invitation from the admin UI; this requires invitation creation but not acceptance.

The user says: "Publish the epic and tickets with the real dependency graph so agents can work in parallel."

Produce `planning-result.md` describing the Jira artifacts, parent relationships, blocking links, and current dependency frontier. Do not implement the feature.

## Input Files

=============== FILE: AGENTS.md ===============
# Agent guide

Planned engineering work lives in Jira project TEAM. Multi-session features use an Epic with child Tasks and `blocks` links for hard dependencies.
=============== END FILE ===============
