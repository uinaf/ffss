# Plan the Export Lane's Landing

The export lane has the four issues below left. Each is small, and each builds
on the one before it. Plan how the lane gets all four merged as fast as it can
while each change still gets a proper review.

You can't reach the repository from here. Write the plan (branches, PRs,
reviews, merge order) to `plan.md`.

======= FILE: issues.md =======
# Epic #172: PDF export (lane export)

- #191 Extract a `PageLayout` type from the export code (S, about 60 lines)
- #192 Compute page breaks from `PageLayout` (S, about 120 lines; needs #191)
- #193 Render the last page's footer (S, about 80 lines; needs #192)
- #194 Respect the page size setting (S, about 90 lines; needs #192)
======= END FILE =======

======= FILE: CONTRIBUTING.md =======
# Contributing

- One PR per issue against `main`, squash merge. Required checks: `lint`,
  `unit`, `e2e-ios`, `e2e-web` (about 25 minutes per push).
- `quillbot` reviews every push; one maintainer approval is required.
- Every change also gets an independent review from a second model before it
  merges.
- Reviewers ask for PRs small enough to read in one sitting.
======= END FILE =======
