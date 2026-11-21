## Delivery

- Use Conventional Commits unless the repository says otherwise.
- The commit message and change request body record what changed and why.
  Keep ADRs, specs, decision logs, hand-written changelogs, and any doc that
  narrates the code out of the repository; plans and decisions live in the
  issue, and release tooling generates changelogs from commits.
- Push directly when the repository guide or user rules allow it, even as an
  administrator bypassing required checks; otherwise open a change request.
- Review bots are advisory. Wait a bounded time for a bot already reviewing
  the head you'll merge; don't wait for others.
