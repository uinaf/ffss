/**
 * Stable failure taxonomy shared by bootstrap, doctor, verify, and teardown.
 * The prefix names the owner, so an orchestrator can route without parsing prose.
 */

export const OWNER_RUNNER = 'runner';
export const OWNER_REPO = 'repo';
export const OWNER_INTERNAL = 'internal';

/** failure class -> { owner, retryable, recovery } */
export const FAILURE_CLASSES = {
  'runner/missing_task_identity': {
    owner: OWNER_RUNNER,
    retryable: false,
    recovery: 'Export AGENT_TASK_ID and AGENT_ATTEMPT_ID before dispatching the attempt.',
  },
  'runner/missing_identity': {
    owner: OWNER_RUNNER,
    retryable: false,
    recovery: 'Attach the scoped machine identity so INFISICAL_TOKEN, INFISICAL_API_URL, and INFISICAL_PROJECT_ID are exported.',
  },
  'runner/invalid_identity': {
    owner: OWNER_RUNNER,
    retryable: false,
    recovery: 'Point INFISICAL_API_URL at an https endpoint reachable from the workspace.',
  },
  'runner/invalid_task_input': {
    owner: OWNER_RUNNER,
    retryable: false,
    recovery: 'Correct the malformed task variable named in the context; see docs/automation.md section 1.',
  },
  'runner/invalid_result_type': {
    owner: OWNER_RUNNER,
    retryable: false,
    recovery: 'Set AGENT_RESULT_TYPE to implementation or qa during triage.',
  },
  'runner/invalid_submission_target': {
    owner: OWNER_RUNNER,
    retryable: false,
    recovery: 'Dispatch a submission target the result type allows; qa tasks submit a qa_report, never a pull_request.',
  },
  'runner/invalid_authority': {
    owner: OWNER_RUNNER,
    retryable: false,
    recovery: 'Grant merge authority only on implementation tasks, otherwise set AGENT_MERGE_AUTHORITY=none.',
  },
  'runner/missing_target_revision': {
    owner: OWNER_RUNNER,
    retryable: false,
    recovery: 'A qa task must declare AGENT_TARGET_REVISION (and AGENT_TARGET_BUILD when a prebuilt artifact is tested).',
  },
  'runner/toolchain_missing': {
    owner: OWNER_RUNNER,
    retryable: false,
    recovery: 'Provision the declared base toolchain on the runner image, then re-dispatch.',
  },
  'runner/workspace_not_writable': {
    owner: OWNER_RUNNER,
    retryable: true,
    recovery: 'Allocate a writable isolated workspace and artifact volume for the attempt.',
  },
  'runner/network_denied': {
    owner: OWNER_RUNNER,
    retryable: true,
    recovery: 'Allow the declared registry and secret-manager destinations in the egress policy.',
  },
  'repo/contract_violation': {
    owner: OWNER_REPO,
    retryable: false,
    recovery: 'Fix the repository lifecycle contract; the declared inputs and the code disagree.',
  },
  'repo/package_manager_mismatch': {
    owner: OWNER_REPO,
    retryable: false,
    recovery: 'Reconcile the packageManager pin in package.json with the activated pnpm version.',
  },
  'repo/missing_lockfile': {
    owner: OWNER_REPO,
    retryable: false,
    recovery: 'Commit pnpm-lock.yaml so installs are reproducible, or remove the declared dependencies.',
  },
  'repo/dependency_install_failed': {
    owner: OWNER_REPO,
    retryable: true,
    recovery: 'Inspect the install log in the attempt artifact directory; a drifted lockfile needs a repository change.',
  },
  'repo/verification_failed': {
    owner: OWNER_REPO,
    retryable: false,
    recovery: 'Read logs/verify.log in the attempt artifact directory and fix the failing behaviour.',
  },
  'repo/verification_timeout': {
    owner: OWNER_REPO,
    retryable: true,
    recovery: 'Verification exceeded its bound; shorten the gate or raise AGENT_VERIFY_TIMEOUT_SECONDS deliberately.',
  },
  'repo/evidence_missing': {
    owner: OWNER_REPO,
    retryable: false,
    recovery: 'Capture the declared qa evidence and describe it in scenarios.json before submitting a report.',
  },
  'repo/evidence_invalid': {
    owner: OWNER_REPO,
    retryable: false,
    recovery: 'Correct scenarios.json against the schema in docs/automation.md.',
  },
  'repo/secret_exposure': {
    owner: OWNER_REPO,
    retryable: false,
    recovery: 'Redact the captured evidence; an injected secret value reached an artifact. Values are never printed.',
  },
  'repo/teardown_failed': {
    owner: OWNER_REPO,
    retryable: true,
    recovery: 'An owned runtime resource survived teardown; inspect teardown.json and release it before reuse.',
  },
  'repo/unreleasable_resource': {
    owner: OWNER_REPO,
    retryable: false,
    recovery: 'Record release and verify argv on ledger kinds this repository has no built-in handler for.',
  },
  'internal/unknown': {
    owner: OWNER_INTERNAL,
    retryable: true,
    recovery: 'Unclassified failure; inspect the attempt artifact directory.',
  },
};

/** Process exit codes, stable across all four entrypoints. */
export const EXIT = {
  ok: 0,
  repository: 1,
  runner: 2,
  /** ordinary work succeeded but cleanup or absence verification did not */
  teardown: 3,
};

export class LifecycleError extends Error {
  constructor(failureClass, message, { context = {}, cause } = {}) {
    super(message, { cause });
    this.name = 'LifecycleError';
    this.failureClass = FAILURE_CLASSES[failureClass] ? failureClass : 'internal/unknown';
    this.context = context;
  }

  get owner() {
    return FAILURE_CLASSES[this.failureClass].owner;
  }

  get retryable() {
    return FAILURE_CLASSES[this.failureClass].retryable;
  }

  get recovery() {
    return FAILURE_CLASSES[this.failureClass].recovery;
  }

  get exitCode() {
    if (this.failureClass === 'repo/teardown_failed') return EXIT.teardown;
    return this.owner === OWNER_RUNNER ? EXIT.runner : EXIT.repository;
  }

  toJSON() {
    return {
      failure_class: this.failureClass,
      owner: this.owner,
      retryable: this.retryable,
      message: this.message,
      recovery: this.recovery,
      context: this.context,
    };
  }
}

export function classify(error) {
  if (error instanceof LifecycleError) return error;
  return new LifecycleError('internal/unknown', error?.message ?? String(error), { cause: error });
}

/**
 * Writes one actionable block to stderr. Context values must already be
 * non-secret; callers pass names and counts, never injected values.
 */
export function printFailure(stage, error) {
  const failure = classify(error);
  const lines = [
    `${stage} failed`,
    `  failure_class: ${failure.failureClass}`,
    `  owner:         ${failure.owner}`,
    `  retryable:     ${failure.retryable}`,
    `  detail:        ${failure.message}`,
    `  recovery:      ${failure.recovery}`,
  ];
  for (const [key, value] of Object.entries(failure.context)) {
    lines.push(`  ${key}: ${typeof value === 'object' ? JSON.stringify(value) : value}`);
  }
  process.stderr.write(`${lines.join('\n')}\n`);
  return failure;
}

/** Single structured line so a log collector can route without parsing prose. */
export function printEvent(event) {
  process.stdout.write(`${JSON.stringify({ ...event, ts: new Date().toISOString() })}\n`);
}
