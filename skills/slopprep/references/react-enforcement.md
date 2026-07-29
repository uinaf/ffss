# React Enforcement

Use this sequence when a React or React Native repository lacks deterministic,
framework-specific checks. React Doctor can cover state and effects,
performance, architecture, security, and accessibility, but each integration
surface has a different role.

## Choose Mechanical Surfaces First

1. Inspect the package manager, React framework, workspace layout, existing
   linter, canonical local gate, hooks, CI provider, and action-pinning policy.
2. Run a one-off full React Doctor scan and record the baseline. Do not add a
   blocking gate before distinguishing existing debt from actionable signal.
3. Add React Doctor to the repository's owned dependencies and commands. Pin it
   through the existing lockfile or tool-version owner; do not leave
   `@latest` in a checked-in script or workflow.
4. Reuse the existing linter:
   - For oxlint, add `oxlint-plugin-react-doctor` and enable the rules the team
     trusts in `.oxlintrc.json`.
   - For ESLint flat config, add `eslint-plugin-react-doctor` with
     `recommended` plus only the framework presets that match the repo.
   - Do not replace or add a second linter solely to host the plugin.
5. Put the lint command in the canonical local gate. Add a full React Doctor CLI
   scan when project-level security, configuration, or artifact checks matter;
   the standalone lint plugins do not run those project-level checks.
6. Add CI using the existing provider and workflow conventions. On GitHub,
   prefer the official `millionco/react-doctor` Action for changed-PR
   diagnostics, and follow the repo's action pinning and update policy.
7. Start with changed-PR scope and advisory behavior. Once the team trusts the
   signal, make new errors or warnings blocking and require that check in branch
   protection.

This gives three useful feedback speeds:

- linter: fast editor and local feedback
- canonical gate: reproducible pre-push proof
- CI: unavoidable pull-request enforcement

## CI Safety

- Fetch full history when the scanner needs the pull request merge base.
- Grant write permissions only for enabled PR comments, inline reviews, or
  commit statuses. A log-only or gate-only job should stay read-only.
- Keep fork behavior in mind: GitHub does not grant write tokens to fork pull
  requests, so comments may be unavailable even when the scan still runs.
- In a monorepo, scope the directory or projects explicitly when automatic
  discovery would scan unrelated packages.
- Keep telemetry policy explicit. React Doctor supports opting out when the
  repository or organization requires it.

## Optional Agent Integration

Agent instructions and edit hooks can shorten the feedback loop, but they are
not a substitute for the repository gate or CI. Add them only after mechanical
checks exist, or when the user explicitly asks for that extra feedback surface.
Do not count a globally installed skill as readiness evidence because activation
is contextual and cannot guard every contribution.

## Verification

- Run the existing lint command and confirm React Doctor rules execute.
- Run the canonical local gate from a clean checkout.
- Introduce a disposable known-bad fixture or temporary changed line, confirm
  the relevant local and CI gate detects it, then remove it.
- Confirm the initial CI workflow is advisory, or that its chosen blocking
  severity matches the accepted baseline.
- For GitHub, confirm the check is required when the intended outcome is merge
  enforcement; a red but optional check does not block a pull request.

Official references:

- React Doctor GitHub Actions setup:
  <https://www.react.doctor/docs/ci-and-prs/github-actions-setup>
- React Doctor ESLint and oxlint plugins:
  <https://www.react.doctor/docs/configuration/eslint-and-oxlint-plugins>
- React Doctor agent installation:
  <https://www.react.doctor/docs/getting-started/install-for-coding-agents>
