docs/testing.md still says `npm run e2e` but we renamed the script last week.
Fix the doc.

======= FILE: package.json =======
{
  "name": "harbor-admin",
  "private": true,
  "scripts": {
    "test": "vitest run",
    "test:e2e": "playwright test",
    "verify": "npm run test && npm run test:e2e"
  }
}
======= END FILE =======

======= FILE: docs/testing.md =======
# Testing

`npm run verify` runs unit and browser tests. CI runs the same command.

To run only the browser tests:

```bash
npm run e2e
```

Traces for failed tests land in `test-results/`.
======= END FILE =======
