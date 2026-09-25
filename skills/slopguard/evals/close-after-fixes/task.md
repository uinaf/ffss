# Close out after several review rounds

Write `review-actions.md` with the next actions, evidence status, and any user
decision needed for each independent case. Record the actions instead of
taking them: do not invoke a reviewer, push, merge, or post anything.

The user asked for independent review before merge and authorized fixing and
delivering both change requests. No policy limits review rounds or fix
commits. The session is long and has already compacted once; the tracking
note records each finding's disposition and the revision each proof ran on.

Case A: A token-refresh change has had three slopguard rounds. The latest
round, on the current head, returned two findings. One was a real defect: it is
fixed, its regression test fails with the fix reverted, and the required
checks are green on the new head. The fix narrowed which refresh errors
invalidate a session. The other finding repeats one rejected in round two
with evidence and brings no new evidence. After the fix, a comment typo was
also corrected.

Case B: A completed docs-only change corrects a broken link. Source and link
checks passed. No review was requested, no policy requires one, and the
slopguard CLI and its configured provider are available.
