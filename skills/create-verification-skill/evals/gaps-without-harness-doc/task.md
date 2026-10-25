I'm about to let agents work in this repo unattended. There's no testing doc,
just the scripts and CI below. Set it up so an agent knows how to prove its
changes here. I'm offline for the rest of the day, so don't wait on me; leave
me whatever you need answered.

======= FILE: README.md =======
# tidepool-web

Booking front end for Tidepool swim schools. Next.js, Playwright.

```bash
pnpm install
pnpm dev
```
======= END FILE =======

======= FILE: package.json =======
{
  "name": "tidepool-web",
  "private": true,
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "lint": "eslint .",
    "typecheck": "tsc --noEmit",
    "test": "vitest run",
    "e2e": "playwright test --project=mocked",
    "e2e:live": "playwright test --project=live",
    "verify": "pnpm lint && pnpm typecheck && pnpm test && pnpm build && pnpm e2e"
  }
}
======= END FILE =======

======= FILE: playwright.config.ts =======
import { defineConfig } from "@playwright/test";

export default defineConfig({
  outputDir: "test-results",
  use: { trace: "retain-on-failure", screenshot: "only-on-failure" },
  projects: [
    // Route-mocked API; no network.
    { name: "mocked", testMatch: /.*\.mocked\.spec\.ts/ },
    // Real staging API. Needs storageState from a signed-in test account.
    {
      name: "live",
      testMatch: /.*\.live\.spec\.ts/,
      use: { baseURL: "https://staging.tidepool.example", storageState: ".auth/live.json" },
    },
  ],
});
======= END FILE =======

======= FILE: e2e/booking.live.spec.ts =======
import { test, expect } from "@playwright/test";

// Creates a real booking on staging, then cancels it.
test("book and cancel a trial lesson", async ({ page }) => {
  await page.goto("/lessons/trial");
  await page.getByRole("button", { name: "Book" }).click();
  await expect(page.getByText("Booked")).toBeVisible();
  await page.getByRole("button", { name: "Cancel booking" }).click();
  await expect(page.getByText("Cancelled")).toBeVisible();
});
======= END FILE =======

======= FILE: .github/workflows/ci.yml =======
name: CI
on:
  pull_request:
  push:
    branches: [main]
jobs:
  verify:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v5
      - uses: pnpm/action-setup@v4
      - run: pnpm install --frozen-lockfile
      - run: pnpm exec playwright install --with-deps chromium
      - run: pnpm verify
======= END FILE =======

======= FILE: .gitignore =======
node_modules/
.next/
test-results/
.auth/
======= END FILE =======
