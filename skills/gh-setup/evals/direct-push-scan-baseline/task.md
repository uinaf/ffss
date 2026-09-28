# Cheapest Secret and Workflow Scanning for a Direct-Push Fleet

## Problem/Feature Description

An owner runs 30 repositories on a GitHub Team organization: 14 public and
16 private. Coding agents push straight to `main` as the owner, who is a
repository administrator. Renovate opens dependency pull requests and merges
patch and minor updates through GitHub auto-merge.

Every repository has a `verify` workflow that runs on pull requests and on
pushes to `main`. Each also calls a shared reusable scan workflow as its own
job, triggered by `pull_request`, `push` to `main`, and a weekly `schedule`.
The job runs Gitleaks, TruffleHog over full history, actionlint, and zizmor.
Private repositories run on a paid third-party runner. Last week the scan
jobs billed 2,400 of 9,200 runner minutes, and most runs lasted under 20
seconds. The organization rulesets require `verify` and, on two repositories,
extra checks named after the old scan jobs.

The owner wants the fewest CI minutes and least money while still catching
leaked secrets, unsafe workflow changes, and vulnerable dependencies. Hard
constraints: no local git hooks, no notification channel beyond GitHub's own,
scans never block a push or merge, and at most one required check.

## Output Specification

Produce `ci-security-baseline.md` containing:

- the detection choice for public and for private repositories, with the paid
  option and its price basis;
- where and when each scanner runs, and why;
- the required-check and ruleset changes, including how renamed or removed
  checks are handled for open pull requests;
- the expected change in weekly runner minutes, with the arithmetic.

Do not mutate live GitHub settings.
