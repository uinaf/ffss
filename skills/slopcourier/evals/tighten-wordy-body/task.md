# Get the export fix PR ready

Branch `fix/csv-export-cursor` in `tallowmere/ledger` (origin
`git@github.com:tallowmere/ledger.git`) is done and CI is green on it. The
repository has no pull request template, and the `tallowmere/.github`
repository has none either. I drafted the description in `body.md`, but it
reads like a press release.

Don't push, open, or edit anything on GitHub. Leave the final description in
`body.md`, and write `delivery.md` with the exact commands you'd run from this
checkout to open the PR, plus the title.

Last three merged PR titles:

- `fix(api): stop dropping auth renewals under clock skew`
- `perf(worker): cut cold-start p95 from 900ms to 320ms`
- `feat(alerts): page on-call before the queue backs up`

======= FILE: body.md =======
## Summary

This PR introduces a comprehensive and robust fix for a critical issue in our
CSV export functionality. It's worth noting that, prior to this change, exports
containing more than 10,000 rows would silently stop at exactly 10,000 rows,
which meant that users were unknowingly receiving incomplete data — a
significant problem for data integrity, user trust, and downstream reporting.

## Root Cause

After a thorough investigation, I discovered that the root cause was that the
exporter was ignoring the `next_cursor` value that the API returns after the
first page. As a result, it never requested any subsequent pages.

## The Fix

The exporter now seamlessly follows `next_cursor` until the API returns an
empty cursor, ensuring that every single row is fetched and written.

## Testing

- Added 6 new unit tests in `export/cursor.test.ts`
- All 312 tests pass locally
- Files changed: `export/csv.ts`, `export/cursor.ts`, `export/cursor.test.ts`
- In staging, a 48,213-row export now returns all 48,213 rows (previously 10,000)
- The export now takes 41 seconds (previously 9 seconds), which is expected given the additional pages
- Peak memory stays flat at roughly 120 MB, since rows are streamed rather than buffered

## Risks

One potential consideration is that very large exports, above roughly 70,000
rows, may hit the 60-second gateway timeout. This is tracked separately in
#212.

## Conclusion

In summary, this change ensures complete, reliable, and accurate CSV exports
for all of our users. Closes #198.
======= END FILE =======
