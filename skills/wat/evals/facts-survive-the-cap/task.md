# wat

You just sent me this update about the dependency sweep:

> Quick update on the version-bump sweep across our repos! It's been quite
> a journey today. I managed to get quite a few of them over the line:
> the bump PR for api landed as #341, the one for webapp went in as #128,
> worker's was #77, ingest's was #204, and docs-site's was #55 — all five
> of those are merged and their CIs came back green, which is great news.
> The cli repo's PR #93 is open with checks still running as of a few
> minutes ago, so fingers crossed there. Unfortunately it wasn't all
> smooth sailing: the bump broke the build in three places. In gateway,
> PR #166 is failing because the new version dropped the deprecated
> `createPool` export the code still uses. In metrics, PR #310 fails two
> snapshot tests in `dashboards.test.ts` that pinned the old output
> format. And in scheduler, PR #89 hits a peer-dependency conflict with
> the pinned redis client, which will probably need its own bump first.
> My plan is to tackle those three tomorrow morning unless you'd rather
> I keep going tonight — happy to do either, just let me know!

wat
