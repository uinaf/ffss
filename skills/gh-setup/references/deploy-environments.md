# Deploy Environments

Use when changing target selection, promotion policy, provider identity, or
manual deploys. Provider commands remain in repo-owned scripts or infrastructure
code.

## Environment Contract

- Declare one GitHub Environment per blast radius: for example `staging`,
  `production`, or an isolated preview.
- Keep production-only secrets and variables on that Environment.
- Use protection rules only for intentional human approval or policy gates.
- Publish jobs may suppress deployment records when no protection-rule app
  needs a deployment object. Running-service deploys keep records enabled and
  publish their target URL.
- Environment branch/tag policy constrains the workflow run ref, not a different
  ref checked out later.

## Identity and Isolation

Prefer provider federation/OIDC scoped to repository, Environment, intended
branch or tag, audience, and one deployment role. Separate staging and
production identities and state. Static credentials are an Environment-scoped
fallback with a documented reason.

When a repo uses SST or an equivalent app-plus-infrastructure deployer, map its
stages to GitHub Environments and keep provider state/resource ownership
isolated. The tool remains a thin promotion layer; it does not justify shared
production/staging state, secret CLI flags, or an unverified rebuild.

## Payload Promotion

Promote the exact payload proved by verification: same trusted job output,
release asset, registry version, provider-native package, or immutable image
digest. Record source commit, producing run, reference, and checksum/digest for
manual or older-version promotion. Verify existence and provenance before
loading credentials.

Runtime secrets belong in the provider secret store or selected Environment;
non-sensitive account/project/region names belong in Environment variables.
Pass secrets through environment variables or stdin and never print rendered
configuration.

## Manual Deploys

Treat `workflow_dispatch` as promotion, not a new build:

1. Validate environment, lane, and ref in a secretless job.
2. Resolve the ref to one immutable SHA and prove the corresponding payload.
3. For production, restrict to the default branch/current default SHA or a
   protected release tag explicitly supported by the repo contract.
4. Emit sanitized outputs, then load the target Environment and credentials.
5. Reuse the normal deploy concurrency key and downstream proof.

After deploy, hand off to repository-owned monitoring, alerting, synthetic
checks, and rollback. A shallow CI curl is not production evidence.
