---
name: slopcourier
description: "Open, babysit, and merge a change request, then watch what the merge runs. Use when opening or updating a PR or MR, when asked to get one merged or babysit it, or to watch the runs a merge, direct push, or dispatched workflow starts."
---

# Slopcourier

Three lanes. Start where the request starts, stop where it stops:

| Request | Lanes |
|---|---|
| "open the PR" | Open, then stop |
| "get it merged", "babysit" | Open (if needed), Babysit, After merge |
| review or CI feedback on an open PR | Babysit, After merge |
| direct push or dispatched workflow | After merge |

`git remote get-url origin` picks the forge: github.com uses `gh`, GitLab uses
`glab`. Any other host: stop and say so. Check auth for that host
(`GH_HOST=github.com gh auth status`, `glab auth status --hostname <host>`).

## Open

1. **Gates.** The repo's required checks pass on this revision. Proof fits the
   change: a bug fix has a check that fails without the fix; a refactor runs
   the same checks before and after; docs are checked against their sources.
2. **Branch.** One task branch. If it already has a PR, update that PR. Open
   ready for review unless asked for a draft.
3. **Title.** The user-visible outcome, in the shape of recent merged titles.
4. **Body.** Repo template first, then the owner's `<owner>/.github` default
   template, else: outcome sentence, one visual aid, a few one-line bullets
   for what the aid doesn't show. Describe the change as it stands.
   Leave out: file lists, test counts, command logs, review history, finding
   counts, fix hashes, reviewer names, and iteration narrative. CI shows
   proof; add proof only where CI can't.
5. **Visual aid.** Screenshot or short recording for UI, Mermaid for a flow,
   table for numbers, before/after code for an API. How to capture and
   upload: [visual-evidence.md](references/visual-evidence.md).
6. **Bodycheck.** From the target repo:
   `go run <this skill's dir>/scripts/bodycheck/main.go body.md`, adding
   `-before <current body file>` for an edit and `-footer <line>` when an
   attribution line is required. Fix what it flags or say why it's wrong.

Done when the PR is open: report its URL and head commit.

## Babysit

### Watch

Wait with prwatch, not a polling loop:

```bash
go run <this skill's dir>/scripts/prwatch/main.go [-R owner/repo] [-since <since>] <number or URL>
```

- **Flags go before the number or URL.** Go stops parsing flags at the first
  argument.
- It exits on the first thing to act on. The first output line is the state:
  `ready`, `checks-failed`, `activity`, `pushed`, `blocked`, `closed`,
  `error`, or `timeout` (30 minutes).
- Act, then rerun with `-since <its since: line>` so nothing between runs is
  missed.
- One background run per PR. For several PRs, start one each.
- **No Go:** poll `gh pr view` / `gh pr checks` (or `glab mr view`) for the
  same evidence, and say prwatch didn't run.

Read everything it surfaces: a `CHANGES_REQUESTED` review can have no inline
thread, and bots often post top-level comments. A requested human reviewer who
hasn't submitted is still pending. An unrelated push doesn't answer older
feedback.

### Bots

- Bots are advisory.
- A bot is pending only while it visibly works on the current head (eyes
  reaction, running check, "reviewing" comment, review request), or for a few
  minutes after a push when it reviewed an earlier head of this PR. A bot that
  never touched this PR is not pending.
- prwatch waits up to 5 minutes for a pending bot (`-bot-wait`; lower it to
  meet a user deadline), then moves on. Name the skipped bot in the report.
- A usage-limit or quota notice means the bot is unavailable. Say so and move
  on.
- Bots are summoned only when the user asks.

### Triage

- **A human's design question or ambiguous point:** stop rework on that PR,
  post nothing, and hand it back to the user with the evidence.
- **Concrete fixes** (from humans or bots): validate each against the code.
  Fix the real ones in one pass, run the gates, push, reply on each thread
  with the commit, and re-request review from anyone who requested changes or
  approved an older head. Out-of-scope findings become tracker items.
- **Failed check:** read the log. Fix what this change broke. If the base has
  since fixed it, bring in the base. A flake or infra failure gets one rerun;
  the same failure twice is real.
- **slopguard:** only when the user asks or the repo has no review bot or
  required review; once, on the final head.

On the forge, post only thread replies and re-requests; status goes to the
user. Update the title and body only when the rework changes what the PR
does.

### Merge

Merge when the latest head has green required checks, no pending human or
bot review, and no open threads or blocking reviews. Use the repo's merge
method and report the merged commit. Hold only when asked.

## After merge

- **Merged PR:** run prwatch on it once. It finds the merge commit and waits
  for the runs it starts: `runs-passed`, `no-runs`, `runs-failed`, or
  `timeout`.
- **Direct push or dispatched workflow:** run prwatch with
  `-commit <full sha>` instead of a number.
- **On failure:** report the run link, the likely cause from the failed job's
  log, and the revert command for the user to run: `gh pr revert <number>` (GitLab: a revert
  branch and `glab mr create`), or `git revert <sha>` where the default branch
  takes direct pushes (`-m 1` for a merge commit).
- **Cleanup:** delete the branch only after the forge shows the PR merged. A
  deleted head branch closes an open PR.
