# Effect Readiness

Use this sequence for a repository that already depends on Effect, or when the
user has explicitly chosen Effect. Do not introduce Effect merely to raise an
agent-readiness grade. Effect implementation conventions remain owned by the
repository's current guidance and its repository-local installation of the
upstream [Effect skills](https://github.com/Effect-TS/skills); this reference
owns only the infrastructure that makes those conventions reproducible and
verifiable.

## Audit

Inspect these surfaces before changing the repository:

1. **Agent guidance** — confirm the upstream `effect-ts` skill is available
   through a repository-owned path such as `.agents/skills/effect-ts/SKILL.md`
   and that the repository can reproduce that installation from
   `Effect-TS/skills`. A machine-global copy does not count because a clean
   runner or worktree cannot rely on it.
2. **Version ownership** — find the package manager, lockfile, installed Effect
   channel, and all `effect` and `@effect/*` packages. They should be owned in
   one checked-in place and kept on compatible, aligned versions.
3. **Research source** — check whether repository guidance expects Effect source
   at `.repos/effect`. If it does, confirm the checkout exists or that an
   idempotent setup path can create it.
4. **Runtime boundary** — identify the application Layer graph and the one
   top-level runtime entrypoint. Boot failures must exit non-zero with a useful,
   redacted cause.
5. **Configuration** — verify required configuration is decoded before the app
   reports ready. Missing or invalid values should fail with actionable field
   context without printing secrets.
6. **Testing** — inspect whether Effect programs use `@effect/vitest`, Layers,
   `TestClock`, and test services instead of ad hoc `runPromise`, real sleeps,
   or repeated local provisioning.
7. **Real-surface proof** — require at least one smoke or integration check
   through the built runtime and a real adapter boundary. In-memory Layers are
   useful unit proof but are not automatically end-to-end evidence.
8. **Observability** — confirm meaningful operations produce structured logs or
   spans that an agent can query. Preserve error tags or cause classification
   while redacting credentials and payloads.
9. **Lifecycle and isolation** — prove scoped resources release on success,
   failure, and interruption. Keep ports, databases, queues, files, and runtime
   instances isolated per worktree when agents run concurrently.

## Repository-Local Skill

Install `effect-ts` from `Effect-TS/skills` into the repository rather than a
user-global skill directory. Use the repository's existing bootstrap or skill
installer, and own enough source or lock metadata that a clean runner can
reproduce the same guidance without an interactive choice. For example, the
Agent Skills installer can be run from the repository root without its global
flag:

```bash
pnpm dlx skills add Effect-TS/skills -y -s effect-ts
```

Confirm the declared agents discover the installed skill from the repository.
If the repository checks in installed skill files, record their upstream
provenance; if setup generates them, pin the source through the repository's
checked-in setup or lock contract. Do not award repository grade B for Effect
implementation or refactor task classes while this guidance exists only in a
developer's global agent home.

## Source Setup

The upstream Effect guidance supports three shapes for `.repos/effect`:

- a squashed git subtree
- a git submodule
- an ignored local clone created by a checked-in prepare task

Treat that as a repository-shape decision. Ask before choosing when the owner has
not established a preference. For a research-only checkout, prefer the ignored
local clone: it keeps the application repository small and avoids making every
contributor manage a submodule.

Use `https://github.com/Effect-TS/effect` as the canonical source. Match its
channel to the installed packages: Effect v4 uses the default branch, while
Effect v3 uses the `v3` branch. Prefer the release tag matching the installed
`effect` package when one exists; otherwise own an exact commit in checked-in
configuration. Clean worktrees must resolve the same source instead of whatever
the remote branch happens to contain.

If an ignored local clone is selected:

- add `.repos/effect` to `.gitignore`
- provide an idempotent setup script that uses a partial or shallow clone and
  checks out the owned ref when missing
- wire it through the repository's existing bootstrap or prepare contract
- validate the remote and ref of an existing checkout; align a clean stale
  checkout to the owned ref, but stop with an actionable repair command rather
  than overwrite local source changes

For repositories whose managed worktrees support `.worktreeinclude`, use the
narrow `/.repos/effect/` pattern as an optional speed and availability layer
when the checkout size is acceptable. It gives each worktree an isolated source
checkout without requiring network access during creation. Do not copy all of
`/.repos/`, and do not treat the copied snapshot as the source of truth: the
prepare task must still validate and align it to the ref owned by that worktree's
checked-in configuration.

The prepare task remains the portable contract for manual, remote, custom-hook,
or otherwise unsupported worktrees. A shared machine cache is a later
optimization, not the default. Never symlink parallel worktrees to one mutable
Effect checkout: a fetch, checkout, or reset would change the source underneath
every agent.

Use a submodule only when the repository deliberately wants the source commit in
its Git contract. Use a subtree only when it deliberately vendors the source.
Neither is the default for an agent research dependency.

## Effect-Specific Readiness Order

Apply the general readiness order with Effect-specific proof:

1. **Boot** — expose one command that constructs the production Layer graph,
   starts the runtime, and reports initialization failure through the process
   exit status.
2. **Smoke** — exercise the built runtime, readiness signal, or shipped CLI
   through a real process. Include one invalid-config failure probe.
3. **Interact and E2e** — cross the real transport or adapter boundary; do not
   substitute a Layer that bypasses the seam under test.
4. **Enforce** — put typecheck, Effect-native tests, the runtime smoke, and any
   stable lint rules in the canonical local gate, then reuse it from CI.
5. **Observe** — expose structured Effect logs, spans, metrics, or a
   machine-readable health signal at meaningful operation boundaries.
6. **Isolate** — build per-worktree configuration and infrastructure Layers so
   parallel agents do not share mutable state accidentally.

## Deterministic Tests

- Prefer `@effect/vitest` for Effect programs.
- Use `it.effect` for normal Effect tests and opt into live services only when
  the test boundary requires them.
- Share service setup with `layer(...)`; use isolated Layer instances when
  state leakage would invalidate a test.
- Use `TestClock` for retries, schedules, timeouts, and polling. Real sleeps make
  autonomous verification slow and flaky.
- Test typed failure tags and recovery behavior, not only successful values.
- Add a resource-lifecycle test that demonstrates finalizers run after failure
  or interruption.
- Keep one honest adapter or process-level test alongside in-memory Layer tests.

## Observable Failures

The readiness target is evidence an agent can inspect, not a mandated coding
style. Verify that:

- business operations have stable observable identities
- structured logs carry useful identifiers without secrets
- expected failures retain their typed classification
- defects, interruption, and expected failures are distinguishable
- telemetry infrastructure is composed at the Layer boundary and shuts down
  with the application scope

## Verification

- Bootstrap the repository-local `effect-ts` skill from a clean checkout and
  confirm every declared agent can discover it without global state.
- Bootstrap the Effect source contract from a clean worktree when the repository
  requires it.
- Run the canonical local gate from a clean checkout.
- Start the real runtime and observe its ready signal.
- Exercise an invalid-config boot and confirm non-zero exit plus actionable,
  redacted output.
- Exercise one typed failure and confirm its classification survives logs or
  traces.
- Interrupt a scoped resource and confirm its finalizer releases the resource.
- Run time-dependent tests under `TestClock` and confirm they use no wall-clock
  delay.
- Confirm CI calls the same meaningful gate and branch protection requires it
  when merge enforcement is intended.
