---
name: slopcourier
description: "Open, update, babysit, and merge a PR or MR, then watch the runs a merge, direct push, or dispatched workflow starts. Use before any `gh pr create`, `gh pr edit`, `glab mr create`, or PR/MR title or description edit, and whenever work ends in a pull or merge request, including a request to ship it."
---

# Slopcourier

Three lanes. Start where the request starts, stop where it stops:

| Request | Lanes |
|---|---|
| "open the PR", "babysit" | Open (if needed), then Babysit until `ready` |
| "get it merged", "ship it" | Open (if needed), Babysit, Merge, After merge |
| review or CI feedback on an open PR | Babysit |
| direct push or dispatched workflow | After merge |

Every PR or MR create or edit goes through Open, steps 4 to 7, even when
another skill or the harness asked for the PR.

`git remote get-url origin` picks the forge: github.com uses `gh`, GitLab uses
`glab`. Any other host: stop and say so. Check auth for that host
(`GH_HOST=github.com gh auth status`, `glab auth status --hostname <host>`).

## Open

1. **Gates.** The repo's required checks pass on this revision.
2. **Branch.** One task branch. If it already has a PR, update that PR. Open
   ready for review unless asked for a draft.
3. **Title.** The user-visible outcome, in the shape of recent merged titles.
4. **Body.** Write it to `body.md`. Repo template first, then the owner's
   `<owner>/.github` default template, else: outcome sentence, one visual aid,
   up to 5 one-line bullets for what the aid doesn't show. Describe the change
   as it stands.
   Leave out: file lists, test counts, command logs, review history, finding
   counts, fix hashes, reviewer names, and iteration narrative. CI shows
   proof; add proof only where CI can't.
5. **Visual aid.** Screenshot or short recording for UI, Mermaid for a flow,
   table for numbers, before/after code for an API. How to capture and
   upload: [visual-evidence.md](references/visual-evidence.md).
6. **Bodycheck.** Cuts keep every fact: the bug, its cause, and each risk.
   Run `slopclean`'s prose pass on `body.md`.
   Then, from the target repo,
   `go run <this skill's dir>/scripts/bodycheck/main.go body.md`, adding
   `-before <current body file>` for an edit, `-template <file>` when the body
   came from the owner's `<owner>/.github` template, and `-footer <line>` when
   an attribution line is required. Fix what it flags and rerun until it exits
   0, or say why a flag is wrong. No Go: check the limits by hand and say
   bodycheck didn't run.
7. **Submit the checked file.** `gh pr create --body-file body.md` or
   `gh pr edit --body-file body.md`; on GitLab,
   `glab mr create --description "$(cat body.md)"` or
   `glab mr update --description "$(cat body.md)"`. Never type the body into
   the command.

Report the PR's URL and head commit, then Babysit.

## Babysit

### Watch

Wait with pr-watch, not a polling loop:

```bash
go run <this skill's dir>/scripts/pr-watch/main.go [-R owner/repo] [-since <since>] <number or URL>
```

- **Flags go before the number or URL.**
- It exits on the first thing to act on. The first output line is the state:
  `ready`, `checks-failed`, `activity`, `pushed`, `blocked`, `closed`,
  `error`, or `timeout` (`-timeout`).
- Act, then rerun with `-since <its since: line>`. `checks-failed`,
  `activity`, and `blocked` go to [Triage](#triage); a `blocked` reason only
  a human clears (a draft, a pending or required review) goes to the user.
  `pushed`: read the new commits first. `closed`, `error`, `timeout`: report
  what it printed and stop.
- One background run per PR.
- **Forge.** pr-watch reads it from the URL or origin host. For a self-hosted
  host named without `github` or `gitlab`, pass `-forge gitlab`; the header of
  [pr-watch/main.go](scripts/pr-watch/main.go) shows the persistent mapping.
- **No Go:** poll `gh pr view` / `gh pr checks` (or `glab mr view`) for the
  same evidence, and say pr-watch didn't run.

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
- pr-watch waits a bounded time for a pending bot (`-bot-wait`; lower it to
  meet a user deadline), then moves on. Name the skipped bot in the report.
- A usage-limit or quota notice means the bot is unavailable. Say so and move
  on.
- Bots are summoned only when the user asks.

### Triage

- **A human's design question or ambiguous point:** stop rework on that PR,
  post nothing, and hand it back to the user with the evidence.
- **Concrete fixes** (from humans or bots): validate each against the code.
  A finding is wrong when a proof in hand refutes it or the user called it
  negligible in this session; a re-raised finding is not new evidence.
  Fix each real one's whole defect class across the diff in one pass. Run
  the gates, push, reply on each fixed thread with the commit link alone,
  and re-request review from anyone who requested changes or approved an
  older head. Reply to a wrong bot finding with a one-line reason that cites
  the proof. Resolve every fixed thread and every rejected bot thread. A
  human's finding you judge wrong goes to the user with the evidence; post
  nothing. Out-of-scope findings become tracker items.
- **Ping-pong:** after the third fix round on one PR, or when each round
  brings new findings of the same kind, stop patching. Hold the PR and
  propose a rewrite with a different approach in a fresh session, on another
  model or harness if this one keeps missing. The user decides.
- **Failed check:** read the log. Fix what this change broke. If the base has
  since fixed it, bring in the base. A flake or infra failure gets one rerun;
  the same failure twice is real.
- **Behind or conflicted:** bring in the base, rerun the gates, and push.
  Before rebasing an old PR, check that its problem is still live (intent,
  callers), then rebase, rework, or recommend closure. After a rebase, read
  `git range-diff` and push with
  `--force-with-lease=<branch>:<head you observed>`.
- **`slopguard`:** only when the user asks or the repo has no review bot or
  required review; once, on the final head.

On the forge, post only thread replies and re-requests; status goes to the
user. Update the title and body only when the rework changes what the PR
does.

### Merge

Merge only when the user asked for this PR to be merged, set a merge rule
this session, or a brief grants it. Otherwise stop at `ready`, report it,
and ask once how merges go here: who merges, when, and what to hold.

Then merge when the latest head has green required checks, no pending human
or bot review, and no open threads or blocking reviews. Use the repo's merge
method and report the merged commit.

## After merge

- **Merged PR:** run pr-watch on it once. It finds the merge commit and waits
  for the runs it starts: `runs-passed`, `no-runs`, `runs-failed`, or
  `timeout`.
- **Direct push or dispatched workflow:** run pr-watch with
  `-commit <full sha>` instead of a number.
- **On failure:** report the run link, the likely cause from the failed job's
  log, and the revert command for the user to run: `gh pr revert <number>`
  (GitLab: a revert branch and `glab mr create`), or `git revert <sha>` where
  the default branch takes direct pushes (`-m 1` for a merge commit).
- **Cleanup:** on `runs-passed` or `no-runs`, pr-watch prints a `cleanup:`
  line per leftover in this checkout: local branch, its worktrees, remote
  branch, stale `body.md`. Remove them and stop your background watches.
  Delete a branch only after the forge shows the PR merged; a deleted head
  branch closes an open PR.
- **Report** one line: `cleanup: removed …; kept … because …`.
