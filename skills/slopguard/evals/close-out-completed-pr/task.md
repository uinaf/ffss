# Get this PR independently reviewed before merge

My Go PR (branch `fix/sigint-children`, targets `main`) is done and `go test ./...`
is green. Claude Code wrote it. I want an independent review before I merge
it. I haven't picked a reviewer. Codex CLI and Claude Code are both installed
and logged in on this machine.

Acceptance criteria: JSON output stays byte-stable, and provider child processes
terminate on SIGINT.

Don't run the review yet; I'll kick it off. Write `closeout.md` with the exact
command you'd run (including the prompt you'd feed it), why you chose the
reviewer, and what you'd do if the review errors out.
