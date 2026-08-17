# doc-report

Rewrote `AGENTS.md` from `inputs/AGENTS.md`. 158 lines → 85 lines, reordered around tasks instead of the file tree.

## What the old file got wrong

- It opened with a directory tree — the cheapest thing an agent can obtain itself, and the first ~35 lines of always-loaded context.
- The two incidents it failed to prevent were both covered only in passing: the migration hazard was a parenthetical inside the tree diagram, and the deprecated setup script sat in the same diagram beside the current one.
- Half the file was a copy of `docs/api.md` and `docs/deployment.md`, so an agent spent context on detail it did not need and could not trust.
- Its links were bare filenames with no trigger, so following one was a guess.

## Kept, and why

| Content | Reason |
| --- | --- |
| Node 24.x, PostgreSQL 15+, install/setup/dev commands | Not inferable; missing them is the 20-minute-exploration failure. |
| Four test commands, with the integration database prerequisite | Exact verification gates. |
| Knex-for-migrations / Sequelize-for-models split | Changes code placement on every schema task. |
| `npx knex migrate:*` commands | Non-obvious, and the only sanctioned way to change schema. |
| Custom `logger` and `AppError` requirements, no raw SQL, `any` comment rule | Repo conventions that differ from model defaults. |
| `STRIPE_WEBHOOK_SECRET` verification and no-log rule | Security boundary. |
| No `.env` commits, no force-push to `main`, branch prefixes, Conventional Commits, one approval | Cheap, and a violation is expensive. |
| Layered flow route → controller → service → data | Decides where new code lands. |
| Two real failure signatures | Known recovery an agent cannot derive. |

## Removed

- **Directory tree (35 lines).** Replaced by a "Where code goes" section that states the *rule* for each directory instead of listing filenames. File search covers the rest and never goes stale.
- **Inline API reference (~50 lines).** Full endpoint list, JSON request bodies, and status codes now live behind the `docs/api.md` pointer. Duplicating a reference guarantees the copy drifts.
- **Deployment step list.** The four CI steps are one sentence; manual deploys and rollbacks route to `docs/deployment.md`.
- **Auto-generated file/line/date table.** Line counts and modification dates last generated 2024-01-15 — stale on the next commit, and no decision depends on them. If `doc-gen` still runs, point it at a separate file rather than the agent entrypoint.
- **`Module not found` after `git pull` → run `npm install`.** A model default; it changes no behavior.
- **Q2 2023 team-merger history behind the dual-ORM setup** and the "we know this is inconsistent" aside. An agent reading "known tech debt" may try to unify the ORMs mid-task. The rewrite states the split as the current contract and what it obliges.
- **"Docker (optional)".** Optional with no command attached is not actionable.

## Changed in kind

- **`scripts/bootstrap.sh`** stayed, as one clause on the `setup.sh` line. Normally a deprecated path gets deleted, but this file is still present in the repo, so an agent can plausibly run it — the boundary earns its words. Delete the file and delete the clause.
- **Migrations became their own section** with the reason stated: the files already ran in staging and production, so editing one desynchronizes environments. A rule with its consequence attached survives pressure; "never edit manually" in a code comment did not.
- **Added a completeness checklist.** Five surfaces a charge/refund/webhook change usually touches. The old file described the structure but never said what one change must cover, which is how a route gets edited without its migration or test.
- **Pointers now lead with the trigger** — "read before adding or changing an endpoint", "read for a manual deploy, a rollback, or when a CI or ECS step fails" — so the decision to spend a tool call is made from the pointer.
- **Orientation names the stakes** (real money, the three things that must not regress) instead of restating that it is a REST API.

## Unresolved

- The old auth line contradicts itself: "Bearer token in the `Authorization` header. Token format: `JWT <token>`." I did not pick a winner. The auth header format is now sourced from `docs/api.md`; reconcile it there.
- No source tree was available in this working directory, so every path, script, and npm script in the rewrite carries over from the input file unverified. Before committing, confirm each one resolves: `scripts/setup.sh`, `scripts/seed.sh`, `src/utils/validation.ts`, the three `docs/*.md` targets, and the `test:integration`, `test:coverage`, and `seed` entries in `package.json`.
- If the repo also has a `CLAUDE.md`, make it a symlink to `AGENTS.md` rather than a second authored file.
