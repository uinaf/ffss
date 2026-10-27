# Installation

```sh
npx skills add Effect-TS/skills --list
npx skills add Effect-TS/skills --skill effect-ts --agent codex --yes
```

Package-shipped skills (TanStack Intent): add `@tanstack/intent` as a dev
dependency, set `package.json#intent.skills` (for example
`["@tanstack/react-start", "@tanstack/start-client-core", "@tanstack/router-core"]`),
then run `intent install` and `intent list`. First-run setup without that
field needs an interactive terminal. Intent writes an `intent-skills` block
into the agent guide; that block replaces step 5's pointers for those skills.

- The installer places skills where each harness finds them; keep its
  managed links. No absolute links into a home directory or plugin cache.
- Keep installed files and lockfiles per repo policy. Check that references
  and companion skills came along.
- **Moving a global skill local** (when asked): remove it from the owning
  install manifest so sync doesn't bring it back, remove the global copy,
  install locally, then check both scopes.
