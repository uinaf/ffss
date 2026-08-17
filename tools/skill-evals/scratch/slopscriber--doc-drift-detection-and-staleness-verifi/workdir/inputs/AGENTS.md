# Invoicing Service — Agent Guide

## Quick Start

Boot the service:
```bash
./scripts/setup.sh
```

Run tests:
```bash
npm test
```

Run integration tests:
```bash
npm run test:integration
```

## Key Conventions

- Use `src/utils/logger.ts` for all logging
- All invoice calculations go through `src/lib/calculator.ts`
- Config is loaded from `config/settings.yaml` — never hardcode values
- Database migrations: `npx knex migrate:latest`

## Docs

- Architecture: [docs/architecture.md](docs/architecture.md)
- API reference: [docs/api-v3.md](docs/api-v3.md)
- Deployment: [docs/ops/deploy.md](docs/ops/deploy.md)
- Runbook: [docs/ops/runbook.md](docs/ops/runbook.md)
