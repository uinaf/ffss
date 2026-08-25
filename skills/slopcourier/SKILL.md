---
name: slopcourier
description: "Deliver a completed, verified change as one change request on the repository's forge and return the URL. Use to deliver, ship, or file finished work, or at slopmachine DELIVER; never to implement, review, or merge."
---

# Slopcourier

Deliver finished, verified work as exactly one change request. You own
delivery mechanics only: don't implement, review, merge, or invent a
second workflow runtime.

## Preconditions

1. The work is complete and the repository's own gates passed (builder-owned
   checks, run fresh). If a gate is missing or failing, report it instead
   of delivering.
2. Don't deliver what you haven't proven: you ran the changed behavior and
   watched it work, a covering test fails on revert, and you can explain
   every hunk in plain sentences. The reviewer is not your first tester.
3. Confirm delivery is authorized: an explicit user request, or an active
   slopmachine run whose status allows `deliver`.
4. Deliver as a change request. A slopmachine run with
   `delivery_mode: direct-trunk` is out of your lane: don't open a change
   request for it; that delivery is the trunk commit itself, recorded with
   `slopmachine deliver` and its `commit_sha`.
5. Deliver only the intended change. Check both the worktree and, when
   reusing a task branch, its commits ahead of the default branch
   (`git log <default>..HEAD`). Preserve unrelated work; never sweep it
   into the change request.

## Forge dispatch

Call it a change request; let the forge decide the tool:

1. Read the remote: `git remote get-url origin`.
2. Dispatch on its host: `github.com` → `gh`; a GitLab host → `glab`;
   anything else → stop and report the unsupported forge. Never guess at
   a forge API.
3. Verify authentication for that host (`GH_HOST=github.com gh auth
   status` or `glab auth status --hostname <host>`); a login on some
   other configured host proves nothing. If a dependency is missing,
   report it; don't install tooling or switch identities.

## Deliver

1. Never commit to the default branch. Create or reuse one task branch named
   for the change.
2. Commit with conventional-commit messages. Never force-push without
   explicit approval.
3. Push with upstream tracking.
4. Check whether a change request for this branch already exists; if it
   does, update it instead of filing a duplicate.
5. Open the change request with the dispatched CLI, ready for review
   unless a draft was explicitly requested; a real change request lets the
   review bots run. Title it the way this repository titles merged work
   (read recent merged change requests and git history first), and prefer
   the outcome over the mechanism:
   - BAD: `perf(server): negotiate permessage-deflate on the websocket`
   - GOOD: `perf(server): cut websocket frame size by 70%+ with gzipping`

   Use the repository's template verbatim, including shared org defaults
   (on GitHub, an `<owner>/.github` repository). Fill it problem-first:
   - the problem as the requester stated it, then the solution in plain
     sentences
   - a risk only when there is a real one
   - proof only when CI cannot show it (a screenshot, before/after
     numbers); delete an empty proof section rather than restating the
     checks CI runs
   - with no template anywhere, the same flow in the
     [house style](../slopscriber/references/style.md): a one- or
     two-sentence problem lead, then labeled bullets; never a
     multi-sentence paragraph wall, no headings
   - no implementation inventories, no headings beyond the template's own
   - BAD: "## Summary Refactors the websocket layer. ## Changed server.ts,
     compression.ts, 12 tests. ## Risks None. ## Verification Tests
     pass. ## Complexity Medium." (a heading scaffold restating the diff)
   - GOOD: "Dashboard clients on slow links were dropping updates because
     every frame shipped uncompressed.
     - **Fix:** negotiate permessage-deflate on the websocket.
     - **Measured:** frame size down 70%+ on the busiest feeds, against
       the staging firehose."
6. Give a non-trivial change its single clearest review aid; load
   [visual-evidence.md](references/visual-evidence.md).
7. Return the change-request URL as the result.

## Compose with slopmachine

When an active slopmachine run asked for this delivery:

- Hand the URL back through `slopmachine deliver` stdin evidence.
- Read `delivery_mode` from the status document; never assume one.
- Follow slopmachine's validate-then-apply protocol: see
  [slopmachine](../slopmachine/SKILL.md).
- Set `commit_sha` to the delivered head.

## Boundaries

- One change request per delivery; never batch unrelated work.
- Don't merge, enable auto-merge, delete branches, or review your own
  delivery.
- Address review feedback only when asked; babysitting the open change
  request is slopnanny's lane.
