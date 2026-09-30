Part of my monthly agent retro: what do reviewers keep catching in the pull
requests my agents opened this month, and what should change so they stop?
I exported the review data and copied my agent setup into this folder
(`home/` is my home directory, `repo/` is the main project). Don't change
anything yet; I want findings to discuss.

=============== FILE: reviews.json ===============
[
  {"pr": 311, "repo": "acme/cli", "author": "agent", "merged": true, "commits_after_open": 3, "review_rounds": 2, "ci_runs": 5,
   "comments": [
     {"by": "review-bot", "path": "scripts/release.sh", "body": "Unquoted $TAG expands with word splitting; quote it.", "outcome": "fixed"},
     {"by": "maintainer:dana", "body": "Needs a changeset.", "outcome": "fixed"}
   ],
   "commits_by_others": []},
  {"pr": 318, "repo": "acme/cli", "author": "agent", "merged": true, "commits_after_open": 1, "review_rounds": 1, "ci_runs": 2,
   "comments": [],
   "commits_by_others": [{"by": "dana", "message": "chore: add changeset"}]},
  {"pr": 322, "repo": "acme/cli", "author": "agent", "merged": true, "commits_after_open": 4, "review_rounds": 3, "ci_runs": 7,
   "comments": [
     {"by": "review-bot", "path": "src/fetch.ts", "body": "The timeout branch has no test; the retry path is untested.", "outcome": "fixed"},
     {"by": "maintainer:dana", "body": "Please cover the error path, not just the happy path.", "outcome": "fixed"},
     {"by": "review-bot", "path": "scripts/ci-cache.sh", "body": "rm -rf $DIR/ with an unset DIR deletes /. Quote and guard.", "outcome": "fixed"}
   ],
   "commits_by_others": []},
  {"pr": 327, "repo": "acme/cli", "author": "agent", "merged": true, "commits_after_open": 2, "review_rounds": 2, "ci_runs": 3,
   "comments": [
     {"by": "maintainer:dana", "body": "Changeset missing again.", "outcome": "fixed"}
   ],
   "commits_by_others": []},
  {"pr": 330, "repo": "acme/api", "author": "agent", "merged": true, "commits_after_open": 3, "review_rounds": 2, "ci_runs": 4,
   "comments": [
     {"by": "review-bot", "path": "src/auth.ts", "body": "No test for the expired-token branch.", "outcome": "fixed"},
     {"by": "review-bot", "path": "src/auth.ts", "body": "Consider renaming `tok` to `token`.", "outcome": "rejected"}
   ],
   "commits_by_others": []},
  {"pr": 334, "repo": "acme/api", "author": "agent", "merged": true, "commits_after_open": 0, "review_rounds": 1, "ci_runs": 1,
   "comments": [], "commits_by_others": []},
  {"pr": 339, "repo": "acme/cli", "author": "agent", "merged": true, "commits_after_open": 1, "review_rounds": 1, "ci_runs": 2,
   "comments": [],
   "commits_by_others": [{"by": "dana", "message": "chore: add changeset"}]},
  {"pr": 341, "repo": "acme/api", "author": "agent", "merged": false, "commits_after_open": 5, "review_rounds": 3, "ci_runs": 9,
   "comments": [
     {"by": "review-bot", "path": "deploy/rollout.sh", "body": "for f in $(ls *.yaml) breaks on spaces; use a glob.", "outcome": "open"},
     {"by": "maintainer:omar", "body": "Error path for a failed rollout isn't tested.", "outcome": "open"}
   ],
   "commits_by_others": []}
]
=============== END FILE ===============

=============== FILE: home/.claude/CLAUDE.md ===============
# Rules

- Lead with the outcome.
- Test error paths, not just the happy path.
- Open a pull request for every change.
=============== END FILE ===============

=============== FILE: repo/CONTRIBUTING.md ===============
# Contributing

- Every user-facing change needs a changeset (`pnpm changeset`).
- CI runs `lint`, `test`, and `build`. `lint` is ESLint only.
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
