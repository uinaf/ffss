---
name: slopcourier
description: "Open or update a change request for completed, verified work. Use for change-request delivery."
---

# Slopcourier

Deliver finished, verified work as one change request. Delivery only: no
implementation, review, merge, or second workflow runtime.

## Preconditions

- Authorized by the user's request or agreed execution plan.
  Preparation-only requests produce a draft and stop.
- The change is complete and its required gates passed on this revision. Don't
  repeat passing proof without new changes or concerns. Report missing or
  failing gates instead of delivering.
- Proof fits the change: bug fixes have a regression check that fails before
  the fix or on revert; features exercise the changed contract; refactors run
  the same checks before and after; performance claims have a measured
  baseline; docs are checked against sources, links, or rendering. Never
  manufacture source-shape tests to make a refactor fail on revert.
- Check the worktree and `git log <default>..HEAD` for unrelated changes.
  Deliver only the intended scope and explain every hunk.
- Authorized direct delivery to the default branch is outside this skill's
  change-request lane; follow repository policy and verify the pushed commit.

## Deliver

1. `git remote get-url origin`: github.com uses `gh`, GitLab uses `glab`, other
   hosts stop. Verify auth for that host (`GH_HOST=github.com gh auth status`
   or `glab auth status --hostname <host>`). Report missing tooling or auth;
   never install or switch identities.
2. One task branch, never the default. Conventional commits, push with
   upstream tracking, no force-push without approval.
3. Update the branch's existing change request instead of filing a duplicate.
   Open ready for review unless a draft was requested.
4. Follow recent merged titles and the repository template, including
   `<owner>/.github` defaults. Without one, use the
   [house style](../slopscriber/references/style.md): problem, solution, real
   risks, proof only when CI cannot show it, headings only when needed. No
   implementation inventory. The body describes the change as it stands: no
   review history, finding counts, fix hashes, reviewer names, or iteration
   narrative. Review results go to the user and to thread replies.
5. Non-trivial changes get one review aid from
   [visual-evidence.md](references/visual-evidence.md).
6. Return the URL and delivered commit. Babysitting or merge, if requested,
   continues with slopnanny. Delivery alone authorizes no merge, auto-merge,
   branch deletion, or rework.
