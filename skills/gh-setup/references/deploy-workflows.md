# Deploy Workflows

Use for GitHub Actions that verify and promote a running application or
service. Provider mechanics belong in maintained Actions, the repository's
typed task surface, a tested local action, or infrastructure code.

## Shape and Trust

Use the smallest workflow layout the repository can operate:

- pull requests and merge queue: verification only, read-only credentials
- default-branch push: detect affected lanes, verify, build one payload, test
  that payload, deploy through a GitHub Environment
- manual promotion: validate a release tag, digest, deployment id, or exact
  SHA in a secretless job, then promote the already verified payload

Do not use `pull_request_target` to execute project code. GitHub Environment
branch rules constrain the workflow run ref, not a separately checked-out
manual input.

Keep complex ref resolution, change mapping, summaries, and provider branching
in tested typed repository code. Workflow YAML should orchestrate narrow
commands and maintained actions; a separate shell file does not make inline
shell complexity structured.

## Lane Detection and Required Checks

Use path filters for simple independent surfaces or the repository's dependency
graph for monorepos. Shared packages, lockfiles, workflows, containers, and
infrastructure paths must fan out to every consuming lane.

Do not use trigger-level path filters when branch protection requires the
workflow. Detect no-op lanes inside the workflow and end with one stable result
job that runs under `always()`, fails closed on unexpected skips, and reports
why a lane ran or did not run.

Detect lanes with a pinned maintained filter. On `pull_request` events it lists
files through the API, so the detect job needs `pull-requests: read` and no
checkout; [runner cost](runner-cost.md#critical-path) covers push and merge
queue events and the file-count limit.

A deploy job downstream of conditional lanes needs an explicit status function,
or GitHub ANDs the implicit `success()` and a skipped lane suppresses it. Gate on
`!cancelled() && needs.<job>.result == 'success'` for each upstream it ships
from. An `always()` job's own result is what downstream jobs read, so an
aggregator fails whenever an underlying result is neither success nor an
expected skip.

## Verified Payload

The same payload crosses build, e2e, and deploy:

- same-job filesystem output for a simple trusted static deploy
- immutable container digest
- package or GitHub Release asset
- provider-native package or deployment id

Do not rebuild after e2e. GitHub Actions artifacts are acceptable as short
same-run scratch storage only when the repository accepts their quota and
retention coupling; they are not the default production registry.

Manual redeploys identify the source commit, producing run, payload reference,
and digest. Prove payload existence before loading deploy credentials.

## Concurrency, Permissions, and Caches

- Verification may cancel superseded runs.
- Deploys serialize with one non-cancellable key per environment and lane,
  shared by automatic and manual paths.
- Workflow permissions start read-only and add OIDC or provider scopes only to
  the Environment-scoped deploy job.
- Monitoring and notification follow-ups stay read-only and receive no deploy
  credentials.
- Privileged deploys do not consume caches populated by untrusted pull requests.

Read [Environments](deploy-environments.md) for target and identity policy and
[credentials](deploy-secrets.md) for secret ownership and log hygiene.

## Deployment Proof and Handoff

Deploy success is the provider's accepted immutable payload plus the
repository's real monitoring or synthetic evidence, not a shallow curl added to
make CI look complete.

End each run with a concise handoff:

- environment and lane
- source revision and payload identity
- deployed URL or provider deployment id
- monitoring and alert surface
- rollback command or runbook

Keep summary formatting in a repository helper once it exceeds a few lines.
Use [deploy troubleshooting](deploy-troubleshooting.md) only for a concrete
failure or mismatch.
