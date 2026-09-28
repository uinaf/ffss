# Fan-out

Run a plan's independent items as parallel headless workers on this machine.
Fan out only when items share no files or ordering and each has its own
endpoint; a single change stays in the session.

## Loop

1. **Batch:** one tracking item on the forge (an issue or equivalent) holds a
   batch's state. Resume an open batch before opening another; one batch per
   session run.
2. **Backlog:** list the candidates. Before dispatching, record every pick as
   `picked` and every pass with its status and reason.
3. **Dispatch:** write the item's brief, record `dispatched` with `host=`,
   `worker=` and a fresh `attempt=`, then run
   `scripts/worker.sh start ID DIR BRIEF ATTEMPT`. If the worker doesn't
   start, record `picked` again.
4. **Work:** workers follow their brief to its endpoint and post their report,
   status line first, on the tracking item.
5. **Watch:** `scripts/worker.sh status`. Don't poll workers; read a report
   when a worker exits, and poll the forge no faster than its rate limits
   allow.
   - `startup-failed`: the worker never ran. `worker.sh log ID` says why; fix
     it and dispatch again.
   - `exit=N` other than 0, or `dead`, with no report: read `worker.sh log
     ID`, then `worker.sh resume ID "Post your report on the tracking item
     now, status line first."` once. If that also ends without a report,
     record `gap` with what the log shows.
6. **Check proof:** confirm each report's claims against the forge before
   recording an outcome. A claim the forge doesn't show is unverified.
7. **Finish:** `worker.sh stop ID ATTEMPT` with the item's current attempt,
   remove the worker's worktree once its commits are on a remote, and record
   the outcome. Close the tracking item when every item has a final status.

Hand anything a rule stops, such as a person's review, a takeover, or a denied
action, to the user with the evidence; don't work around it.

## Ledger

Status lines on the tracking item hold all batch state; worker run files are
scratch.

- One line per status change:
  `slopmachine: #<item> <status> [key=value ...] [-- note]`. The latest line
  for an item wins; keys it omits carry over.
- Lines count in creation order, the description's before every comment, so
  record each change as a new comment and never edit an earlier one.
- `attempt=` ties lines to one dispatch. A `dispatched` line with a new
  attempt starts it; a repeated `dispatched`, or any line naming another
  attempt, is ignored. Put the item's current attempt on every line about a
  dispatched item so a late write from an older attempt can't overwrite a
  redispatch. A line without `attempt=` always applies.
- The plan defines its statuses; `picked`, `dispatched` and `gap` belong to
  the loop.
- On resume from another machine, leave items `dispatched` with another
  `host=`; that session owns them.

## Briefs

A brief is the worker's whole context: the item, tracking item, attempt,
endpoint, rules, and report format. Every brief also says:

- No coordinator is attached. If an action is denied, don't work around it:
  list the command and why under "Needs coordinator" and carry on.
- Stay in the foreground: the session ends when the turn does, killing
  background tasks. Wait in bounded poll loops inside one command, and don't
  end the turn before the endpoint unless done or blocked.
- Every status line carries `attempt=<attempt>`.

## Workers

[`scripts/worker.sh`](../scripts/worker.sh) runs each worker headless in tmux
session `slop-<id>` on this machine, at most `SLOPMACHINE_MAX_WORKERS` (2) at
once. Its header lists the commands and the contract for another harness.
Each `start` is a new attempt with its own prompt, log and exit code;
`stop ID ATTEMPT` refuses with exit 3 once the worker has moved to a newer
attempt. It needs tmux; if tmux is missing, stop and report.
