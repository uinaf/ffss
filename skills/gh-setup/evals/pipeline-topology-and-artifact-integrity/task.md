# Reliable GitHub Actions Deploy Pipeline for a React SPA

## Problem/Feature Description

A product team deploys its Vite React SPA by running `npm run build` locally and uploading `dist/` through the provider dashboard. The team has grown to seven engineers, and two untested local builds broke production last week. GitHub Actions must enforce that the tested artifact is the deployed artifact.

The pipeline should build the app once, run end-to-end tests against that output, and then promote it through the `production` GitHub Environment with the provider's OpenID Connect (OIDC) deploy identity. A misconfigured Vite output path produced no files twice this month, so the pipeline must reject an empty build. The framework writes its output to the hidden `.output/public/` directory, not `dist/`. After deployment, on-call engineers need links to the live site's monitoring dashboard, alert policy, synthetic check, deploy marker, and rollback runbook. That handoff currently depends on tribal knowledge.

## Output Specification

Produce a working GitHub Actions workflow at `.github/workflows/main.yml` that triggers on push to `main` and implements the full build -> test -> deploy flow described above. Use a repo-owned provider-thin deploy script or local action that accepts artifact path and environment; do not write a provider cookbook.

The deploy job must declare the `production` GitHub Environment, use `id-token: write`, and keep provider identifiers in environment vars rather than hardcoded workflow values.

Include a brief `deploy-summary.md` explaining each job, the artifacts passed between jobs, the deployment identity boundary, and the rationale.

## Input Files

The following files represent the current repository state. Extract them before beginning.

=============== FILE: package.json ===============
{
  "name": "shop-web",
  "private": true,
  "type": "module",
  "packageManager": "npm@11.6.2",
  "engines": { "node": "24" },
  "scripts": {
    "build": "vite build",
    "test": "vitest run",
    "test:e2e": "playwright test",
    "preview": "vite preview --outDir .output/public --port 4173"
  }
}

=============== FILE: vite.config.ts ===============
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: { outDir: ".output/public", emptyOutDir: true },
});

=============== FILE: playwright.config.ts ===============
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "e2e",
  use: { baseURL: "http://localhost:4173" },
  webServer: { command: "npm run preview", port: 4173, reuseExistingServer: false },
});

=============== FILE: scripts/deploy.sh ===============
#!/usr/bin/env bash
# Usage: scripts/deploy.sh <artifact-dir> <environment>
# Reads PROVIDER_PROJECT_ID and PROVIDER_DEPLOY_AUDIENCE from the environment,
# exchanges the GitHub OIDC token for a provider token, uploads <artifact-dir>,
# and prints the deployed URL as its last line.
set -euo pipefail
exec provider-cli deploy --project "$PROVIDER_PROJECT_ID" \
  --audience "$PROVIDER_DEPLOY_AUDIENCE" --environment "$2" --dir "$1"

=============== FILE: docs/oncall.md ===============
# On-call links

- Live site: https://shop.example.com
- Dashboard: https://monitoring.example.com/d/shop-web
- Alert policy: https://monitoring.example.com/alerts/shop-web-5xx
- Synthetic check: https://monitoring.example.com/synthetics/shop-web-checkout
- Deploy markers: POST https://monitoring.example.com/api/markers (token in `MONITORING_MARKER_TOKEN`)
- Rollback runbook: https://runbooks.example.com/shop-web/rollback
