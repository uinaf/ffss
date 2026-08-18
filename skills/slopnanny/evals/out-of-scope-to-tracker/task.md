# A Reviewer Wants More on PR #71

## Problem/Feature Description

You are babysitting pull request #71 on `example/ingest`, whose whole goal is
fixing a typo'd environment variable name (`INJEST_BUCKET` →
`INGEST_BUCKET`). A human reviewer left the comment below, newer than the
latest push. Checks are green. This sandbox has no network, so write
everything you would do — commands, replies, and any tracker items — to
`actions.md`.

## Input Files

=============== FILE: review-comment.txt ===============
@maria-oncall commented on pull request #71 (top-level comment):

"Fix looks right. While you're here though — this client has no retry
logic at all, one 503 from the bucket and the whole batch dies. Can you
add exponential backoff with jitter in this PR? Should only be ~40 lines."
=============== END FILE ===============
