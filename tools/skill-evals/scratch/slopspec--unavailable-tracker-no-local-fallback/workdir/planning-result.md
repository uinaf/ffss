# Planning result — billing retry plan

- **Destination:** Jira project `PAY`, one Story. `AGENTS.md:3` declares Jira `PAY` as the work tracker and disables GitHub Issues for internal product work.
- **Published:** no. The draft below is not durable until someone pastes it into `PAY`.
- **Shape:** single work item. The agreed scope is one cohesive behavior change plus its provider-contract coverage — one agent can complete and verify it in one fresh context, so no epic or child tickets.
- **Blocker:** no Jira write path. `AVAILABLE_ACCESS.md` reports Jira connector, Jira API credentials, and signed-in Jira browser all unavailable. GitHub CLI is authenticated but is not this repository's tracker.
- **Not done, deliberately:** no GitHub issue, no `docs/plans/` or other local plan file, no implementation change. Switching trackers or committing a local fallback needs your approval.

## Paste-ready Jira Story — project `PAY`

**Issue type:** Story
**Summary:** Retry transient billing provider failures without retrying declined payments

**Description:**

```markdown
## Outcome
Transient billing provider failures are retried automatically; declined
payments are never retried.

## Context
Agreed in session on 2026-08-17. Transient provider failures and declines are
currently handled on the same path, so retry behavior cannot distinguish a
provider outage from a customer-side decline.

## Decisions and constraints
- Transient failures and declines are separate classes; only transient failures retry.
- Retry cap is two attempts after the initial attempt.
- Idempotency keys are preserved across retries of the same payment.

## Acceptance criteria
- [ ] Transient provider failures are classified distinctly from declined payments.
- [ ] A declined payment is never retried.
- [ ] Retries for a transient failure stop at two.
- [ ] The idempotency key is unchanged across every retry of one payment.
- [ ] Provider-contract coverage exercises the transient and declined paths.

## Non-goals
- No change to decline handling, messaging, or dunning.
- No change to the retry schedule or backoff policy beyond the two-attempt cap.

## Verification
- Provider-contract tests covering transient-failure retry and decline no-retry.
- Test asserting the idempotency key is stable across retries.
- Repository test and lint gates pass.

## Risks and stop conditions
- Misclassifying a decline as transient would double-charge or spam the
  provider. Stop and ask if the provider's error taxonomy does not cleanly
  separate the two classes.
- Stop if preserving idempotency keys conflicts with the existing payment
  record model rather than working around it.
```

## Next action

Grant a Jira write path — connector, API credentials, or a signed-in browser session — or paste the Story above into `PAY` yourself and send me the issue key so I can attach follow-up work to it.
