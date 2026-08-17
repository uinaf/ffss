# Setup notes

## Hook activation

`npm install` runs `prepare` → `hooks:install`, which points Git at the
versioned hook directory and repairs its executable bit:

```
git config core.hooksPath .git-hooks
chmod +x .git-hooks/pre-push
```

Run `npm run hooks:install` directly if you changed `core.hooksPath` by hand.
Outside a Git checkout (tarball install, some CI caches) the step prints a
notice and exits 0 instead of failing the install.

`.git-hooks/pre-push` is an adapter, not a gate. Its whole body is
`exec npm run --silent verify < /dev/null`. It holds no assertions, no parsing,
no retries, and no policy, so the hook and CI cannot drift apart. To change what
is enforced, edit `verify` in `package.json`.

Bypass is `git push --no-verify`, which is a deliberate, visible act.

## The canonical gate

One command, used by humans, hooks, and CI alike:

```
npm run verify
```

It chains the existing package scripts in dependency order:

| Step | Command | Proves |
| --- | --- | --- |
| clean | `rm -rf dist` | no stale build satisfies the test |
| typecheck | `tsc --noEmit -p tsconfig.test.json` | `src/` **and** `e2e/` compile |
| lint | `eslint --ext .ts src/` | static rules |
| deadcode | `knip` | no unreachable files, exports, or types |
| build | `tsc` | `dist/cli.js` exists |
| test | `jest --ci` | unit + real-process end-to-end |

`clean` runs first on purpose: without it, a build that no longer compiles can
still pass because `dist/` holds yesterday's output. `build` runs before `test`
because the end-to-end suite consumes `dist/cli.js` rather than the sources.

Each step exits non-zero on failure and `&&` stops the chain, so the first real
failure is the last thing printed. Nothing prompts, and nothing reads stdin.

## Dead-code owner

`knip`, pinned to an exact version (`5.0.0`, no caret) in `devDependencies` and
configured by `knip.json`. Pinned and repository-owned so every machine and CI
run gets the same answer; a floating range turns a new release into a surprise
push failure. Upgrades are a deliberate commit.

`knip.json` declares the real entrypoints — the CLI plus both test suites — so
test-only helpers are not misread as dead. `include` is scoped to `files`,
`exports`, `types`, and `duplicates`: unreferenced-dependency reporting is left
off because it is the noisiest class and not what this gate is for.

Its first finding was `legacyParseCsv` in `src/parser.ts`, an export commented
"no longer used". It is deleted.

## What the real-process test proves

`e2e/cli.test.ts` spawns `dist/cli.js` with `process.execPath` as a child
process, against real files on disk, and reads back exit code, stdout, stderr,
and written output. It never imports or mocks `parseCsv`.

That distinction is the reason this work exists. The previous `src/parser.test.ts`
called `jest.mock('./parser')` and then asserted `typeof parseCsv === 'function'`
— it asserted against the mock, so it stayed green for any implementation,
including one that does not compile. Tests that reach inside the module cannot
observe a broken build, a bad `bin` path, or a compile error.

Four cases, chosen to cover both success and failure:

1. stdout mode — exit 0, empty stderr, parsed output equals
   `e2e/fixtures/expected.json`, and the output is 2-space pretty-printed.
2. file mode — exit 0, silent stdout, and the file on disk matches the same
   fixture byte for byte.
3. no arguments — exit 1 with the usage line on stderr and nothing on stdout.
4. missing input file — non-zero exit, stderr names `ENOENT` and the offending
   path, and no partial output file is left behind.

The fixture pins observed behavior, not wishes: `sample.csv` carries a padded
header (` email `), an indented row, and a short final row, so trimming and the
missing-column fill are both covered.

Isolation and cleanup: each case runs in its own `mkdtemp` directory, removed in
`afterEach` on pass or fail, so nothing is written into the checkout and cases
cannot observe each other. Child processes carry a 10s timeout and the suite a
30s one, so a hung CLI fails instead of hanging the push.

Running `npm test` alone, without a build, fails in `beforeAll` with the exact
command to run rather than a confusing missing-module error.

## Two config traps worth knowing

`tsconfig.json` excludes `**/*.test.ts`, so `npm run build` emits only shipped
code. Without it, `dist/` collected compiled test files, and the build broke
outright on `jest` globals that nothing supplied types for.

`tsconfig.test.json` therefore has to re-open that door with `"exclude": []`.
`extends` inherits `exclude`, so omitting the reset would leave `typecheck`
silently skipping every test file — green, and checking nothing. That is the
same failure shape as the mocked test it replaces.

## Verification status

Not exercised in the environment where these files were written: `npm install`,
`chmod`, and `node` were all denied, so no command in the table above has been
run and the `knip@5.0.0` pin is unconfirmed against the registry. Everything
here is static construction. First run on a machine with registry access should
be `npm install && npm run verify`; expect to adjust the `knip` pin to the
current 5.x and to tune `knip.json` if it reports anything beyond
`legacyParseCsv`, which is already deleted.
