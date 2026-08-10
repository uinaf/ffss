# Making an Existing Effect Service Verifiable

## Problem/Feature Description

A Node.js worker already uses Effect, but agents cannot work on it reliably.
The declared runner does not provide the upstream `effect-ts` skill, the
repository has no local fallback, and its stale agent guidance points at a
hand-maintained `.repos/effect` checkout. The `verify` script runs only
TypeScript, and no smoke command proves that the worker boots or that invalid
configuration fails usefully.

Add the smallest readiness infrastructure that makes the upstream guidance
available to the runner and gives the existing service a canonical verification
path. Do not copy Effect implementation conventions into repository docs,
maintain a separate Effect source checkout, redesign the worker's business
logic, or introduce a second dependency-injection system.

## Output Specification

Produce:

1. A repository-local fallback for `effect-ts` from `Effect-TS/skills`, owned by
   checked-in skill files or a reproducible setup and lock contract, and
   discoverable by the repository's declared agents.
2. Agent guidance that follows the upstream skill and points agents to the
   guidance and source shipped with the installed `effect` package, with the
   stale `.repos/effect` requirement removed.
3. The repository's ordinary dependency bootstrap proves that the installed
   Effect guidance is present.
4. A real-process smoke command that boots the production Layer graph, checks a
   machine-readable ready signal, and verifies invalid configuration exits
   non-zero with redacted, actionable output.
5. A canonical `verify` command that runs typecheck, tests, and the smoke check,
   reused by CI.
6. `effect-readiness.md` recording the proof and remaining gaps.

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
