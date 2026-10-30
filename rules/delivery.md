## Delivery

- Use Conventional Commits unless the repository says otherwise.
- The commit message and change request body record what changed and why.
  Keep ADRs, specs, decision logs, and hand-written changelogs out of the
  repository; release tooling generates changelogs from commits.
- Push directly when the repository guide or user rules allow it, even as an
  administrator bypassing required checks; otherwise open a change request.
- Review bots are advisory: give one reviewing this change request a bounded
  wait on the head you merge, and leave the rest alone.
