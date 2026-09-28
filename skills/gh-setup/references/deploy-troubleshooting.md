# Deploy Troubleshooting

Use only after a concrete deploy failure or mismatch.

## Selection and Payload

- **Every lane deployed:** inspect affected-graph outputs. Shared packages,
  lockfiles, workflow, and infrastructure changes must fan out to every
  consuming lane.
- **Production differs from E2E:** compare source SHA, producing run, payload
  reference, digest/checksum, and deployed version. Remove any deploy-time
  rebuild or checkout drift; promote the verified payload.
- **Artifact quota blocks deploy:** replace cross-run Actions-artifact handoff
  with a durable release asset, registry version, image digest, or
  provider-native package. Same-run deploys may keep the verified build in one
  trusted job.
- **Unsafe manual ref:** validate and resolve inputs in a secretless job, prove
  payload existence, then load the Environment and credentials.

## Identity and Concurrency

- **Environment secret missing:** verify the job declares the intended
  Environment, its branch policy permits the run ref, protections completed,
  and the secret exists at that scope.
- **OIDC rejected:** compare actual repository, event, ref, Environment,
  audience, and provider role claims with the trust policy. Separate staging
  and production roles.
- **Older deploy wins:** every push and manual path for the same target/lane
  must share one non-cancellable `deploy-<environment>-<lane>` critical section.
- **Post-deploy job has provider credentials:** split monitoring, notification,
  and synthetic work into a read-only job with no inherited auth setup.

## False Green

If users fail while the workflow is green, remove shallow endpoint-check
claims. Confirm the deployed URL and payload identity, publish the repository's
real deploy marker, inspect monitoring and alert coverage, and expose the
rollback pointer.

Every deploy summary should name environment, lane, source commit, payload
identity, deployed URL, monitoring/alert evidence, and rollback route. Prefer a
small tested repo-owned summary helper over repeated workflow shell.
