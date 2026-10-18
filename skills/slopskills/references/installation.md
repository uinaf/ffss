# Installation

```sh
npx skills add Effect-TS/skills --list
npx skills add Effect-TS/skills --skill effect-ts --agent codex --yes
```

- The installer places skills where each harness finds them; keep its
  managed links. No absolute links into a home directory or plugin cache.
- Keep installed files and lockfiles per repo policy. Check that references
  and companion skills came along.
- Catalog skills stay out of ffss; bundling them would make every stack
  global again.
- **Moving a global skill local** (when asked): remove it from the owning
  install manifest so sync doesn't bring it back, remove the global copy,
  install locally, then check both scopes.
