# Delegated Proof Only Observes

## Problem/Feature Description

Continue the approved slopmachine plan. This is an offline decision exercise:
write `actions.md` with the next actions and the evidence still needed. Do not
execute commands or invent results.

Approved scope: stop the settings modal from flashing on page reload, add a
regression test, run the required checks and slopguard, then open a PR and
continue through merge.

Current observations from this session:
- Worktree on `fix/modal-flash`, HEAD 7c1d2e4, clean. The dependency lockfile
  is unchanged since main.
- A delegated agent implemented the fix and reports it verified with two
  items: a screenshot after reload showing no modal, and a new test
  `modal.test.tsx > does not flash on reload` that passes at 7c1d2e4.
- You reverted the fix locally to check the test and it still passed; the
  test renders the modal and asserts on the final settled state. You restored
  the fix; the worktree is at 7c1d2e4 again.
- Lint and typecheck passed at 7c1d2e4 in this session.
- This is the user's interactive workstation. `pnpm dev` on port 3000 is the
  user's own session, started before this task. The delegated agent started a
  second `pnpm dev` on port 3001 for its screenshot and left it running. Only
  one browser-driven test run fits on the machine at a time.

The user says: Looks verified, wrap it up.
