---
name: gh-setup
description: "Configure GitHub settings, collaboration files, Actions, releases, and deployments. Use for setup or changes to those surfaces; excludes provider infrastructure and product code."
disable-model-invocation: true
---

# GitHub Setup

Make GitHub the enforceable shell around the repository's existing build,
verification, release, and deployment contracts.

## Scope and Route

Start with repository guidance and the files and commands that own the requested
change. Select the relevant route below before loading references.

- **Collaboration files:** inspect the local file and applicable owner defaults.
- **Workflow correction:** inspect the affected jobs, their inputs, permissions,
  dependencies, and required-check behavior. Preserve unrelated delivery policy.
- **Settings or delivery policy:** read the affected live settings and actors,
  including default-branch writers when branch rules or required checks change.
  Record the before-state and rollback path before authorized mutations.
- **Release or deployment:** inspect the owning scripts and runbook, then use
  the matching release or deploy route. For both, publish one immutable payload
  and deploy that payload instead of rebuilding it.

A local correction does not require a full live-policy inventory. Expand
inspection only when a dependency or changed trust boundary requires it.
Use repo-local commands as authority. If the claimed delivery surface cannot
build, verify, package, observe, or roll back reproducibly, report the missing
prerequisite instead of hiding it in workflow YAML.

## Shared Contract

- Pull requests execute untrusted code with read-only credentials.
- Trusted release and deploy jobs load credentials only after verification and
  input or ref validation.
- Workflow permissions default to read-only or `{}` and widen per job.
- High-trust remote Actions use reviewed immutable pins with an update path.
- Environment secrets and policy match the release or deployment blast radius.
- Release, publish, signing, promotion, and deploy critical sections are
  non-cancellable and reconcilable.
- One verified payload crosses build, test, publish, and deploy boundaries.
- Required checks use a stable final result when matrices, conditional lanes,
  or no-op paths make individual jobs unstable.
- Live settings and downstream state changed by the task must be read back;
  a green workflow alone does not prove those changes.

Read [security baseline](references/security-baseline.md) when adding or
changing scans, required checks, schedules, or repository security features.

Read [Actions security](references/actions-security.md) when adding workflows or
changing code execution, credential, publication, signing, or deploy boundaries.

Read [runner cost](references/runner-cost.md) when configuring triggers, runners,
concurrency, expensive job selection, or the wall time of a
required check.
For new release or deploy machinery, start from the closest [maintained
implementation](references/implementations.md). For local corrections, consult
an example only when repository code leaves an implementation question unresolved.
Adapt its contract, not literal versions, identities, or provider details.

## Repository Policy

Read [repository settings](references/repo-settings.md) when changing merge methods,
rulesets, required checks, signed commits, tags, Actions policy, Environments,
the cost-safe organization security baseline, CodeQL posture, and repository
metadata.

Preserve existing approval, actor, signed-commit, tag, and status-check rules
unless the requested change owns them. Running a check and enforcing it are
separate operations. Before requiring pull requests or a check, inventory
release bots, dependency bots, generated writebacks, and maintainers who still
write the default branch.

Do not require pull requests by default. When repository policy permits direct
updates and a reproducible local gate is mirrored by default-branch CI, allow
verified fast-forward pushes. Require pull requests only for pre-merge review,
untrusted contributions, merge queues, checks that must pass before the default
branch moves, or an explicit owner policy. Post-push CI detects regressions
after the branch moves, so run the local gate before pushing and monitor CI to
completion.

## Collaboration Files

Read [templates](references/templates.md) when adding or aligning pull-request
templates, issue forms, `SECURITY.md`, `CONTRIBUTING.md`, or shared community
defaults.

- Prefer public owner-level defaults only for policy true across every repo.
- Keep templates short and evidence-oriented; avoid checklist theater.
- Public security guidance needs a working private reporting route. Private
  repos use an existing private maintainer channel.
- Read [dependency updates](references/dependency-updates.md) before adding
  or migrating Dependabot or Renovate. Run one bot per repository. Keep
  Dependabot alerts on; leave Dependabot security updates off under Renovate,
  which raises its own vulnerability pull requests.

## Release and Deploy Routes

Release work uses:

- [release workflows](references/release-workflows.md) for trust, publication,
  signed writeback, immutable releases, recovery, and completion proof
- [publish targets](references/release-targets.md) for npm, Swift/CocoaPods, Go,
  Rust, GitHub Actions, Homebrew, and monorepos
- [semantic-release](references/semantic-release.md) only when that tool is selected
- [release troubleshooting](references/release-troubleshooting.md) only after a
  concrete failure or inconsistent durable state

Deploy work uses:

- [deploy workflows](references/deploy-workflows.md) for triggers, lane
  detection, verified payloads, concurrency, and monitoring handoff
- [Environments](references/deploy-environments.md) when target selection,
  protection, OIDC, or provider boundaries change
- [credentials](references/deploy-secrets.md) when secret ownership or logging changes
- [deploy troubleshooting](references/deploy-troubleshooting.md) only after a
  concrete failure

## Verify and Finish

Run repository gates plus `actionlint` and `zizmor` when workflows changed.
When live delivery is in scope and authorized, perform its narrowest safe
proof. A workflow-only change does not authorize a release or deployment.
Continue authorized local work when live proof is unavailable and report the gap.
Dry-runs and static inspection cannot prove immutable publication, signed writeback,
registry or tap parity, deployment, monitoring, or rollback.

After authorized live changes, read back every setting, Environment, rule,
release, registry, tag, deployment, or downstream pointer in scope. On partial
failure, reconcile durable state before retrying; never create a new version or
mutate an immutable release merely to make a workflow green.

## Output

```text
files: changed GitHub and documentation surfaces
settings: live changes and readback, or not checked
delivery: target and immutable payload boundary
evidence: local, workflow, and live proof actually exercised
risks: remaining authority, recovery, or downstream gaps
```
