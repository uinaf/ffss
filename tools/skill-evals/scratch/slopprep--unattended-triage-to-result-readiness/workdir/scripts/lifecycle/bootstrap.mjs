#!/usr/bin/env node
/**
 * `pnpm bootstrap` — noninteractive cold start.
 *
 * Validates the runner contract and machine identity, activates the pinned
 * package manager, installs reproducibly, and reports whether a failure belongs
 * to the repository or the runner. Never prompts, never writes a secret to disk,
 * never prints a secret value.
 */

import { existsSync, readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';
import { join } from 'node:path';

import { EXIT, LifecycleError, printEvent, printFailure } from './failure.mjs';
import { IDENTITY_VARS, parseRunnerContract } from './contract.mjs';
import { artifactPaths, ensureArtifactDirs, describeEnvironment, writeJson } from './artifacts.mjs';

const repoRoot = process.cwd();
const REQUIRED_NODE_MAJOR = 22;

function which(command) {
  return spawnSync('/usr/bin/env', ['which', command], { encoding: 'utf8', timeout: 5000 }).status === 0;
}

function requireToolchain() {
  const nodeMajor = Number(process.versions.node.split('.')[0]);
  if (nodeMajor < REQUIRED_NODE_MAJOR) {
    throw new LifecycleError('runner/toolchain_missing', `node >= ${REQUIRED_NODE_MAJOR} is required`, {
      node: process.version,
    });
  }
  for (const command of ['git', 'corepack']) {
    if (!which(command)) {
      throw new LifecycleError('runner/toolchain_missing', `${command} is not on PATH`, { command });
    }
  }
  return { node: process.version, git: true, corepack: true };
}

function activatePackageManager() {
  const pkg = JSON.parse(readFileSync(join(repoRoot, 'package.json'), 'utf8'));
  const pin = pkg.packageManager;
  if (!pin || !pin.startsWith('pnpm@')) {
    throw new LifecycleError('repo/contract_violation', 'package.json must pin packageManager to a pnpm version', {
      package_manager: pin ?? null,
    });
  }
  const prepared = spawnSync('corepack', ['prepare', pin, '--activate'], {
    cwd: repoRoot,
    encoding: 'utf8',
    timeout: 300_000,
  });
  if (prepared.status !== 0) {
    const stderr = prepared.stderr ?? '';
    const offline = /ENOTFOUND|EAI_AGAIN|ETIMEDOUT|network|ECONNREFUSED/i.test(stderr);
    throw new LifecycleError(
      offline ? 'runner/network_denied' : 'runner/toolchain_missing',
      `corepack could not activate ${pin}`,
      { exit_code: prepared.status },
    );
  }
  const active = spawnSync('pnpm', ['--version'], { cwd: repoRoot, encoding: 'utf8', timeout: 60_000 });
  if (active.status !== 0) {
    throw new LifecycleError('runner/toolchain_missing', 'pnpm is not runnable after corepack activation', {
      exit_code: active.status,
    });
  }
  const expected = pin.slice('pnpm@'.length);
  const actual = active.stdout.trim();
  if (actual !== expected) {
    throw new LifecycleError('repo/package_manager_mismatch', 'active pnpm does not match the declared pin', {
      declared: expected,
      active: actual,
    });
  }
  return { declared: pin, active: `pnpm@${actual}` };
}

function installDependencies(logPath) {
  const pkg = JSON.parse(readFileSync(join(repoRoot, 'package.json'), 'utf8'));
  const declared =
    Object.keys(pkg.dependencies ?? {}).length +
    Object.keys(pkg.devDependencies ?? {}).length +
    Object.keys(pkg.optionalDependencies ?? {}).length;

  if (declared === 0) {
    return { performed: false, reason: 'no declared dependencies', declared_dependencies: 0 };
  }
  if (!existsSync(join(repoRoot, 'pnpm-lock.yaml'))) {
    throw new LifecycleError('repo/missing_lockfile', 'declared dependencies require a committed pnpm-lock.yaml', {
      declared_dependencies: declared,
    });
  }
  const installed = spawnSync('pnpm', ['install', '--frozen-lockfile'], {
    cwd: repoRoot,
    encoding: 'utf8',
    timeout: 900_000,
    env: { ...process.env, CI: 'true' },
  });
  writeJson(logPath, {
    command: 'pnpm install --frozen-lockfile',
    exit_code: installed.status,
    stdout_tail: (installed.stdout ?? '').split('\n').slice(-40).join('\n'),
    stderr_tail: (installed.stderr ?? '').split('\n').slice(-40).join('\n'),
  });
  if (installed.status !== 0) {
    const stderr = installed.stderr ?? '';
    if (/ERR_PNPM_OUTDATED_LOCKFILE/.test(stderr)) {
      throw new LifecycleError('repo/missing_lockfile', 'pnpm-lock.yaml is out of date with package.json', {
        declared_dependencies: declared,
      });
    }
    if (/ENOTFOUND|EAI_AGAIN|ETIMEDOUT|ECONNREFUSED|registry/i.test(stderr)) {
      throw new LifecycleError('runner/network_denied', 'the package registry was unreachable', {
        exit_code: installed.status,
      });
    }
    throw new LifecycleError('repo/dependency_install_failed', 'pnpm install failed', {
      exit_code: installed.status,
      log: logPath,
    });
  }
  return { performed: true, declared_dependencies: declared, frozen_lockfile: true };
}

/**
 * Optional liveness check for the injected identity. Off by default so bootstrap
 * stays offline-safe; enable with AGENT_IDENTITY_PROBE=1 when egress is allowed.
 */
async function probeIdentity(contract) {
  if (process.env.AGENT_IDENTITY_PROBE !== '1') {
    return { probed: false, reason: 'AGENT_IDENTITY_PROBE is not 1' };
  }
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), 10_000);
  try {
    const response = await fetch(new URL('/api/status', contract.identity.apiUrl), {
      method: 'GET',
      headers: { authorization: `Bearer ${process.env.INFISICAL_TOKEN}` },
      signal: controller.signal,
    });
    if (response.status === 401 || response.status === 403) {
      throw new LifecycleError('runner/missing_identity', 'the injected identity was rejected by the secret manager', {
        http_status: response.status,
      });
    }
    return { probed: true, http_status: response.status };
  } catch (error) {
    if (error instanceof LifecycleError) throw error;
    throw new LifecycleError('runner/network_denied', 'the secret manager endpoint was unreachable', {
      code: error.name,
    });
  } finally {
    clearTimeout(timer);
  }
}

async function main() {
  const startedAt = new Date().toISOString();
  const contract = parseRunnerContract(process.env);
  const paths = ensureArtifactDirs(artifactPaths(contract, repoRoot));

  const toolchain = requireToolchain();
  const packageManager = activatePackageManager();
  const install = installDependencies(join(paths.logs, 'install.json'));
  const identity = await probeIdentity(contract);

  const record = {
    stage: 'bootstrap',
    outcome: 'ready',
    task_id: contract.taskId,
    attempt_id: contract.attemptId,
    attempt_number: contract.attemptNumber,
    result_type: contract.resultType,
    submission_target: contract.submissionTarget,
    merge_authority: contract.mergeAuthority,
    started_at: startedAt,
    finished_at: new Date().toISOString(),
    toolchain,
    package_manager: packageManager,
    install,
    identity: {
      variables_present: IDENTITY_VARS,
      project_id: contract.identity.projectId,
      api_url: contract.identity.apiUrl,
      token_present: true,
      probe: identity,
    },
    environment: describeEnvironment(contract, repoRoot),
  };
  writeJson(paths.bootstrap, record);
  printEvent({
    event: 'lifecycle.bootstrap',
    operation: 'bootstrap',
    resource_id: `${contract.taskId}/${contract.attemptId}`,
    outcome: 'ready',
    error_kind: null,
    install_performed: install.performed,
  });
  return EXIT.ok;
}

main()
  .then((code) => process.exit(code))
  .catch((error) => {
    const failure = printFailure('bootstrap', error);
    printEvent({
      event: 'lifecycle.bootstrap',
      operation: 'bootstrap',
      resource_id: `${process.env.AGENT_TASK_ID ?? 'unknown'}/${process.env.AGENT_ATTEMPT_ID ?? 'unknown'}`,
      outcome: 'failed',
      error_kind: failure.failureClass,
      retryable: failure.retryable,
    });
    process.exit(failure.exitCode);
  });
