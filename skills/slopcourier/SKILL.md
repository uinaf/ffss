---
name: slopcourier
description: "Open or update a change request for completed, verified work. Use for change-request delivery, including slopmachine DELIVER."
---

# Slopcourier

Deliver finished, verified work as one change request. Delivery mechanics only:
no implementation, review, merge, or second workflow runtime.

## Preconditions

- Delivery is authorized by the user or an active slopmachine status allowing
  `deliver`. Preparation-only requests produce a draft artifact and stop.
- The intended change is complete and its applicable repository gates passed
  for this revision. Use owner-approved affected checks; don't repeat passing
  proof without new changes, failures, or unresolved concerns. Report missing
  or failing required gates instead of delivering.
- Proof fits the change: bug fixes have a regression check failing before the
  fix or on revert; features exercise the changed contract; refactors run the
  same behavioral checks before and after; performance claims have a measured
  baseline; docs are checked against sources, links, or rendering as applicable.
  Don't manufacture source-shape tests to make a refactor fail on revert.
- Check the worktree and `git log <default>..HEAD` for unrelated changes. Deliver
  only the intended scope and explain every hunk.
- `delivery_mode: direct-trunk` is outside this lane. Its delivered trunk commit
  is recorded through `slopmachine deliver` with `commit_sha`, not a change request.

## Dispatch and deliver

1. Read `git remote get-url origin`: github.com uses `gh`; a GitLab host uses
   `glab`; unsupported hosts stop. Verify auth for that exact host with
   `GH_HOST=github.com gh auth status` or `glab auth status --hostname <host>`.
   Report missing tooling or auth; don't install or switch identities.
2. Create or reuse one task branch, never commit to the default branch. Commit
   conventionally and push with upstream tracking. No force-push without approval.
3. Find an existing change request for the branch and update it rather than
   filing a duplicate. Open ready for review unless the user requested a draft.
4. Follow recent merged titles and the repository template, including shared
   organization defaults such as `<owner>/.github`. Lead with the problem and
   solution, include actual risks, and add proof only when CI cannot show it.
   With no template, use the concise
   [house style](../slopscriber/references/style.md): problem-first, no invented
   headings or implementation inventory. Prefer outcomes over mechanisms.
5. For non-trivial changes, use the clearest review aid from
   [visual-evidence.md](references/visual-evidence.md); skip filler.
6. Return the change-request URL. If the user also requested babysitting or
   merge, continue that authorized work with slopnanny. Delivery alone does
   not authorize merge, auto-merge, branch deletion, or review rework.

## Slopmachine handoff

Read `delivery_mode` from status. Send the URL and delivered head as `commit_sha`
through `slopmachine deliver` stdin evidence, following its
[validate-then-apply protocol](../slopmachine/SKILL.md).
