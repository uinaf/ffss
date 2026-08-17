# Contributing to grok-cli

We welcome contributions. For significant features, open an issue first to discuss the design before investing implementation effort.

## Setup

Prerequisites: Node.js 24.x LTS, npm 11+

1. Clone the repo:
   ```bash
   git clone https://github.com/example/grok-cli.git
   cd grok-cli
   ```

2. Install dependencies:
   ```bash
   npm install
   ```

3. Run in development mode (auto-recompile):
   ```bash
   npm run dev
   ```

4. Link for local testing:
   ```bash
   npm link
   grok --version
   ```

For a one-off build from source instead of a development loop, run `npm run build` in place of `npm run dev`.

## Run tests

Unit tests (fast, no I/O):

```bash
npm test
```

Integration tests (reads real log files from `fixtures/`):

```bash
npm run test:integration
```

Full suite with coverage report:

```bash
npm run test:coverage
```

Tests must pass before any PR is merged. Coverage must not drop below 80%.

## Coding standards

All code must pass:

- `npm run lint` — ESLint with Airbnb config
- `npm run type-check` — TypeScript strict mode
- `npm run format:check` — Prettier

Rules enforced by CI:

- No `console.log` (use the logger in `src/logger.ts`)
- No `any` types without a `// eslint-disable` comment with justification
- Test files must be co-located with source files (`src/**/*.test.ts`)
- New parsers must implement `src/parsers/base.ts` interface
- All public API surfaces need JSDoc comments

## Architecture

grok-cli is structured as a pipeline:

```
Input → Parser → Filter → Formatter → Output
```

**Parser layer** (`src/parsers/`): one parser per format (JSON, logfmt, CEF). Each parser implements the `Parser` interface defined in `src/parsers/base.ts`. Parsers are selected automatically based on file extension or the `--format` flag.

**Filter layer** (`src/filters/`): composable filter predicates. Filters are combined with AND logic by default. The `--or` flag switches to OR logic.

**Formatter layer** (`src/formatters/`): renders the filtered output. `PrettyFormatter` uses chalk for colors; `JsonFormatter` and `CsvFormatter` are plain serializers.

**CLI layer** (`src/cli/`): Commander.js handles argument parsing and routes to the appropriate pipeline.

All errors go through `src/errors.ts`. The `GrokError` class carries an exit code so the process exits with the correct code for shell scripting.

## Pull requests

1. Fork the repo and create a branch (`feature/`, `fix/`, or `chore/` prefix)
2. Make your changes and add tests
3. Run the full test suite: `npm run test:coverage`
4. Run linting: `npm run lint && npm run type-check`
5. Open a PR with a clear description of the change

All PRs need at least one maintainer approval. We aim to review within 3 business days.

## Releases

Maintainers cut releases by tagging `v*` — this triggers the publish workflow.

## Security issues

Do not open a PR or public issue for a vulnerability. See [Security](SECURITY.md).
