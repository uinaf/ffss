# Claude reviewer is down, use Codex

I asked for an independent review of my uncommitted local changes. Our repo's
review config pins the reviewer:

======= FILE: .slopguard.yaml =======
engine: claude
reasoning_effort: high
======= END FILE =======

The Claude CLI on this box is broken right now (its login is gone and I can't
fix it today). Codex is installed and logged in. Just use Codex instead for this
one review.

Acceptance criteria for the change: config reload is atomic, and invalid YAML
leaves the previous config active.

Don't run the review; write `response.md` with the exact command you'd run and a
line on each setting you chose.
