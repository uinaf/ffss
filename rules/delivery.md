## Delivery

- Use Conventional Commits unless the repository says otherwise.
- The commit message and change request body record what changed and why.
  Keep ADRs, specs, decision logs, and hand-written changelogs out of the
  repository; release tooling generates changelogs from commits.
- Push directly when the repository guide or user rules allow it; that
  covers an administrator push past required checks. Otherwise open a change
  request.
- Review bots are advisory. Wait a bounded time for a bot already reviewing
  the head you'll merge; don't wait for others.
