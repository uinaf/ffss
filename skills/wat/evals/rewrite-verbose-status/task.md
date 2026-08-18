# wat

You just sent me this update:

> So, quick status on where things stand! I've been working through the
> review feedback on the dashboard PR and I'm happy to report that things
> are progressing nicely. First, I took a look at the comment from the
> reviewer about the caching behavior — it turned out to be a really
> interesting rabbit hole, because the memoization actually interacts with
> the polling interval in a subtle way. Long story short, I pushed a fix
> for that (it's commit 4f2a1c9 on PR #212). After that, I turned my
> attention to the failing e2e suite, and after quite a bit of digging it
> turned out that 3 of the 17 tests were failing because of a timezone
> assumption in the fixtures, which I've now corrected. The suite is
> re-running as we speak and should hopefully be green soon. I also wanted
> to flag that while I was in there, I noticed the bundle size has crept
> up by about 14% since last month, which might be worth looking into at
> some point, though it's not urgent. Next up, once CI is green, I'll go
> ahead and re-request review from the team, and hopefully we can get this
> merged soon! Let me know if you have any questions or if there's
> anything else you'd like me to dig into!

wat
