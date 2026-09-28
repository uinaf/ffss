# Reliable GitHub Actions Deploy Pipeline for a React SPA

## Problem/Feature Description

A product team deploys its Vite React SPA by running `npm run build` locally and uploading `dist/` through the provider dashboard. The team has grown to seven engineers, and two untested local builds broke production last week. GitHub Actions must enforce that the tested artifact is the deployed artifact.

The pipeline should build the app once, run end-to-end tests against that output, and then promote it through the `production` GitHub Environment with the provider's OpenID Connect (OIDC) deploy identity. A misconfigured Vite output path produced no files twice this month, so the pipeline must reject an empty build. The framework uses a non-standard output structure. After deployment, on-call engineers need links to the live site's monitoring dashboard, alert policy, synthetic check, deploy marker, and rollback runbook. That handoff currently depends on tribal knowledge.

## Output Specification

Produce a working GitHub Actions workflow at `.github/workflows/main.yml` that triggers on push to `main` and implements the full build -> test -> deploy flow described above. Use a repo-owned provider-thin deploy script or local action that accepts artifact path and environment; do not write a provider cookbook.

The deploy job must declare the `production` GitHub Environment, use `id-token: write`, and keep provider identifiers in environment vars rather than hardcoded workflow values.

Include a brief `deploy-summary.md` explaining each job, the artifacts passed between jobs, the deployment identity boundary, and the rationale.
