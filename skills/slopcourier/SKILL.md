---
name: slopcourier
description: "Deliver a completed, verified change as a change request on the repository's forge: branch discipline, conventional commits, push, open the change request from the repo's template, attach the clearest visual evidence, and return the URL. Use when asked to deliver, ship, file, or open a pull/merge/change request for finished work, or when a slopmachine run reaches DELIVER. Do not use to implement or review changes, merge or enable auto-merge, or record slopmachine evidence by itself."
---

# Slopcourier

Deliver finished, verified work as exactly one change request. This skill
owns delivery mechanics only; it never implements, reviews, merges, or
invents a second workflow runtime.

## Preconditions

1. The work is complete and the repository's own gates passed (builder-owned
   checks, run fresh). Report a missing or failing gate instead of delivering.
2. Delivery is authorized: an explicit user request, or an active slopmachine
   run whose status allows `deliver`.
3. The delivery is a change request. A slopmachine run with
   `delivery_mode: direct-trunk` is out of this skill's lane — do not open
   a change request for it; the delivery is the trunk commit itself,
   recorded with `slopmachine deliver` and its `commit_sha`.
4. The delivery contains only the intended change — both the worktree and,
   when reusing a task branch, its commits ahead of the default branch
   (`git log <default>..HEAD`). Preserve unrelated work; never sweep it
   into the change request.

## Forge dispatch

The vocabulary is "change request"; the forge decides the tool:

1. Read the remote: `git remote get-url origin`.
2. Dispatch on its host: `github.com` → `gh`; a GitLab host → `glab`;
   anything else → stop and report the unsupported forge. Never guess at
   a forge API.
3. Verify authentication for that host (`GH_HOST=github.com gh auth
   status` or `glab auth status --hostname <host>`); a login on some
   other configured host proves nothing. Report a missing dependency; do
   not install tooling or switch identities.

## Deliver

1. Never commit to the default branch. Create or reuse one task branch named
   for the change.
2. Commit with conventional-commit messages. Never force-push without
   explicit approval.
3. Push with upstream tracking.
4. Check whether a change request for this branch already exists; if it
   does, update it instead of filing a duplicate.
5. Open the change request with the dispatched CLI, ready for review unless
   a draft was requested. Title it the way this repository titles merged
   work — read recent merged change requests and git history first — and
   prefer the outcome over the mechanism ("cut frame size 70% with
   gzipping", not "negotiate permessage-deflate"). Use the repository's
   template verbatim; if none exists, cover Summary, Changed, Risks,
   Verification, and Complexity in plain language. Open the description
   with the problem as the requester stated it, then the solution — never
   an implementation inventory. Never fabricate verification results —
   report exactly what ran.
6. Give a non-trivial change its single clearest review aid — a labeled
   screenshot or recording, a focused diagram, or sanitized contract
   input/output. Load [visual-evidence.md](references/visual-evidence.md)
   for the attachment ladder. Never commit proof assets to the repository.
7. Return the change-request URL as the result.

## Compose with slopmachine

When an active slopmachine run asked for this delivery, hand the URL
straight back through stdin evidence and let the binary judge it (a
forge-bound repo verifies the change request and head before accepting).
`delivery_mode` must match the run — read it from the status document
(`delivery_mode` field) instead of assuming one:

Validate first, then apply, per the slopship protocol — the same payload
with `--dry-run --json`, proceed only when the projection matches:

```bash
slopmachine deliver --evidence - --dry-run --json --run <run-id> <<'JSON'
{"delivery_mode":"<delivery_mode from status>","pr_url":"<change request URL>","commit_sha":"<delivered head>"}
JSON
# projection ok -> repeat without --dry-run
```

A rejection means the forge disagreed with the claim; fix the delivery, not
the evidence.

## Boundaries

- One change request per delivery; unrelated work is never batched.
- No merging, no auto-merge enablement, no branch deletion, no review of
  your own delivery.
- Address review feedback only when asked; when a finding is fixed, reply
  with the commit hash.
