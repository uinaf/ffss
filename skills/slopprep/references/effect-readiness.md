# Effect Readiness

Use this only when the repository already uses Effect or the user has chosen it.
Do not introduce Effect to improve a readiness grade. Implementation and testing
conventions belong to the upstream
[`effect-ts`](https://github.com/Effect-TS/skills/tree/main/skills/effect-ts)
skill and the guidance shipped with the installed `effect` package; do not copy
those conventions here.

## Guidance Availability

1. Check whether the declared agent environment already exposes `effect-ts`.
   If it does, use that installation.
2. If it does not, install `effect-ts` from `Effect-TS/skills` into the
   repository through its existing skill or bootstrap contract. Check in the
   installed package or pin enough setup metadata that a clean checkout can
   reproduce it without an interactive choice.
3. Confirm the declared agents discover the skill. A copy in an unrelated user
   profile does not prove that the selected runner can use it.
4. Follow the upstream skill's current setup and research instructions. Keep
   only repository-specific deviations or invariants in local agent guidance.

The repository fallback can use its existing installer. For example:

```bash
pnpm dlx skills add Effect-TS/skills -y -s effect-ts
```

Do not maintain a separate Effect source-checkout design in this reference. The
upstream skill owns where agents read Effect guidance and source.

## Verification

- Confirm `effect-ts` is discoverable in the declared environment or from the
  repository fallback.
- From a clean checkout, run the repository's ordinary dependency bootstrap and
  confirm the installed Effect guidance expected by the upstream skill exists.
- Run the repository's canonical local gate and task-relevant runtime proof.
- Apply the general readiness checks for failures, observability, cleanup,
  isolation, CI, and result submission; do not duplicate them here.
