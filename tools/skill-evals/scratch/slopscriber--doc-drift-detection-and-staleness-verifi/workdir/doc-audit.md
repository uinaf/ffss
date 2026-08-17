# Documentation Drift Audit — invoicing-service

Audited `AGENTS.md`, `README.md`, and `docs/ops/runbook.md` against
`repo-manifest.json`, which is the authoritative record of post-refactor
repository state.

Method: extracted every file path, script invocation, and API endpoint named in
the three docs, then matched each against the manifest's `exists` and `removed`
lists. Re-ran the sweep after editing to confirm no removed identifier survives.

**Result:** 11 stale references across 3 files. 10 repointed, 1 feature section
deleted. 6 references checked and confirmed still correct. 8 references fall
outside the manifest's coverage and were left untouched — listed under
[Unverifiable](#unverifiable--left-unchanged).

## Stale references fixed

| # | File | Line | Was | Now | Basis |
|---|------|------|-----|-----|-------|
| 1 | `AGENTS.md` | 7 | `./scripts/bootstrap.sh` | `./scripts/setup.sh` | `bootstrap.sh` in `scripts.removed`; `setup.sh` in `scripts.exists`. **Inferred** — see note below. |
| 2 | `AGENTS.md` | 24 | `config/app.yaml` | `config/settings.yaml` | `config.removed` → `config.exists` |
| 3 | `AGENTS.md` | 30 | `docs/api-v2.md` | `docs/api-v3.md` | `docs.removed` → `docs.exists` (link text and target both) |
| 4 | `README.md` | 9 | `config/app.yaml.example` | `config/settings.yaml.example` | `config.removed` → `config.exists` |
| 5 | `README.md` | 9 | `config/app.yaml` (copy target) | `config/settings.yaml` | `config.removed` → `config.exists` |
| 6 | `README.md` | 15 | `./scripts/bootstrap.sh` | `./scripts/setup.sh` | same as #1 |
| 7 | `README.md` | 22 | `docs/api-v2.md` | `docs/api-v3.md` | `docs.removed` → `docs.exists` |
| 8 | `runbook.md` | 14 | `/api/v2/health` | `/api/v3/health` | `api_endpoints.removed` → `api_endpoints.exists` |
| 9 | `runbook.md` | 32 | `/api/v2/webhooks/stripe` | `/api/v3/webhooks/stripe` | `api_endpoints.removed` → `api_endpoints.exists` |
| 10 | `runbook.md` | 17–25 | entire "PDF export broken" section | **deleted** | see below |

Every replacement target above appears verbatim in a manifest `exists` list.

### Note on `bootstrap.sh` → `setup.sh` (#1, #6)

This is the one mapping the manifest does **not** state explicitly. It records
that `bootstrap.sh` was removed and that `setup.sh` exists; it does not declare
a rename. `setup.sh` is the only script in `scripts.exists` plausibly serving as
the boot entrypoint (`seed.sh` and `migrate.sh` are data and schema
operations), so I repointed both docs at it.

Leaving `bootstrap.sh` in place was not an option — it is the dead command
that triggered the pages. Confidence that `setup.sh` exists: verified.
Confidence that it is the correct replacement: high but inferred. **Worth a
one-line confirmation from the backend team.**

### Note on the deleted PDF export section (#10)

Removed rather than repointed. The manifest lists PDF export under
`features.removed` ("feature was cut from v3 scope"), and its helper script
`scripts/test-pdf.sh` under `scripts.removed`. There is no successor to point
at. The deletion took four dependent references with it:

- `./scripts/test-pdf.sh` — confirmed in `scripts.removed`
- `which wkhtmltopdf` — tooling for a cut feature
- `logs/pdf-errors.log` — log file for a cut feature
- `DISABLE_PDF_EXPORT=true` — toggle for a cut feature

No tombstone note was left behind. A runbook is read under time pressure; a
section describing a feature that cannot fire is a false lead. The removal is
recorded here instead.

## Verified correct — no change needed

Checked against the manifest and confirmed present. These were left as-is:

| Reference | Cited in | Manifest |
|-----------|----------|----------|
| `src/utils/logger.ts` | `AGENTS.md:22` | `src.exists` |
| `src/lib/calculator.ts` | `AGENTS.md:23` | `src.exists` |
| `src/api/webhooks.ts` | `runbook.md:29` | `src.exists` |
| `docs/architecture.md` | `AGENTS.md:29`, `README.md:21` | `docs.exists` |
| `docs/ops/deploy.md` | `AGENTS.md:31`, `README.md:23` | `docs.exists` |
| `docs/ops/runbook.md` | `AGENTS.md:32`, `README.md:24` | `docs.exists` |

## Unverifiable — left unchanged

The manifest covers `scripts/`, `config/`, `src/`, `docs/`, API endpoints, and
features. It does not describe `package.json`, dependencies, root-level files,
environment variables, or runtime topology. The following are therefore
**unconfirmed in either direction** and were not touched:

| Reference | Cited in | Why unverifiable |
|-----------|----------|------------------|
| `npm install`, `npm run dev` | `README.md:8,16` | no `package.json` in manifest |
| `npm test`, `npm run test:integration` | `AGENTS.md:12,17` | no `package.json` in manifest |
| `npx knex migrate:latest` | `AGENTS.md:25`, `runbook.md:33` | dependencies not in manifest — **but see flag below** |
| `CONTRIBUTING.md` | `README.md:28` | manifest tracks `docs/`, not root files |
| `docker logs invoicing-service` | `runbook.md:10` | runtime, not repo state |
| `http://localhost:3000` | `runbook.md:14` | port not in manifest |
| `STRIPE_WEBHOOK_SECRET` | `runbook.md:31` | env vars not in manifest |
| `DATABASE_URL` | `runbook.md:33` | env vars not in manifest |

### Flag: `npx knex migrate:latest` vs. `scripts/migrate.sh`

The refactor added `scripts/migrate.sh` (in `scripts.exists`) while both docs
still instruct readers to call knex directly. Either `migrate.sh` is now the
canonical migration entrypoint and both call sites are stale, or it is
unrelated and the knex command is still correct.

The manifest cannot settle this — it does not say what `migrate.sh` does or
whether knex is still a dependency. I left the knex command in place rather
than guess. **This is the highest-value open question in the audit:** a wrong
migration command in an on-call runbook fails at the worst moment. Confirm
with the backend team and update `AGENTS.md:25` and `runbook.md:33` together.

## Gaps — present in the repo, absent from the docs

Not drift (nothing points at a dead target), but the refactor shipped
capability that no doc mentions. I did not write entries for these, because
the manifest gives names only — no behavior, arguments, or preconditions to
document accurately.

| Exists in repo | Documented? |
|----------------|-------------|
| `scripts/seed.sh` | no |
| `scripts/migrate.sh` | no — see flag above |
| `src/api/invoices.ts` | no |
| `src/api/payments.ts` | no |
| `/api/v3/invoices` | no |
| `/api/v3/payments` | no |

`docs/api-v3.md` exists and may already cover the two endpoints; its contents
were not provided, so this is unconfirmed.

## Verification

- Sweep for removed identifiers (`bootstrap.sh`, `test-pdf.sh`, `config/app.yaml`,
  `api-v2`, `/api/v2/`, `wkhtmltopdf`, `pdf-errors`, `DISABLE_PDF_EXPORT`, `PDF`)
  across all three docs after editing: **0 matches**.
- Sweep for replacement targets (`setup.sh`, `settings.yaml`, `api-v3`,
  `/api/v3/`): **8 matches**, one per intended edit site.
- Each replacement target matched verbatim against a manifest `exists` entry.

Verification is against `repo-manifest.json` only. No working tree was
available, so no `test -e` or command execution was possible; every claim here
traces to the manifest.
