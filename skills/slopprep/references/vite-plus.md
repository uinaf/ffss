# Vite+ Migration and Upgrades

Rules for moving a repository onto Vite+ or across its releases that the
upstream docs leave implicit. Cache and timing rules stay in
[fast-portable-execution.md](fast-portable-execution.md#selection-and-caching).

## Migrate

- Run the target migrator before any `pnpm update` or `pnpm add`, with the
  original lockfile installed: it detects the source Vitest version from it.
  Projects below Vitest 4 go to a 0.x release first.
- Convert tsup with `pnpm --package=vite-plus@<exact> dlx vp migrate --full`;
  plain `vp migrate` skips it. On 1.0.0-rc.0 it leaves an intermediate
  `tsdown.config.ts`, a direct `tsdown` dependency, and a `tsdown` script.
  Move the settings into the `pack` block of `vite.config.ts`, switch the
  script to `vp pack`, drop `tsdown`, delete the intermediate config, and
  prove output parity for ESM, CJS, `.d.ts`, and `.d.cts`.
- The 1.0 migrator exits 0 with `REVIEW` items outstanding; resolve all of
  them before committing. `BLOCK` stops dependency updates: fix and rerun.
  Save the review report before reinstalling.
- Restore `vite: catalog:` where `vp up` wrote a literal alias (fixed in
  0.3.0), and point the catalog entry at
  `npm:@voidzero-dev/vite-plus-core@<target>`.
- Under pnpm 12, unknown keys in `pnpm-workspace.yaml` fail the install. Move
  them to a non-pnpm owner instead of relaxing validation or downgrading pnpm.

## Checks and tasks

- Set `lint.options.typeCheck: true` so `vp check` reports type errors. Drop a
  separate `tsc --noEmit` only after one run shows identical diagnostics. A
  framework checker such as `vue-tsc` stays as the only standalone type
  check, on TypeScript 6 until the TypeScript 7.1 API ships.
- 1.0 no longer ships the `oxlint` and `oxfmt` binaries. Point editors and
  scripts at `vp lint --lsp`, `vp fmt --lsp`, and `vp fmt --stdin-filepath`.
- From Vitest 5, `vp test` no longer searches parent directories for a
  config; package-level runs need `--config <root>/vite.config.ts --dir .`.
- From 0.3.0, bare app commands at the workspace root prefer `defaultPackage`
  or the root config, so a green root run does not prove workspace coverage.
  Use `vp run -r <task>`.
- Built-ins don't dispatch same-named package scripts; use `vp run <script>`
  or `vpr <script>`.

## Runtimes and CI

- 1.0.0-rc.0 requires Node `^22.18.0 || ^24.11.0 || >=26.0.0`. The migrator
  bumps version files, not CI images, containers, or published
  `engines.node`. To keep Node 20 in the public contract, constrain
  contributors with `devEngines.runtime`.
- `vp staged` on 1.0 needs Node `^22.22.1 || ^24.11.0 || >=26.0.0` and Git
  2.32+ wherever the hook runs; the migrator's `.nvmrc` of 22.18.0 is too low.
- Projects tested with `@cloudflare/vitest-pool-workers` 0.22.0 stay on their
  0.x Vite+ pin until
  [workers-sdk#15618](https://github.com/cloudflare/workers-sdk/issues/15618)
  ships: every Worker test file fails to start on 1.0.0-rc.0.
- In secret-bearing release, publish, or deploy jobs keep dependency caches
  off (`setup-vp` `cache` defaults to `false`; don't carry over
  `cache: pnpm`), install fresh, and verify the exact payload before
  credentials are used.
- Never run `vp implode` to align CI paths; it deletes managed runtimes and
  global state. The toolchain container is for build tooling, never a
  production runtime.
