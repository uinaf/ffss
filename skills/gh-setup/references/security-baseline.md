# CI and Security Baseline

The cheapest setup that catches leaked secrets, unsafe workflows, and
vulnerable dependencies, for owners whose agents push directly to default
branches. Every added job, trigger, or schedule names its threat and its
weekly minute cost.

## Detection

- Public repositories: GitHub secret scanning and push protection, both free.
  They block provider-pattern secrets on command-line, web, and API pushes, and
  a bypass emails the owner
  ([push protection](https://docs.github.com/en/code-security/secret-scanning/introduction/about-push-protection)).
- Private repositories: the Gitleaks CLI on the pushed range.
- Actionlint and Zizmor when a push changes `.github/`, action metadata, or
  their configuration.
- Renovate for updates and vulnerability pull requests, with Dependabot alerts
  on (free on every repository).
- SHA pinning required in the Actions policy for the organization or
  repository.

## Shape

- Each repository has one `verify` job on push to the default branch, pull
  requests, and `workflow_dispatch`. A repository with nothing to build runs
  it on push and dispatch.
- The scan is the last step of `verify`
  ([uinaf/.github `actions/scan`](https://github.com/uinaf/.github#scan)). It
  scans the pushed range on push and full history on dispatch, and passes
  through pull requests. It adds seconds to a job GitHub already bills by the
  whole minute
  ([rates](https://docs.github.com/en/billing/reference/actions-runner-pricing)).
- Merges land as pushes, so the push scan covers them. New advisories arrive
  as Dependabot alerts and Renovate pull requests. Dispatch a full-history
  scan after a scanner-rule upgrade.
- Concurrency cancels superseded pull-request runs only; every push run
  finishes so its range is scanned.
- A finding fails the pushed commit's `verify` run, and GitHub's failed-run
  email is the notification.

## Rules

- Require `verify` where pull requests merge without a human, such as Renovate
  platform automerge, which needs a required check to wait for
  ([Renovate](https://docs.renovatebot.com/configuration-options/#automerge)).
  Administrators keep an always-bypass so direct pushes land at once.
- Require checks only from workflows that always run: a job skipped by `if:`
  reports success, while a path-filtered workflow leaves the check pending
  ([required checks](https://docs.github.com/en/pull-requests/collaborating-with-pull-requests/collaborating-on-repositories-with-code-quality-features/troubleshooting-required-status-checks)).
- Keep deletion, force-push, and signed-commit protections in their own
  ruleset.
- When renaming or removing a required check, update the ruleset and confirm
  every open pull request reports the new name.
