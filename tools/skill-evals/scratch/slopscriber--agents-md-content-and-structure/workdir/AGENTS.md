# Payments Service

Express + TypeScript REST API at `/api/v1`. It creates and captures charges, processes refunds, and ingests Stripe webhooks. It moves real money for Meridian customers: a wrong amount, a double charge, or a dropped webhook is a production incident, not a failed test.

Never regress: charge and refund amounts, idempotency of webhook handling, and Stripe signature verification.

## Setup

Requires Node.js 24.x LTS and PostgreSQL 15+.

```bash
npm install
cp .env.example .env   # fill in DATABASE_URL, Stripe, and SendGrid values
scripts/setup.sh       # canonical setup; scripts/bootstrap.sh is unmaintained
npm run dev
```

## Verify

Run before every handoff. Integration tests need a reachable `DATABASE_URL`.

| Command | Scope |
| --- | --- |
| `npm test` | unit tests |
| `npm run test:integration` | integration tests, live database |
| `npm run test:coverage` | full suite with coverage |
| `npm run seed` | load fixture data into the dev database |

## Where code goes

Request flow is one direction: route → controller → service → data layer. A route handler that talks to Stripe, SendGrid, or the database directly is misplaced.

- `src/api/` — Express route handlers, one file per resource.
- `src/services/` — every third-party call. Stripe and SendGrid are wrapped here; do not import their SDKs elsewhere.
- `src/db/models/` — Sequelize models. All application reads and writes go through them; raw SQL is allowed only inside migrations.
- `src/db/migrations/` — Knex migrations.
- `src/utils/` — `logger.ts`, `errors.ts`, `validation.ts`.

The split is deliberate and load-bearing: Knex owns schema, Sequelize owns runtime access. Changing a table means writing a Knex migration *and* updating the Sequelize model.

## Migrations

Migration files are append-only. Never edit or overwrite a file in `src/db/migrations/` — it has already run in staging and production, so an edit silently desynchronizes environments. Correct a mistake with a new migration.

```bash
npx knex migrate:make <name>   # new migration; the only way to change schema
npx knex migrate:latest        # apply
npx knex migrate:rollback      # undo the last batch, local only
```

## Conventions

- Log through `src/utils/logger.ts`. `console.log` is not permitted anywhere in `src/`.
- Throw `AppError` from `src/utils/errors.ts`. Bare `Error` bypasses the API error mapping and leaks 500s.
- Strict mode is on. An `any` needs a comment on the same line saying why.
- Validate request bodies through `src/utils/validation.ts` before touching a service.
- Branches use `feature/`, `fix/`, or `chore/` prefixes. Commits follow Conventional Commits. PRs need one approval.

## Secrets and safety

- `STRIPE_WEBHOOK_SECRET` verifies inbound signatures in `src/api/webhooks.ts` before any event is processed. Never log it, never bypass the check, never accept an unsigned event.
- Do not log full request bodies or Stripe payloads; they carry payment method and customer data.
- Never commit `.env` or any populated secret. `.env.example` holds keys only.
- Never `git push --force` to `main`.

## Completeness check for a change

A change to charge, refund, or webhook behavior usually touches all five:

1. Route handler in `src/api/`.
2. Service wrapper in `src/services/` if a third party is involved.
3. Sequelize model plus a new Knex migration if the schema moves.
4. Tests under `tests/unit/` and, for anything crossing the database, `tests/integration/`.
5. `docs/api.md` if the request or response contract changed.

## Deeper docs

- [API reference](docs/api.md) — read before adding or changing an endpoint, or when you need exact request and response shapes, status codes, or the auth header format.
- [Architecture](docs/architecture.md) — read when a change crosses layers, adds a third-party integration, or you need the webhook and retry model.
- [Deployment](docs/deployment.md) — read for a manual deploy, a rollback, or when a CI or ECS step fails. Merges to `main` deploy automatically through `.github/workflows/`.

## Recover

- `Cannot connect to database` — PostgreSQL is not running or `DATABASE_URL` is wrong. Integration tests fail this way first.
- `Stripe signature verification failed` — local `STRIPE_WEBHOOK_SECRET` does not match the secret for the endpoint in the Stripe dashboard. Fix the value; do not disable verification.
