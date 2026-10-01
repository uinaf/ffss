Monthly agent retro, September edition: what do reviewers keep catching in the
pull requests my agents opened this month, and what should change so they
stop? I exported the review data and copied my agent setup into this folder
(`home/` is my home directory, `repo/` is acme/cli, our main project; acme/api
is ours too, fmtkit is someone else's project I contribute to). Last month's
retro notes are in `retros/`. Don't change anything yet; I want findings to
discuss.

=============== FILE: reviews.json ===============
[
  {"pr": 311, "repo": "acme/cli", "merged": true, "opened": "2026-09-02T09:10Z", "merged_at": "2026-09-03T16:40Z",
   "commits_after_open": 3, "review_rounds": 2, "ci_runs": 5,
   "comments": [
     {"by": "review-bot", "path": "scripts/release.sh", "body": "Unquoted $TAG expands with word splitting; quote it.", "outcome": "fixed"},
     {"by": "maintainer:dana", "body": "Needs a changeset.", "outcome": "fixed"}
   ],
   "commits_by_others": [], "edits_before_merge": []},
  {"pr": 318, "repo": "acme/cli", "merged": true, "opened": "2026-09-05T08:00Z", "merged_at": "2026-09-05T11:30Z",
   "commits_after_open": 1, "review_rounds": 1, "ci_runs": 2,
   "comments": [
     {"by": "review-bot", "path": "src/config.ts", "body": "Exported function `loadConfig` has no JSDoc block.", "outcome": "fixed"}
   ],
   "commits_by_others": [{"by": "dana", "message": "chore: add changeset"}, {"by": "dana", "message": "chore: drop JSDoc block"}],
   "edits_before_merge": []},
  {"pr": 322, "repo": "acme/cli", "merged": true, "opened": "2026-09-08T10:00Z", "merged_at": "2026-09-11T15:20Z",
   "commits_after_open": 5, "review_rounds": 3, "ci_runs": 8,
   "comments": [
     {"by": "review-bot", "path": "src/fetch.ts", "body": "The timeout branch has no test; the retry path is untested.", "outcome": "fixed"},
     {"by": "maintainer:dana", "body": "Please cover the error path, not just the happy path.", "outcome": "fixed"},
     {"by": "review-bot", "path": "scripts/ci-cache.sh", "body": "rm -rf $DIR/ with an unset DIR deletes /. Quote and guard.", "outcome": "fixed"},
     {"by": "review-bot", "path": "src/fetch.ts", "body": "Exported function `fetchWithRetry` has no JSDoc block.", "outcome": "fixed"}
   ],
   "commits_by_others": [{"by": "dana", "message": "chore: drop JSDoc block"}], "edits_before_merge": []},
  {"pr": 327, "repo": "acme/cli", "merged": true, "opened": "2026-09-12T09:00Z", "merged_at": "2026-09-12T17:05Z",
   "commits_after_open": 2, "review_rounds": 2, "ci_runs": 3,
   "comments": [
     {"by": "maintainer:dana", "body": "Changeset missing again.", "outcome": "fixed"},
     {"by": "review-bot", "path": "src/log.ts", "body": "Exported function `logLine` has no JSDoc block.", "outcome": "rejected"}
   ],
   "commits_by_others": [], "edits_before_merge": []},
  {"pr": 330, "repo": "acme/api", "merged": true, "opened": "2026-09-15T08:30Z", "merged_at": "2026-09-16T10:00Z",
   "commits_after_open": 3, "review_rounds": 2, "ci_runs": 4,
   "comments": [
     {"by": "review-bot", "path": "src/auth.ts", "body": "No test for the expired-token branch.", "outcome": "fixed"},
     {"by": "review-bot", "path": "src/auth.ts", "body": "Consider renaming `tok` to `token`.", "outcome": "rejected"}
   ],
   "commits_by_others": [], "edits_before_merge": []},
  {"pr": 339, "repo": "acme/cli", "merged": true, "opened": "2026-09-18T13:00Z", "merged_at": "2026-09-18T15:10Z",
   "commits_after_open": 1, "review_rounds": 1, "ci_runs": 2,
   "comments": [],
   "commits_by_others": [{"by": "dana", "message": "chore: add changeset"}], "edits_before_merge": []},
  {"pr": 341, "repo": "acme/api", "merged": false, "opened": "2026-09-22T09:00Z", "merged_at": null,
   "commits_after_open": 5, "review_rounds": 3, "ci_runs": 9,
   "comments": [
     {"by": "review-bot", "path": "deploy/rollout.sh", "body": "for f in $(ls *.yaml) breaks on spaces; use a glob.", "outcome": "open"},
     {"by": "maintainer:omar", "body": "Error path for a failed rollout isn't tested.", "outcome": "open"}
   ],
   "commits_by_others": [], "edits_before_merge": []},
  {"pr": 88, "repo": "fmtkit/fmtkit", "merged": true, "opened": "2026-09-09T12:00Z", "merged_at": "2026-09-19T08:00Z",
   "commits_after_open": 1, "review_rounds": 1, "ci_runs": 2,
   "comments": [],
   "commits_by_others": [],
   "edits_before_merge": [{"by": "maintainer:lin", "summary": "reverted whitespace-only changes to 5 files the PR did not need to touch"}]},
  {"pr": 91, "repo": "fmtkit/fmtkit", "merged": true, "opened": "2026-09-16T10:00Z", "merged_at": "2026-09-27T09:30Z",
   "commits_after_open": 0, "review_rounds": 1, "ci_runs": 1,
   "comments": [],
   "commits_by_others": [],
   "edits_before_merge": [{"by": "maintainer:lin", "summary": "reverted import reordering in 3 unrelated files"}]},
  {"pr": 94, "repo": "fmtkit/fmtkit", "merged": false, "opened": "2026-09-24T14:00Z", "merged_at": null, "closed_at": "2026-09-25T07:00Z",
   "commits_after_open": 0, "review_rounds": 1, "ci_runs": 1,
   "comments": [
     {"by": "maintainer:lin", "body": "Closing: this reformats a dozen files unrelated to the fix. Please keep PRs to the change.", "outcome": "closed"}
   ],
   "commits_by_others": [], "edits_before_merge": []}
]
=============== END FILE ===============

=============== FILE: retros/2026-08.md ===============
# Agent retro, August 2026

## Review lessons

1. Missing changesets in acme/cli (3 PRs). Fix applied: added a changeset rule
   to home/.claude/CLAUDE.md.
2. Unquoted shell variables (2 PRs). Fix applied: added a quoting rule to
   home/.claude/CLAUDE.md.

## Baseline

- Agent PRs merged: 7
- Rework commits per merged PR: 1.6
- Review rounds per merged PR: 1.7
- PRs needing a commit from someone else: 1
=============== END FILE ===============

=============== FILE: home/.claude/CLAUDE.md ===============
# Rules

- Lead with the outcome.
- Test error paths, not just the happy path.
- Always add a changeset for user-facing changes in acme/cli.
- Always quote shell variables in scripts.
- Open a pull request for every change.
=============== END FILE ===============

=============== FILE: home/.claude/skills/testing/SKILL.md ===============
---
name: testing
description: Write and update tests when adding or changing behavior in acme repositories.
---

# Testing

- Use vitest; colocate tests as `*.test.ts` next to the source.
- Add a test for each new behavior you introduce.
- Run `pnpm test` before pushing.
=============== END FILE ===============

=============== FILE: home/.claude/skills/contributing-upstream/SKILL.md ===============
---
name: contributing-upstream
description: Prepare and open pull requests to projects we do not own.
---

# Contributing upstream

- Read the project's CONTRIBUTING file first and follow its conventions.
- Run the project's formatter before committing.
- Keep the PR description short and link the issue.
=============== END FILE ===============

=============== FILE: repo/CONTRIBUTING.md ===============
# Contributing

- Every user-facing change needs a changeset (`pnpm changeset`).
- No JSDoc blocks; names and types are the documentation.
- CI runs `lint`, `test`, and `build`. `lint` is ESLint only.
=============== END FILE ===============

=============== FILE: repo/package.json ===============
{
  "name": "acme-cli",
  "private": true,
  "scripts": {
    "build": "tsc -p .",
    "test": "vitest run",
    "lint": "eslint .",
    "lint:sh": "shellcheck scripts/*.sh",
    "changeset": "changeset"
  },
  "devDependencies": {
    "@changesets/cli": "^2.27.0",
    "eslint": "^9.0.0",
    "shellcheck": "^3.0.0",
    "typescript": "^5.6.0",
    "vitest": "^2.1.0"
  }
}
=============== END FILE ===============

=============== FILE: repo/.github/workflows/ci.yml ===============
jobs:
  lint:
    steps:
      - run: pnpm eslint .
  test:
    steps:
      - run: pnpm test
  build:
    steps:
      - run: pnpm build
=============== END FILE ===============

=============== FILE: repo/.github/review-bot.yml ===============
rules:
  - shell-quoting
  - untested-branches
  - jsdoc-required
  - naming
=============== END FILE ===============
