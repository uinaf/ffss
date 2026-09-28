# Cost-Safe Organization Security Baseline

## Problem/Feature Description

A GitHub Team organization contains both public and private repositories. Its
current organization security configuration enables `advanced_security` and
CodeQL by default for public repositories only. One public repository may soon
become private, and the organization owner wants to avoid surprise
per-active-committer Advanced Security charges while retaining the useful free
dependency protections available across the fleet. Renovate owns version and
vulnerability pull requests fleet-wide.

Design the recommended organization-level default for all current and future
repositories. Distinguish paid Secret Protection and Code Security features
from the dependency graph and Dependabot features. Include the visibility-change
preflight and readback needed to catch billing changes. Paid security features
may be enabled for an exceptional repository only through an explicit,
cost-approved opt-in.

## Output Specification

Produce `github-security-baseline.md` containing:

- the proposed organization security configuration and enforcement scope;
- the paid and free feature choices, including CodeQL default setup;
- a safe rollout and live readback checklist;
- the public-to-private visibility transition preflight;
- the exception path for a repository that genuinely needs a paid feature.

Do not mutate a live GitHub organization or claim that disabling the paid
bundles disables every overlapping security capability on public repositories.
