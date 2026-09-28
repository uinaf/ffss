# Dependency Updates

Choose one update bot per repository. Running both opens duplicate pull
requests. Dependabot alerts are a separate GitHub feature and stay on under
either choice. Dependabot security updates stay off; Renovate raises its own
vulnerability pull requests.

## Choose

| Signal | Choice |
| --- | --- |
| Only npm, Go, Cargo, or GitHub Actions manifests | Dependabot is sufficient; Renovate is equivalent |
| Pins in `mise.toml`, OpenTofu or Terraform providers, or annotated version variables | Renovate; Dependabot has no manager for these |
| Container images pinned by digest | Either; both update tag and digest together |
| Organization already runs one bot on most repositories | Match it; one mental model beats a marginal feature |
| Fork or mirror with no owned manifests | Neither |

The hosted Mend Renovate app is free for private repositories on the
Community plan. Its `IAC`, `SAST`, and `SCA` columns are separate paid Mend
scanners, unrelated to dependency updates. Verify the current plan on the
[Mend-hosted overview](https://docs.renovatebot.com/mend-hosted/overview/)
before relying on any limit.

Neither bot rewrites a sha256 checksum stored beside a version. Ansible and
script pins with checksums stay manual, or let the target fetch the upstream
`.sha256` file at install time so only the version needs bumping.

## Renovate

- Extend the target organization’s shared preset and keep
  repository files to opt-outs, approvals, and gated automerge opt-ins. Encode
  schedule, release age, grouping, commit prefixes, and registry overrides
  once. The preset repository must be public: the hosted app reads public
  repositories with a token that cannot see private presets, and the failure is a
  "Cannot find preset's package" issue on every public consumer.
- The [uinaf preset](https://github.com/uinaf/renovate-config) runs daily with
  a seven-day `minimumReleaseAge`, one day for `github-actions` and `docker`,
  and none for `uinaf/.github`; patch and minor updates group per manager and
  majors stay separate.
- Use [GitHub-native automerge](#faster-github-automerge) when enforceable
  required checks are ready. Keep `platformAutomerge: false` for repositories
  without those gates; Renovate then waits for visible checks on a later run.
  Keep majors on `dependencyDashboardApproval`.
- Update repository checks that require `.github/dependabot.yml` to require
  `renovate.json` instead, or the migration commit fails its own hook.
- Keep **Require config file** on in the Mend organization settings so
  unmigrated repositories receive nothing while they still run Dependabot.
  Turn **Create onboarding PRs** off when migrating by commit.
- Migrate a repository in one commit: add `renovate.json`, delete
  `.github/dependabot.yml`, update any documentation that names Dependabot.
  Renovate skips onboarding when a config already exists on the default
  branch.
- OpenTofu repositories set `registryUrls` to `https://registry.opentofu.org`
  for the `terraform-provider` and `terraform-module` datasources and disable
  the `hashicorp/terraform` dependency, which otherwise tracks Terraform
  releases for `required_version`.
- Use `dependencyDashboardApproval` for majors that need a planned migration
  instead of `enabled: false`; the update stays visible on the dashboard.
- Validate with `npx --yes --package renovate -- renovate-config-validator`
  before pushing. The validator checks option names only; it does not
  resolve preset names. A misspelled preset such as `:pinDigests` instead
  of `docker:pinDigests` passes locally, then opens an "Action Required"
  issue and blocks all pull requests until fixed.

## Faster GitHub Automerge

Renovate-owned merging waits for another bot run after CI. The free Mend plan
currently schedules active repositories every four hours, with one concurrent
job per organization; webhooks can enqueue additional jobs. Check the current
[hosted scheduling limits](https://docs.renovatebot.com/mend-hosted/overview/#resources-and-scheduling).
GitHub-native automerge removes that post-CI wait once Renovate has enabled it
on the PR; update discovery and rebases still depend on Renovate.

Roll out to a few repositories first:

1. Inventory effective rules and every authorized default-branch writer using
   [repository settings](repo-settings.md). Require explicit verification and
   security checks before enabling native automerge. GitHub waits only for
   required checks; adding a workflow later does not make it required. Prefer
   non-strict checks to avoid serial rebases after each merge: they validate
   the PR head, not its combination with the latest base. Use strict checks
   or a merge queue when integration risk requires that extra proof.
2. Require a stable `always()` result gate for conditional jobs or matrices.
   Check upstream results explicitly: intentional no-op lanes may pass, but
   failures, cancellations, and unexpected skips must fail the gate. Every PR
   must trigger the gate, including dependency-only changes.
   Required pre-merge verification must compile or test the tools used by
   push-only release or apply jobs with the same pinned runtime and
   dependencies when those pins change. Green unrelated checks are not
   compatibility proof.
3. Apply the writer compatibility and bypass rules in
   [repository settings](repo-settings.md#collaboration-policy). The new
   required checks constrain all targeted branch updates, not only Renovate
   merges; name the ruleset accordingly. Do not add mandatory review merely
   to enable automerge.
4. Enable GitHub's **Allow auto-merge**, then opt ready repositories into
   `platformAutomerge: true` with `automergeType: "pr"`. Keep the shared preset
   conservative until every consumer has enforceable gates. Preserve existing
   eligibility, release age (including a seven-day policy), and manual major
   or digest rules; changing the merge mechanism does not widen eligibility.
5. Audit Renovate status checks too. Preserve release-age enforcement through
   strict branch creation or a required `renovate/stability-days` check;
   an optional pending status does not block native merging. Cover artifact
   failures through required CI (for example frozen-lockfile installation)
   or a bot status. If requiring `renovate/artifacts`, set
   `statusCheckWhen.artifactError: "always"`; otherwise successful PRs never
   emit it. Ensure required Renovate contexts also have a valid path for human
   PRs, which Renovate does not process. Do not weaken artifact or release-age
   safeguards to clear a pending check. See [platform automerge requirements](https://docs.renovatebot.com/configuration-options/#platformautomerge).
6. Read back settings, effective required contexts, and PR auto-merge state.
   Existing PRs can need a follow-up Renovate run; confirm `autoMergeRequest`
   was enabled by Renovate rather than assuming the config armed it.
   Observe an eligible PR waiting while required CI runs, then merging after
   it passes. Confirm failures remain blocked and manual updates remain manual
   before expanding the cohort. Enabling auto-merge manually on an existing PR
   proves GitHub's gate, not that Renovate will enable it on future PRs.

Keep `platformAutomerge: false` if a merge-time schedule is mandatory: native
merging does not honor Renovate's `automergeSchedule`. To roll back, disable
native auto-merge on already-enqueued PRs as well as reverting the opt-in;
changing the config alone does not cancel GitHub's queued merges.

## Dependabot

- Configure only ecosystems and manifests that exist.
- Group patch and minor updates and separate majors.
- Prefix commits per ecosystem (`ci` for Actions, `deps` otherwise) so
  release tooling classifies them.
- Preserve compatibility constraints with `ignore` rules rather than closing
  pull requests repeatedly.

## Readback

After the first run, confirm the bot opened pull requests with the expected
prefix, grouping, and registry, and that the retired bot opened none. For
Renovate, the hosted job log on the Mend developer portal shows why a
repository produced nothing.
