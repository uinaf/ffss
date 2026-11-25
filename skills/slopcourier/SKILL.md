---
name: slopcourier
description: "Open, update, and babysit a PR or MR until it is ready, then watch the runs a merge, direct push, or dispatched workflow starts. Use before any `gh pr create`, `gh pr edit`, `glab mr create`, or PR/MR title or description edit, and whenever work ends in a pull or merge request, including a request to ship it."
---

# Slopcourier

Three lanes. Start where the request starts, stop where it stops:

| Request | Lanes |
|---|---|
| open, update, or ship a PR | Open (if needed), then Babysit until `ready` |
| review or CI feedback on an open PR | Babysit |
| a merged PR, direct push, or dispatched workflow | After merge |

Every PR or MR create or edit goes through Open, steps 4 to 7, even when
another skill or the harness asked for the PR.

`git remote get-url origin` picks the forge: github.com uses `gh`, GitLab uses
`glab`. Name any other host to the user. Check auth for that host
(`GH_HOST=github.com gh auth status`, `glab auth status --hostname <host>`).

## Open

1. **Gates.** The repo's required checks pass on this revision. No
   committed media the project doesn't own, such as other people's avatars:
   a pushed commit stays fetchable by SHA.
2. **Branch.** One task branch. If it already has a PR, update that PR. Open
   ready for review unless asked for a draft.
3. **Title.** The user-visible outcome, in the shape of recent merged titles.
4. **Body.** Write it to `body.md`. Repo template first, then the owner's
   `<owner>/.github` default template, else: outcome sentence, one visual aid,
   up to 5 one-line bullets for what the aid doesn't show. Describe the change
   as it stands. A closing keyword (`fix:`, `fixes`, `closes`, `resolves`)
   goes only before an issue this change resolves; write `Related:` before
   other links. With semantic-release, the `BREAKING CHANGE:` footer holds
   the whole list of what callers must change, because release notes show
   only the footer.
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
   0, or say why a flag is wrong. No Go: say bodycheck didn't run.
7. **Submit the checked file.** `gh pr create --body-file body.md` or
   `gh pr edit --body-file body.md`; on GitLab,
   `glab mr create --description "$(cat body.md)"` or
   `glab mr update --description "$(cat body.md)"`.

Report the PR's URL and head commit, then Babysit.

## Babysit

### Watch

Wait with pr-watch:

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
thread, and bots often post top-level comments. A requested human reviewer
is pending until they submit. Feedback stays open until its own fix or
reply.

### Bots

- A bot is pending while it visibly works on the current head (eyes
  reaction, running check, "reviewing" comment, review request), or for a few
  minutes after a push when it reviewed an earlier head of this PR.
- pr-watch waits `-bot-wait` (5 minutes by default; lower it to meet a user
  deadline) for a pending bot, then moves on. Name the skipped bot in the report.
- A usage-limit or quota notice means the bot is unavailable. Say so and move
  on.

### Triage

- **A human's design question or ambiguous point:** pause that PR and hand
  it to the user with the evidence.
- **Concrete fixes** (from humans or bots): validate each against the code.
  A finding is wrong when a proof in hand refutes it or the user called it
  negligible in this session, and stays wrong when re-raised.
  Fix each real one's whole defect class across the diff in one pass. Run
  the gates, push, reply on each fixed thread with the commit link alone,
  and re-request review from each human who requested changes or approved
  an older head. Reply to a wrong bot finding with a one-line reason that cites
  the proof. Resolve every fixed thread and every rejected bot thread. A
  human's finding you judge wrong goes to the user with the evidence.
  Out-of-scope findings become tracker items.
- **Ping-pong:** after the third fix round on one PR, or when each round
  brings new findings of the same kind, hold the PR and propose a rewrite
  with a different approach in a fresh session, on another model or harness
  if this one keeps missing. The user decides.
- **Failed check:** read the log. Fix what this change broke. If the base has
  since fixed it, bring in the base. A flake or infra failure gets one rerun;
  the same failure twice is real.
- **Behind or conflicted:** bring in the base, rerun the gates, and push.
  Before rebasing an old PR, check that its problem is still live (intent,
  callers), then rebase, rework, or recommend closure. After a rebase, read
  `git range-diff` and push with
  `--force-with-lease=<branch>:<head you observed>`.
- **`slopguard`:** on the final head, when the user or the repository guide
  asks for it.

Post thread replies and re-requests on the forge. Report status to the user.
Update the title and body when the rework changes what the PR does.

### Ready

`ready` means the latest head has green required checks, no pending human or
bot review, and no open threads or blocking reviews. Report it with the head
commit.

## After merge

- **Merged PR:** run pr-watch on it once. It finds the merge commit and waits
  for the runs it starts: `runs-passed`, `no-runs`, `runs-failed`, or
  `timeout`.
- **Direct push or dispatched workflow:** run pr-watch with
  `-commit <full sha>`.
- **On failure:** report the run link, the likely cause from the failed job's
  log, and the revert command for the user to run: `gh pr revert <number>`
  (GitLab: a revert branch and `glab mr create`), or `git revert <sha>` where
  the default branch takes direct pushes (`-m 1` for a merge commit).
- **Cleanup:** on `runs-passed` or `no-runs`, pr-watch prints a `cleanup:`
  line per leftover in this checkout: local branch, its worktrees, remote
  branch, stale `body.md`. Remove them and stop your background watches.
- **Report** one line: `cleanup: removed …; kept … because …`.
