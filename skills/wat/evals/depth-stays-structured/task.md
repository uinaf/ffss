# wat, and Then the Details

You just sent me this:

> Just to keep you in the loop, I've been investigating the intermittent
> deploy failures we discussed and I believe I've gotten to the bottom of
> it! It turned out to be a fascinating interplay of a few different
> factors. Essentially, the deploy job in `release.yml` and the nightly
> cleanup job were racing over the shared artifact cache, and under
> certain timing conditions the cleanup would evict the layer the deploy
> was about to reference, leading to those confusing manifest-unknown
> errors we saw in runs 4412 and 4437. I've put together a fix in PR #250
> which introduces a lock so the two can't overlap anymore, and I've
> verified it across 20 consecutive staging deploys with zero failures,
> which I'm feeling really good about! There were also a couple of other
> small things I tidied up along the way that I can walk you through
> sometime if you're curious.

wat. and then walk me through exactly how the race breaks the deploy,
step by step — I need to explain it to the platform team.
