/**
 * Runner contract: the environment an orchestrator must supply, parsed once into
 * a validated shape the lifecycle entrypoints use internally.
 *
 * Nothing here reads a secret's value. Identity variables are checked for
 * presence and shape only.
 */

import { LifecycleError } from './failure.mjs';

export const RESULT_TYPES = ['implementation', 'qa'];

/** Submission targets each result type is allowed to reach. */
export const ALLOWED_TARGETS = {
  implementation: ['pull_request', 'none'],
  qa: ['qa_report', 'none'],
};

export const DEFAULT_TARGET = {
  implementation: 'pull_request',
  qa: 'qa_report',
};

export const MERGE_AUTHORITIES = ['none', 'merge'];

/** Injected by the runner's machine identity. Presence is required, values are never read. */
export const IDENTITY_VARS = ['INFISICAL_TOKEN', 'INFISICAL_API_URL', 'INFISICAL_PROJECT_ID'];

/** Non-secret of the three above; safe to echo. */
export const NON_SECRET_IDENTITY_VARS = ['INFISICAL_API_URL', 'INFISICAL_PROJECT_ID'];

/** Path-safe identifiers: keeps artifact directories inside the workspace. */
const IDENTIFIER = /^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$/;

const DEFAULTS = {
  verifyTimeoutSeconds: 900,
  retryBudget: 2,
  baseRef: 'main',
  artifactRoot: 'artifacts',
};

function required(env, name, failureClass) {
  const value = env[name];
  if (typeof value !== 'string' || value.trim() === '') {
    throw new LifecycleError(failureClass, `${name} is not set`, { missing: name });
  }
  return value.trim();
}

function identifier(env, name, failureClass) {
  const value = required(env, name, failureClass);
  if (!IDENTIFIER.test(value)) {
    throw new LifecycleError(
      failureClass,
      `${name} must match ${IDENTIFIER.source} so artifact paths stay inside the workspace`,
      { variable: name, length: value.length },
    );
  }
  return value;
}

function positiveInteger(env, name, fallback) {
  const raw = env[name];
  if (raw === undefined || raw === '') return fallback;
  const parsed = Number(raw);
  if (!Number.isInteger(parsed) || parsed < 0) {
    throw new LifecycleError('runner/invalid_task_input', `${name} must be a non-negative integer`, {
      variable: name,
    });
  }
  return parsed;
}

/**
 * @param {Record<string, string | undefined>} env
 * @returns {{
 *   taskId: string, attemptId: string, attemptNumber: number, retryBudget: number,
 *   resultType: 'implementation'|'qa', submissionTarget: string, mergeAuthority: string,
 *   baseRef: string|null, targetRevision: string|null, targetBuild: string|null,
 *   environmentName: string, runner: string, verifyTimeoutSeconds: number,
 *   artifactDir: string, identity: { apiUrl: string, projectId: string, tokenPresent: true }
 * }}
 */
export function parseRunnerContract(env) {
  const taskId = identifier(env, 'AGENT_TASK_ID', 'runner/missing_task_identity');
  const attemptId = identifier(env, 'AGENT_ATTEMPT_ID', 'runner/missing_task_identity');

  const resultType = required(env, 'AGENT_RESULT_TYPE', 'runner/invalid_result_type');
  if (!RESULT_TYPES.includes(resultType)) {
    throw new LifecycleError(
      'runner/invalid_result_type',
      `AGENT_RESULT_TYPE must be one of ${RESULT_TYPES.join(', ')}`,
      { received: resultType },
    );
  }

  const submissionTarget = (env.AGENT_SUBMISSION_TARGET ?? DEFAULT_TARGET[resultType]).trim();
  if (!ALLOWED_TARGETS[resultType].includes(submissionTarget)) {
    throw new LifecycleError(
      'runner/invalid_submission_target',
      `result type ${resultType} may only submit to ${ALLOWED_TARGETS[resultType].join(' or ')}`,
      { result_type: resultType, requested_target: submissionTarget },
    );
  }

  const mergeAuthority = (env.AGENT_MERGE_AUTHORITY ?? 'none').trim();
  if (!MERGE_AUTHORITIES.includes(mergeAuthority)) {
    throw new LifecycleError(
      'runner/invalid_authority',
      `AGENT_MERGE_AUTHORITY must be one of ${MERGE_AUTHORITIES.join(', ')}`,
      { received: mergeAuthority },
    );
  }
  if (mergeAuthority === 'merge' && resultType !== 'implementation') {
    throw new LifecycleError(
      'runner/invalid_authority',
      'merge authority is only grantable on implementation tasks',
      { result_type: resultType },
    );
  }

  for (const name of IDENTITY_VARS) {
    required(env, name, 'runner/missing_identity');
  }
  const apiUrl = env.INFISICAL_API_URL.trim();
  let parsedUrl;
  try {
    parsedUrl = new URL(apiUrl);
  } catch {
    throw new LifecycleError('runner/invalid_identity', 'INFISICAL_API_URL is not a URL', {
      variable: 'INFISICAL_API_URL',
    });
  }
  if (parsedUrl.protocol !== 'https:') {
    throw new LifecycleError('runner/invalid_identity', 'INFISICAL_API_URL must use https', {
      protocol: parsedUrl.protocol,
    });
  }

  let targetRevision = env.AGENT_TARGET_REVISION?.trim() || null;
  if (resultType === 'qa' && !targetRevision) {
    throw new LifecycleError(
      'runner/missing_target_revision',
      'a qa task must declare the revision under test',
      { variable: 'AGENT_TARGET_REVISION' },
    );
  }

  const artifactRoot = env.AGENT_ARTIFACT_ROOT?.trim() || DEFAULTS.artifactRoot;

  return {
    taskId,
    attemptId,
    attemptNumber: positiveInteger(env, 'AGENT_ATTEMPT_NUMBER', 1) || 1,
    retryBudget: positiveInteger(env, 'AGENT_RETRY_BUDGET', DEFAULTS.retryBudget),
    resultType,
    submissionTarget,
    mergeAuthority,
    baseRef: resultType === 'implementation' ? env.AGENT_BASE_REF?.trim() || DEFAULTS.baseRef : null,
    targetRevision,
    targetBuild: env.AGENT_TARGET_BUILD?.trim() || null,
    environmentName: env.AGENT_ENVIRONMENT?.trim() || 'devbox',
    runner: env.AGENT_RUNNER?.trim() || 'unknown',
    verifyTimeoutSeconds:
      positiveInteger(env, 'AGENT_VERIFY_TIMEOUT_SECONDS', DEFAULTS.verifyTimeoutSeconds) ||
      DEFAULTS.verifyTimeoutSeconds,
    artifactDir: `${artifactRoot}/${taskId}/${attemptId}`,
    identity: {
      apiUrl,
      projectId: env.INFISICAL_PROJECT_ID.trim(),
      tokenPresent: true,
    },
  };
}

/**
 * Branch name an implementation attempt is allowed to push. QA attempts get
 * null: they must not manufacture a ref.
 */
export function allowedBranch(contract) {
  if (contract.resultType !== 'implementation') return null;
  return `agent/${contract.taskId}/${contract.attemptId}`;
}

/**
 * Literal values that must never appear in a preserved artifact. Collected by
 * name pattern so an added secret is covered without editing this list.
 */
export function secretValues(env) {
  const pattern = /(TOKEN|SECRET|PASSWORD|PASSPHRASE|CREDENTIAL|PRIVATE_KEY|_KEY$|API_KEY)/i;
  const values = new Set();
  for (const [name, value] of Object.entries(env)) {
    if (typeof value !== 'string' || value.length < 8) continue;
    if (NON_SECRET_IDENTITY_VARS.includes(name)) continue;
    if (pattern.test(name)) values.add(value);
  }
  return [...values];
}
