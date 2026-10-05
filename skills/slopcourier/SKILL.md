---
name: slopcourier
description: "Open or update a change request for completed, verified work, including its title, body, and review aids. Use whenever creating, drafting, or editing a pull or merge request, or deciding what follows once one is open. Not for resolving review feedback or CI on an open one."
---

# Slopcourier

Deliver finished, verified work as one change request. This skill owns
change-request delivery; separately authorized review, babysitting, or merge
continues through its owning workflow, so this lane does not end the task.

## Preconditions

- Authorized by the user's request or agreed plan. Preparation-only requests
  produce a local draft without publication.
- Required gates passed on this revision; report missing or failing gates
  instead of delivering. Don't repeat passing proof without new changes.
- Proof fits the change: a bug fix has a regression check that fails before the
  fix or on revert; a refactor runs the same checks before and after; docs are
  checked against sources, links, or rendering. Never manufacture source-shape
  tests to make a refactor fail on revert.
- Check the worktree and `git log <default>..HEAD`; deliver only the intended
  scope and keep unrelated work.
- Authorized direct delivery to the default branch is outside this lane;
  follow repository policy and verify the pushed commit.

## Deliver

1. `git remote get-url origin`: github.com uses `gh`, GitLab uses `glab`, other
   hosts stop. Verify auth for that host (`GH_HOST=github.com gh auth status`
   or `glab auth status --hostname <host>`). Report missing tooling or auth;
   never install or switch identities.
2. One task branch, never the default. Conventional commits, push with
   upstream tracking, no force-push without approval.
3. Update the branch's existing change request instead of filing a duplicate.
   Open ready for review unless a draft was requested.
4. Title the user-visible outcome, not the mechanism, in the form of recent
   merged titles. Follow the repository template; without one, the owner's
   `<owner>/.github` default template; without either, the
   [house style](../slopscriber/references/style.md): problem, solution, real
   risks, headings only when needed.
5. Keep the body as short as the change allows, describing the change as it
   stands: an outcome sentence, one visual aid, and at most a few one-line
   bullets for what the aid doesn't show. Proof only when CI cannot show it.
   Out: implementation inventory, file lists, test counts, command logs,
   fixture ids, review history, finding counts, fix hashes, reviewer names,
   and iteration narrative. Review results go to the user and to thread
   replies.
6. Every change request carries a visual aid in place of the prose it makes
   redundant: a screenshot or short recording for UI, a Mermaid diagram for a
   flow, a table for numbers, or a short before/after code sample for an API
   or contract. Mechanics:
   [visual-evidence.md](references/visual-evidence.md).
7. Check the body before posting or editing it: from the target repository,
   run `go run <this skill's directory>/scripts/bodycheck/main.go body.md`,
   adding `-before <current body>` for an edit and `-footer <line>` when an
   attribution line is required. Fix what it flags, or tell the user why a
   flag is wrong. Without Go, apply its limits by hand and say it didn't run.
8. Return the URL and delivered commit. Babysitting or merge, if requested,
   continues with [slopnanny](../slopnanny/SKILL.md), carrying the user's
   existing authority across the handoff. Delivery alone grants no merge,
   auto-merge, branch deletion, or rework authority. Review bots are
   advisory; slopnanny decides when to wait for one.
