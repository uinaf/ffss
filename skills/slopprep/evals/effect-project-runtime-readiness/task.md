# Making an Existing Effect Service Verifiable

## Problem/Feature Description

A Node.js worker already uses Effect, but agents cannot work on it reliably.
The upstream `effect-ts` skill is available only in a developer's global agent
home, and the repository's Effect guidance expects source at `.repos/effect`,
yet a fresh checkout has no setup path for either. Effect package versions have
drifted, tests call `Effect.runPromise` from plain Vitest tests and wait on real
timers, and the `verify` script runs only TypeScript. The worker has a Layer
graph, but no smoke command proves that it boots, invalid configuration is
actionable, or scoped resources close during interruption.

The team has selected the ignored local clone plus checked-in prepare task for
`.repos/effect`. Add the smallest readiness infrastructure that gives agents a
reproducible research source, deterministic Effect tests, a real runtime smoke,
observable failures, and cleanup proof. Do not redesign the worker's business
logic or introduce a second dependency-injection system.

## Output Specification

Produce:

1. A repository-local installation of `effect-ts` from `Effect-TS/skills`,
   owned by checked-in skill files or a reproducible setup and lock contract,
   and discoverable by the repository's declared agents without global state.
2. `.gitignore` and `scripts/prepare-effect.sh` for the selected Effect source
   setup, wired into the repository's existing prepare/bootstrap contract. Use
   the canonical Effect repository, own an exact source ref in checked-in
   configuration, use a narrow `.worktreeinclude` entry as an optional seed,
   and safely align clean stale checkouts without overwriting local changes.
3. Aligned `effect` and `@effect/*` dependencies in `package.json` and its
   lockfile.
4. Effect-native tests using `@effect/vitest`, Layers, and `TestClock`, including
   typed failure and interrupted-resource cleanup coverage.
5. A real-process smoke command that boots the production Layer graph, checks a
   machine-readable ready signal, and verifies invalid configuration exits
   non-zero with redacted, actionable output.
6. A canonical `verify` command reused by CI.
7. `effect-readiness.md` recording the proof and remaining gaps.

## Input Files

=============== FILE: package.json ===============
{
  "name": "email-worker",
  "private": true,
  "scripts": {
    "typecheck": "tsc --noEmit",
    "test": "vitest run",
    "verify": "npm run typecheck"
  },
  "dependencies": {
    "effect": "4.0.0-beta.12",
    "@effect/platform-node": "4.0.0-beta.11"
  },
  "devDependencies": {
    "@effect/vitest": "4.0.0-beta.10",
    "typescript": "^5.9.0",
    "vitest": "^3.0.0"
  }
}

=============== FILE: src/retry.test.ts ===============
import { Effect } from "effect"
import { describe, expect, it } from "vitest"

describe("retry", () => {
  it("waits before retrying", async () => {
    const result = await Effect.runPromise(
      Effect.sleep("1 second").pipe(Effect.as("retried"))
    )
    expect(result).toBe("retried")
  })
})

=============== FILE: src/main.ts ===============
import { Effect, Layer } from "effect"

export const AppLayer = Layer.empty

export const program = Effect.logInfo("worker ready").pipe(
  Effect.provide(AppLayer)
)

Effect.runPromise(program)
