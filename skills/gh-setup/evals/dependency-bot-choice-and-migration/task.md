# Dependency Bot Choice and Migration

## Problem/Feature Description

A private infrastructure repository pins tool versions in `mise.toml`, pins
OpenTofu providers in `versions.tf`, runs container images by tag and digest
in Compose files, and stores binary versions with sha256 checksums in Ansible
role defaults. It currently runs Dependabot for GitHub Actions and Compose.
The organization has installed the free hosted Renovate app with
"Require config file" enabled and wants a gradual migration across its
repositories without duplicate pull requests.

Decide whether this repository should move to Renovate, and describe the
migration if so. Also assess a proposed fleet rollout of GitHub-native
automerge: a ruleset called “Renovate required checks” targets each default
branch with no App bypass. A content App commits validated entries directly;
a release App writes versions in its source repository and formulas in a
separate tap. Another release App only creates tags. Signing, deletion, and
force-push protections already exist separately. The private fleet has a
checked-in policy and audit command, but one repository's rule read failed.

## Output Specification

Produce `dependency-updates-plan.md` containing:

- the bot choice with the concrete gaps that justify it;
- what changes in one migration commit and what stays untouched;
- how the organization preset and repository config split responsibilities;
- the OpenTofu registry handling;
- which pins remain manual and why;
- the readback that proves the first run behaved;
- whether the proposed rules preserve those writers, the exact scope of any
  correction, and a rollout/readback plan suitable for the private fleet.

Do not mutate a live organization.
