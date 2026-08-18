# Deliver the Pagination Fix, and Only That

## Problem/Feature Description

Deliver the pagination fix from branch `fix/cursor-pagination` as a pull
request against `main`. Remote: `git@github.com:example/api.git`, `gh` is
authenticated for github.com, and the repository gate passed fresh
(`go test ./...`, ok).

Current state of the branch, gathered just now:

```
$ git log --oneline main..HEAD
9c1e2aa fix(api): stop skipping the last page on cursor pagination
41b7d03 test(api): reproduce the dropped-last-page case
f00dbeef wip: experiment with redis-backed cursor cache

$ git status --short
 M internal/cache/redis.go   (half-finished cache experiment, not part of the fix)
```

I have no PR for this branch yet. Because this sandbox has no network, do not
actually push; instead write the exact sequence of git/gh commands you would
run, plus the PR title and body, to `delivery-plan.md`.
