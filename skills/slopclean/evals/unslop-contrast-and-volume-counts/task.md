# Unslop the pgsnap README

Unslop the pgsnap README before the 2.0 announcement. Edit `README.md` in
place; `bench/sizes.md` is there for reference, don't touch it.

=============== FILE: README.md ===============
# pgsnap

pgsnap takes consistent Postgres snapshots while writes continue. Snapshots
are consistent, not best-effort. Since its first release, pgsnap has shipped
412 commits from 19 contributors across 38 releases, and the test suite now
runs 1,840 tests.

## How It Works

pgsnap opens one `REPEATABLE READ` transaction and streams each table with
`COPY`, instead of reading rows one at a time. Retention is a count, not a
guess: pgsnap keeps the last 24 snapshots and deletes older ones.

Snapshots compress with zstd. The median snapshot of a 40 GB staging
database went from 9.1 GB to 3.4 GB ([bench/sizes.md](bench/sizes.md)).
Speed isn't the goal here; restorability is.

## Why Not pg_dump?

pg_dump also reads inside one transaction, but it writes one file per
database. pgsnap writes one file per table, so a restore can start on the
first table before the last one finishes downloading.

## Upgrading From 1.x

`pgsnap dump` is removed in 2.0. Run `pgsnap take` instead of `pgsnap dump`;
the old command exits with code 64.
=============== END FILE ===============

=============== FILE: bench/sizes.md ===============
# Snapshot sizes, 40 GB staging database

| Codec | Median snapshot |
| ----- | --------------- |
| gzip  | 9.1 GB          |
| zstd  | 3.4 GB          |
=============== END FILE ===============
