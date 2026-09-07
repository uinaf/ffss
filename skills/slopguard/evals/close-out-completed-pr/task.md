# Close out a completed pull request

A Go pull request is implementation-complete and its full test suite has passed.
No provider is named by the user or repository configuration. Codex CLI, Claude
Code, and Cursor Agent are installed. No task requirement needs web access.

Write `closeout.md` with the exact review command, the decision behind it,
failure handling, and the evidence the final report must contain. The pull request targets `main` and its acceptance criteria
are: preserve stable JSON output and terminate provider children on SIGINT.
