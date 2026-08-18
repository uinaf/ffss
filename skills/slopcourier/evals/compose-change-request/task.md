# Prepare the Change Request for a Finished Fix

## Problem/Feature Description

The fix is done and verified on branch `fix/ws-compression`: the repository's
gate (`npm test`, 214 passed) ran fresh and green, and staging measurements
are attached below. The remote is `git@github.com:example/dashboard.git` and
`gh auth status` is green for github.com.

I want to review the change request before it goes up. Write the exact title
and complete body you will submit to `delivery.md`, then stop; do not push or
open anything yet.

Context: dashboard clients on slow links were dropping live updates because
every websocket frame shipped uncompressed. The change negotiates
permessage-deflate on the websocket server. Staging measurement against the
busiest feed: median frame size down from 41 KB to 11 KB.

The last three merged change requests in this repository were titled:

- `fix(api): stop dropping auth renewals under clock skew`
- `perf(worker): cut cold-start p95 from 900ms to 320ms`
- `feat(alerts): page on-call before the queue backs up`

## Input Files

=============== FILE: .github/PULL_REQUEST_TEMPLATE.md ===============
## Problem

## Solution

## Risk

## Proof
<!-- Only if CI cannot show it -->
=============== END FILE ===============
