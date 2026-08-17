#!/usr/bin/env node
/**
 * `pnpm doctor` — read-only "is this workspace worth driving?" check.
 *
 * Never installs, repairs, or mutates state. Run it before driving the workspace
 * and again after anything surprising. It names the missing capability and its
 * owner; bootstrap is what fixes things.
 */

import { accessSync, constants, existsSync, readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { dirname, join } from 'node:path';

import { EXIT, FAILURE_CLASSES, LifecycleError, printEvent } from './failure.mjs';
import { IDENTITY_VARS, parseRunnerContract } from './contract.mjs';
import { artifactPaths, writeJson } from './artifacts.mjs';
import { readLedger, readSnapshot, releasable } from './resources.mjs';

const repoRoot = process.cwd();

function check(name, fn) {
  try {
    const detail = fn();
    return { check: name, ok: true, ...detail };
  } catch (error) {
    const failure = error instanceof LifecycleError ? error : null;
    return {
      check: name,
      ok: false,
      failure_class: failure?.failureClass ?? 'internal/unknown',
      owner: failure?.owner ?? 'internal',
      detail: error.message,
      recovery: failure?.recovery ?? FAILURE_CLASSES['internal/unknown'].recovery,
    };
  }
}

function nearestExistingDir(path) {
  let current = path;
  while (!existsSync(current) && dirname(current) !== current) current = dirname(current);
  return current;
}

function toolPresent(command) {
  return spawnSync('/usr/bin/env', ['which', command], { encoding: 'utf8', timeout: 5000 }).status === 0;
}

function main() {
  let contract = null;
  const results = [];

  results.push(
    check('runner_contract', () => {
      contract = parseRunnerContract(process.env);
      return {
        task_id: contract.taskId,
        attempt_id: contract.attemptId,
        result_type: contract.resultType,
        submission_target: contract.submissionTarget,
        merge_authority: contract.mergeAuthority,
      };
    }),
  );

  results.push(
    check('machine_identity', () => {
      const missing = IDENTITY_VARS.filter((name) => !process.env[name]?.trim());
      if (missing.length > 0) {
        throw new LifecycleError('runner/missing_identity', `missing: ${missing.join(', ')}`, { missing });
      }
      return { variables_present: IDENTITY_VARS, values_read: false };
    }),
  );

  results.push(
    check('toolchain', () => {
      const missing = ['git', 'corepack', 'pnpm'].filter((command) => !toolPresent(command));
      if (missing.length > 0) {
        throw new LifecycleError('runner/toolchain_missing', `missing: ${missing.join(', ')}`, { missing });
      }
      const pkg = JSON.parse(readFileSync(join(repoRoot, 'package.json'), 'utf8'));
      const active = spawnSync('pnpm', ['--version'], { encoding: 'utf8', timeout: 60_000 });
      const activePin = active.status === 0 ? `pnpm@${active.stdout.trim()}` : null;
      if (activePin !== pkg.packageManager) {
        throw new LifecycleError('repo/package_manager_mismatch', 'run bootstrap to activate the pinned pnpm', {
          declared: pkg.packageManager ?? null,
          active: activePin,
        });
      }
      return { node: process.version, package_manager: activePin };
    }),
  );

  results.push(
    check('dependencies_installed', () => {
      const pkg = JSON.parse(readFileSync(join(repoRoot, 'package.json'), 'utf8'));
      const declared =
        Object.keys(pkg.dependencies ?? {}).length + Object.keys(pkg.devDependencies ?? {}).length;
      if (declared === 0) return { declared_dependencies: 0, node_modules: false };
      if (!existsSync(join(repoRoot, 'node_modules'))) {
        throw new LifecycleError('repo/dependency_install_failed', 'node_modules is absent; run bootstrap', {
          declared_dependencies: declared,
        });
      }
      return { declared_dependencies: declared, node_modules: true };
    }),
  );

  results.push(
    check('revision_under_test', () => {
      const head = spawnSync('git', ['rev-parse', 'HEAD'], { cwd: repoRoot, encoding: 'utf8', timeout: 15_000 });
      if (head.status !== 0) {
        throw new LifecycleError('runner/workspace_not_writable', 'the workspace is not a resolvable git checkout', {});
      }
      if (contract?.resultType === 'qa') {
        const target = spawnSync('git', ['cat-file', '-e', `${contract.targetRevision}^{commit}`], {
          cwd: repoRoot,
          encoding: 'utf8',
          timeout: 15_000,
        });
        if (target.status !== 0) {
          throw new LifecycleError(
            'runner/missing_target_revision',
            'AGENT_TARGET_REVISION is not present in this checkout',
            { target_revision: contract.targetRevision },
          );
        }
      }
      return { head: head.stdout.trim(), target_revision: contract?.targetRevision ?? null };
    }),
  );

  results.push(
    check('artifact_workspace', () => {
      if (!contract) throw new LifecycleError('runner/missing_task_identity', 'no contract to resolve paths from', {});
      const paths = artifactPaths(contract, repoRoot);
      const anchor = nearestExistingDir(paths.root);
      try {
        accessSync(anchor, constants.W_OK);
      } catch {
        throw new LifecycleError('runner/workspace_not_writable', `${anchor} is not writable`, {});
      }
      return { artifact_dir: contract.artifactDir, writable_anchor: anchor };
    }),
  );

  results.push(
    check('bootstrapped_for_this_attempt', () => {
      if (!contract) throw new LifecycleError('runner/missing_task_identity', 'no contract', {});
      const paths = artifactPaths(contract, repoRoot);
      if (!existsSync(paths.bootstrap)) {
        throw new LifecycleError('repo/contract_violation', 'bootstrap has not run for this attempt', {
          expected: paths.bootstrap,
        });
      }
      const record = JSON.parse(readFileSync(paths.bootstrap, 'utf8'));
      if (record.task_id !== contract.taskId || record.attempt_id !== contract.attemptId) {
        throw new LifecycleError('repo/contract_violation', 'the bootstrap record belongs to another attempt', {
          recorded: `${record.task_id}/${record.attempt_id}`,
        });
      }
      return { bootstrapped_at: record.finished_at, install_performed: record.install?.performed ?? null };
    }),
  );

  results.push(
    check('no_leaked_owned_resources', () => {
      if (!contract) throw new LifecycleError('runner/missing_task_identity', 'no contract', {});
      const paths = artifactPaths(contract, repoRoot);
      const outstanding = releasable(readLedger(paths.ledger), {
        preexisting: readSnapshot(paths.snapshot).items,
      });
      if (outstanding.length > 0) {
        throw new LifecycleError(
          'repo/teardown_failed',
          `${outstanding.length} owned resource(s) from an earlier run are still held; run pnpm teardown`,
          { outstanding: outstanding.map((entry) => `${entry.kind}:${entry.id}`) },
        );
      }
      return { outstanding: 0 };
    }),
  );

  const failed = results.filter((result) => !result.ok);
  const report = {
    stage: 'doctor',
    checked_at: new Date().toISOString(),
    outcome: failed.length === 0 ? 'healthy' : 'unhealthy',
    checks: results,
    first_gap: failed[0] ? { check: failed[0].check, failure_class: failed[0].failure_class } : null,
  };

  if (contract) {
    const paths = artifactPaths(contract, repoRoot);
    if (existsSync(paths.root)) writeJson(paths.doctor, report);
  }
  process.stdout.write(`${JSON.stringify(report, null, 2)}\n`);
  printEvent({
    event: 'lifecycle.doctor',
    operation: 'doctor',
    resource_id: contract ? `${contract.taskId}/${contract.attemptId}` : 'unknown',
    outcome: report.outcome,
    error_kind: report.first_gap?.failure_class ?? null,
  });

  if (failed.length === 0) return EXIT.ok;
  return failed.some((result) => result.owner === 'runner') ? EXIT.runner : EXIT.repository;
}

process.exit(main());
