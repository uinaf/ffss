# Merge Queue

Use only when the repository runs a merge queue. GitHub's native queue groups
pull requests but does not combine their builds and removes a failing entry
rather than bisecting
([GitHub](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue));
batched builds with bisection exist in Mergify and Graphite-class queues
([Graphite](https://graphite.com/blog/merge-queue-batching)).

- Batching relaxes per-commit correctness, so enable it only where a broken
  intermediate commit is acceptable.
- Set group minimum, maximum, and wait timeout from arrival rate
  ([Mergify](https://docs.mergify.com/merge-queue/batches/)); GitHub's native
  queue exposes those three settings.
- One published model tiers checks
  ([Tian Pan](https://tianpan.co/blog/2026-07-02-the-merge-queue-is-the-new-bottleneck)):
  lint, type check, and affected unit tests before the queue; deterministic
  integration in the queue; end-to-end and performance after merge with a fast
  revert path, only when the owner accepts post-merge detection for them.
  Required pre-merge coverage stays by default.
- Cascade rate rises with flake rate (same source); quarantine flaky tests
  instead of retrying entries, and give automated authors a retry budget set
  from measured queue capacity and the owner's failure policy.
- Queue-required workflows never use workflow-level `paths` filters, which
  leave the required check unreported on the queue branch
  ([GitHub](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/configuring-pull-request-merges/managing-a-merge-queue));
  skip inside the job.
