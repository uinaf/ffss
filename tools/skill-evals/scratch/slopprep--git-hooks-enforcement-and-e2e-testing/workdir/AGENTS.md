# AGENTS.md

`csv-to-json-cli` converts a CSV file to JSON. The shipped surface is one
executable, `csv2json` → `dist/cli.js`, which takes an input CSV and an optional
output path; with no output path it prints JSON to stdout.

## Layout

```
src/cli.ts       argument handling and IO (the shipped entrypoint)
src/parser.ts    parseCsv: the only exported behavior
e2e/cli.test.ts  spawns the built dist/cli.js as a child process
e2e/fixtures/    sample.csv and the expected.json it must produce
.git-hooks/      pre-push adapter, activated by npm install
```

## Lifecycle

| Stage | Command | Notes |
| --- | --- | --- |
| bootstrap | `npm install` | also activates the pre-push hook via `prepare` |
| verify | `npm run verify` | the canonical gate; the only one to run |
| teardown | `npm run clean` | removes `dist/`; tests clean their own temp dirs |

`npm run verify` is noninteractive, bounded, and the same command the pre-push
hook and CI run. Individual steps (`typecheck`, `lint`, `deadcode`, `build`,
`test`) exist for fast iteration; `verify` is what decides whether work is done.

## Proof boundary

- `jest` unit project covers `parseCsv` directly, with no mocking of the module
  under test.
- `jest` e2e project is the only real-surface proof: it runs the built CLI as a
  process against real files and checks exit code, stdout, stderr, and output
  files, including two failure paths.
- `knip` owns dead-code detection; `tsc --noEmit -p tsconfig.test.json` is the
  only step that type-checks `e2e/`.
- A green `build` proves nothing about behavior. Do not report a change as
  working on the strength of compilation, and do not mock `parseCsv` in a test
  whose purpose is to catch a broken build.

Details and rationale: [setup-notes.md](setup-notes.md).
