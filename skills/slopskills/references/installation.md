# Installation details

Open when composing an install command, choosing harness paths, handling source
locks, or migrating global skills to a repository.

## Example

List the source's skills, then install explicit names for one harness, here
Codex:

```sh
npx skills add Effect-TS/skills --list
npx skills add Effect-TS/skills --skill effect-ts --agent codex --yes
```

## Paths, locks, and migration

Codex discovers repo skills under `.agents/skills/`; retain the
installer-managed links for other selected harnesses. Keep installed files and
source locks according to repository policy. Check that references and
required companion skills survive installation.

Do not bundle the catalog's skill bodies into ffss: that would expose every
stack globally again. An explicitly requested global migration must also
update the owning installation manifest so synchronization does not reinstall
them. Complete global removal before installing local replacements, then
verify both scopes; do not assume a global removal preserved a same-named
local skill.
