# Installation

```sh
npx skills add Effect-TS/skills --list
npx skills add Effect-TS/skills --skill effect-ts --agent codex --yes
```

TanStack skills ship inside the npm packages and load through
[Intent](https://tanstack.com/intent), not the skills CLI. Follow its
[consumer quick start](https://tanstack.com/intent/latest/docs/getting-started/quick-start-consumers)
with the repo's package manager, hooks included. Intent's `intent-skills` block
in the agent guide replaces step 5's pointers for those skills.

- The installer places skills where each harness finds them; keep its
  managed links. No absolute links into a home directory or plugin cache.
- Keep installed files and lockfiles per repo policy. Check that references
  and companion skills came along.
- **Moving a global skill local** (when asked): remove it from the owning
  install manifest so sync doesn't bring it back, remove the global copy,
  install locally, then check both scopes.
