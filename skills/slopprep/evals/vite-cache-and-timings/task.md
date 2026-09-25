Our CI verify job takes about nine minutes and agents wait on it constantly.
Someone suggested just turning on `run: { cache: true }` in `vite.config.ts`
so everything gets cached. Make verification faster, edit the files here as
you see fit, and write `perf-plan.md` covering what you changed and how we'll
know it actually got faster. Don't push, publish, deploy, or run migrations.

=============== FILE: vite.config.ts ===============
import { defineConfig } from "vite-plus";

export default defineConfig({
  run: {
    tasks: {
      typecheck: { command: "tsc -b", inputs: ["packages/*/src/**", "tsconfig*.json"] },
      test: { command: "vitest run", inputs: ["packages/*/src/**", "packages/*/test/**"] },
      lint: { command: "oxlint packages", inputs: ["packages/**", ".oxlintrc.json"] },
      verify: { dependsOn: ["typecheck", "test", "lint"] },
    },
  },
});
=============== END FILE ===============

=============== FILE: package.json ===============
{
  "name": "harbor",
  "private": true,
  "packageManager": "pnpm@10.18.0",
  "scripts": {
    "verify": "vp run verify",
    "build": "vp run -r build",
    "db:migrate": "node scripts/migrate.mjs",
    "deploy": "wrangler deploy --env production",
    "release": "changeset publish"
  },
  "devDependencies": {
    "vite-plus": "0.4.2",
    "typescript": "5.9.3",
    "vitest": "3.2.4",
    "oxlint": "1.19.0"
  }
}
=============== END FILE ===============

=============== FILE: .github/workflows/verify.yml ===============
name: verify
on: [pull_request]
jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      code: ${{ steps.f.outputs.code }}
    steps:
      - uses: actions/checkout@v4
      - id: f
        uses: dorny/paths-filter@v3
        with:
          filters: |
            code:
              - 'packages/*/src/**'
              - 'packages/*/test/**'
  verify:
    needs: changes
    if: needs.changes.outputs.code == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: pnpm/action-setup@v4
        with:
          version: 10.18.0
      - uses: actions/setup-node@v4
        with:
          node-version: 22
      - run: pnpm install --frozen-lockfile
      - run: pnpm verify
=============== END FILE ===============
