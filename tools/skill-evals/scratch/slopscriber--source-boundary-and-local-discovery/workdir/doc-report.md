# Doc update report

## Source boundary

Only facts this repository owns were written into `README.md` and `AGENTS.md`:

| Source | Use |
|---|---|
| `package.json` scripts and fields | Canonical `setup`, `dev`, `verify` commands; ESM and `tsx` conventions |
| `docs/setup.md` | Canonical setup order (`npm ci` → `npm run setup` → `npm run dev` → `npm run verify`); kept as the single setup home and linked, not duplicated |
| Untracked local discovery note | Not promoted; summarized by category below |

`docs/setup.md` stays the canonical setup guide. `README.md` links to it; `AGENTS.md` carries the command contract agents need without restating the guide.

## Verification performed

- Read `package.json`; every documented command matches a declared script: `setup`, `dev`, `verify`.
- Checked `docs/setup.md` exists, so the `README.md` link resolves.
- Checked `AGENTS.md` exists, so the `README.md` link resolves.
- No command was executed, and no runtime behavior is claimed.

## Excluded evidence categories

The untracked discovery note is machine-local observation, not a repository contract. These categories were kept out of the checked-in docs, and their values are not reproduced here:

- A machine-specific checkout path and a developer home directory path.
- A specific workstation hostname.
- A private sibling repository named as a configuration source.
- A cloud account/credential profile set in one developer's shell.
- An internal deployment dashboard URL.
- A one-off local bootstrap helper binary used instead of the checked-in setup script.
- A claim that a command "was observed working" on one machine — single-host observation is not repo verification.

## Gaps

- `scripts/setup.ts` and the CLI entrypoint referenced by `package.json` scripts are not present in this checkout, so those paths are described by role rather than by literal path.
- No lockfile is present in this checkout; `npm ci` is documented because `docs/setup.md` and standard npm usage establish it, not because a lockfile was verified.
- No `CONTRIBUTING.md` or `SECURITY.md` exists. None was drafted; the repository has not established that policy.

## Approval needed before promotion

Each item below requires explicit maintainer confirmation that the repository owns it, that it is stable, and that its wording exposes no private or machine detail:

1. Making the local bootstrap helper a supported alternative to `npm run setup` — needs the helper checked in or replaced by a repo script first.
2. Documenting a dependency on the private sibling repository — needs the maintainer to state the dependency in repo-neutral terms and name the owner.
3. Documenting any cloud account profile or credential requirement — needs a maintainer-approved, non-secret description of the required access.
4. Linking the internal deployment dashboard — needs confirmation that the URL is appropriate for a checked-in doc and reachable by the intended readers.
