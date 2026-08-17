#!/usr/bin/env node
/**
 * `pnpm teardown` — idempotent release of resources this attempt raised.
 *
 * Safe on success, failure, cancellation, timeout, and retry. It releases only
 * ledger entries recorded by the attempt, preserves everything in the
 * pre-existing snapshot, verifies absence rather than trusting exit status, and
 * never deletes captured evidence.
 *
 * Also importable: verify.mjs calls runTeardown() on every exit path.
 */

import { existsSync, readFileSync, realpathSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

import { EXIT, LifecycleError, printEvent, printFailure } from './failure.mjs';
import { parseRunnerContract } from './contract.mjs';
import { artifactPaths, writeJson } from './artifacts.mjs';
import { readLedger, readSnapshot, recordResource, releasable, releaseEntry, verifyPreservation } from './resources.mjs';

export const TEARDOWN_REASONS = ['success', 'failure', 'cancelled', 'timeout', 'retry'];

/**
 * @param {object} options
 * @param {'bootstrap'|'verify'|'attempt'} options.scope which stage's resources to release
 * @param {string} options.reason why teardown ran, for the record
 * @param {number} options.primaryStatus exit status of the work that preceded teardown
 */
export function runTeardown({ contract, paths, scope = 'attempt', reason = 'success', primaryStatus = 0 }) {
  const startedAt = new Date().toISOString();
  const snapshot = readSnapshot(paths.snapshot);
  const outstanding = releasable(readLedger(paths.ledger), { scope, preexisting: snapshot.items });

  const releases = outstanding.map((entry) => releaseEntry(entry));
  for (const [index, result] of releases.entries()) {
    if (result.absent) {
      recordResource(paths.ledger, { ...outstanding[index], released_at: result.released_at });
    }
  }

  const survivors = releases.filter((result) => !result.absent);
  const preservationViolations = verifyPreservation(snapshot);
  const clean = survivors.length === 0 && preservationViolations.length === 0;

  const history = existsSync(paths.teardown) ? JSON.parse(readFileSync(paths.teardown, 'utf8')).runs ?? [] : [];
  const record = {
    stage: 'teardown',
    task_id: contract.taskId,
    attempt_id: contract.attemptId,
    runs: [
      ...history,
      {
        scope,
        reason,
        primary_status: primaryStatus,
        started_at: startedAt,
        finished_at: new Date().toISOString(),
        outcome: clean ? 'clean' : 'incomplete',
        released: releases.filter((result) => result.absent).map(({ kind, id, label, note }) => ({ kind, id, label, note })),
        survivors,
        preserved_pre_existing: snapshot.items.length,
        preservation_violations: preservationViolations,
      },
    ],
  };
  writeJson(paths.teardown, record);

  const failureClass = clean ? null : 'repo/teardown_failed';
  printEvent({
    event: 'lifecycle.teardown',
    operation: 'teardown',
    resource_id: `${contract.taskId}/${contract.attemptId}`,
    outcome: clean ? 'clean' : 'incomplete',
    error_kind: failureClass,
    scope,
    reason,
    released: releases.filter((result) => result.absent).length,
    survivors: survivors.length,
  });

  // Preserve a primary failure or signal status; report cleanup separately.
  // If the primary work succeeded, an incomplete cleanup makes the command fail.
  let exitCode = primaryStatus;
  if (!clean && primaryStatus === 0) exitCode = EXIT.teardown;

  return { clean, exitCode, record, survivors, preservationViolations };
}

function parseArgs(argv) {
  const options = { scope: 'attempt', reason: 'success', primaryStatus: 0 };
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    const split = token.indexOf('=');
    const flag = split === -1 ? token : token.slice(0, split);
    const inline = split === -1 ? undefined : token.slice(split + 1);
    const value = inline ?? argv[index + 1];
    if (inline === undefined && value !== undefined) index += 1;
    if (flag === '--scope') options.scope = value;
    else if (flag === '--reason') options.reason = value;
    else if (flag === '--primary-status') options.primaryStatus = Number(value);
    else throw new LifecycleError('repo/contract_violation', `unknown teardown flag: ${flag}`, {});
  }
  if (!TEARDOWN_REASONS.includes(options.reason)) {
    throw new LifecycleError('repo/contract_violation', `--reason must be one of ${TEARDOWN_REASONS.join(', ')}`, {
      received: options.reason,
    });
  }
  if (!Number.isInteger(options.primaryStatus) || options.primaryStatus < 0) {
    throw new LifecycleError('repo/contract_violation', '--primary-status must be a non-negative integer', {});
  }
  return options;
}

function main() {
  const options = parseArgs(process.argv.slice(2));
  const contract = parseRunnerContract(process.env);
  const paths = artifactPaths(contract, process.cwd());
  const result = runTeardown({ contract, paths, ...options });
  if (!result.clean) {
    printFailure(
      'teardown',
      new LifecycleError('repo/teardown_failed', 'owned resources survived teardown or a preserved resource vanished', {
        survivors: result.survivors.map((item) => `${item.kind}:${item.id}`),
        preservation_violations: result.preservationViolations.map((item) => `${item.kind}:${item.id}`),
        record: paths.teardown,
      }),
    );
  }
  return result.exitCode;
}

// Only run as a CLI, so verify.mjs and the tests can import runTeardown without
// side effects.
function invokedDirectly() {
  if (!process.argv[1]) return false;
  try {
    return realpathSync(process.argv[1]) === realpathSync(fileURLToPath(import.meta.url));
  } catch {
    return false;
  }
}

if (invokedDirectly()) {
  try {
    process.exit(main());
  } catch (error) {
    const failure = printFailure('teardown', error);
    process.exit(failure.exitCode);
  }
}
