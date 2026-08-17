# Agent Guide

`@acme/widget-cli` is a private, ESM-only Node package providing command-line tooling for inspecting widget bundles. Setup, development, and verification run through the checked-in npm scripts in `package.json`; they are the canonical entrypoints.

## Setup

```bash
npm ci
npm run setup
```

`npm run setup` runs the repo's checked-in setup script through `tsx`. Setup is complete when both commands exit 0.

## Develop

```bash
npm run dev
```

Runs the CLI entrypoint through `tsx` in watch mode.

## Verify

```bash
npm run verify
```

Runs TypeScript type checking (`tsc --noEmit`) and the Node test runner (`node --test`). Both must pass before handoff.

## Conventions

- Node ESM only (`"type": "module"`); use import syntax and ESM-safe paths.
- TypeScript runs through `tsx`; there is no build step to invoke.
- Add or change lifecycle commands in `package.json` scripts, then update [Setup guide](docs/setup.md) and this file in the same change.
