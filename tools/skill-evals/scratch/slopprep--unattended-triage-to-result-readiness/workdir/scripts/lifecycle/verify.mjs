#!/usr/bin/env node
/**
 * `pnpm verify` — the canonical bounded gate, reused by humans, agents, and CI.
 *
 * It still runs the repository's own test gate (`pnpm test`). On top of that it
 * records inspectable outcome evidence and a machine-readable manifest under
 * artifacts/<task>/<attempt>/, scans preserved text evidence for injected secret
 * values, and releases the process tree it raised on every exit path.
 *
 * Result paths:
 *  - implementation: the repository gate is mandatory; failure exits non-zero.
 *  - qa: scenarios.json plus its referenced evidence is mandatory; the repository
 *    gate is optional because the revision or build under test may be prebuilt.
 *    A failing scenario is a finding, not a broken attempt, so it does not fail
 *    the command unless AGENT_QA_FAIL_ON_SCENARIO_FAILURE=1.
 */

import { createWriteStream, existsSync } from 'node:fs';
import { spawn } from 'node:child_process';
import { join, relative } from 'node:path';

import { EXIT, LifecycleError, classify, printEvent, printFailure } from './failure.mjs';
import { allowedBranch, parseRunnerContract, secretValues } from './contract.mjs';
import {
  MANIFEST_SCHEMA,
  REDACTION,
  artifactPaths,
  classifyFormat,
  describeArtifacts,
  describeEnvironment,
  describeRevision,
  ensureArtifactDirs,
  listArtifactFiles,
  readScenarios,
  scanForSecrets,
  writeManifest,
} from './artifacts.mjs';
import { recordResource, snapshotPreexisting, writeSnapshot } from './resources.mjs';
import { runTeardown } from './teardown.mjs';

const repoRoot = process.cwd();
const GATE_COMMAND = ['pnpm', 'test'];

/**
 * Runs the repository gate as an owned process group so a launcher's descendants
 * are released together, bounded by AGENT_VERIFY_TIMEOUT_SECONDS.
 */
function runGate({ contract, paths }) {
  return new Promise((resolve) => {
    const logPath = join(paths.logs, 'verify.log');
    const log = createWriteStream(logPath, { flags: 'w' });
    const startedAt = new Date().toISOString();
    const [command, ...args] = GATE_COMMAND;
    const child = spawn(command, args, { cwd: repoRoot, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });

    if (child.pid) {
      recordResource(paths.ledger, {
        kind: 'process_group',
        id: String(child.pid),
        pgid: child.pid,
        label: GATE_COMMAND.join(' '),
        scope: 'verify',
      });
    }

    let timedOut = false;
    const timer = setTimeout(() => {
      timedOut = true;
      try {
        process.kill(-child.pid, 'SIGTERM');
      } catch {
        /* teardown re-checks and escalates */
      }
    }, contract.verifyTimeoutSeconds * 1000);

    for (const stream of [child.stdout, child.stderr]) {
      stream.on('data', (chunk) => {
        log.write(chunk);
        process.stdout.write(chunk);
      });
    }

    // Resolve only once the log is flushed: the manifest hashes this file.
    child.on('error', (error) => {
      clearTimeout(timer);
      log.end(`\nspawn error: ${error.code ?? error.message}\n`, () =>
        resolve({
          command: GATE_COMMAND.join(' '),
          exitCode: 127,
          signal: null,
          timedOut: false,
          log: relative(paths.root, logPath),
          startedAt,
          finishedAt: new Date().toISOString(),
          spawn_error: error.code ?? error.message,
        }),
      );
    });

    child.on('close', (code, signal) => {
      clearTimeout(timer);
      log.end(() =>
        resolve({
          command: GATE_COMMAND.join(' '),
          exitCode: code === null ? 124 : code,
          signal: signal ?? null,
          timedOut,
          log: relative(paths.root, logPath),
          startedAt,
          finishedAt: new Date().toISOString(),
        }),
      );
    });
  });
}

function summariseScenarios(scenarios) {
  const summary = { passed: 0, failed: 0, blocked: 0, skipped: 0 };
  for (const scenario of scenarios) summary[scenario.result] += 1;
  return summary;
}

function gateScenario(gate) {
  return {
    id: 'repository-gate',
    description: `${gate.command} on the checked-out revision`,
    result: gate.exitCode === 0 ? 'passed' : 'failed',
    producer: gate.command,
    started_at: gate.startedAt,
    finished_at: gate.finishedAt ?? null,
    evidence: [gate.log].filter(Boolean),
  };
}

async function main() {
  const startedAt = new Date().toISOString();
  const contract = parseRunnerContract(process.env);
  const paths = ensureArtifactDirs(artifactPaths(contract, repoRoot));

  if (!existsSync(paths.snapshot)) writeSnapshot(paths.snapshot, snapshotPreexisting());

  // Cleanup is registered before the first failure point below.
  let tornDown = false;
  const cleanup = (reason, primaryStatus) => {
    if (tornDown) return { exitCode: primaryStatus, clean: true };
    tornDown = true;
    return runTeardown({ contract, paths, scope: 'verify', reason, primaryStatus });
  };
  for (const signal of ['SIGINT', 'SIGTERM', 'SIGHUP']) {
    process.on(signal, () => {
      cleanup('cancelled', 130);
      process.exit(130);
    });
  }

  const runGateForQa = contract.resultType === 'implementation' || process.env.AGENT_QA_RUN_REPOSITORY_GATE === '1';
  const gate = runGateForQa ? await runGate({ contract, paths }) : null;

  let failure = null;
  const scenarios = [];
  let scenarioByPath = new Map();
  let declaredScenarios = null;

  if (gate) scenarios.push(gateScenario(gate));

  if (gate && gate.spawn_error === 'ENOENT') {
    failure = new LifecycleError('runner/toolchain_missing', `${gate.command} is not runnable in this workspace`, {
      command: gate.command,
    });
  } else if (gate && gate.timedOut) {
    failure = new LifecycleError('repo/verification_timeout', `the gate exceeded ${contract.verifyTimeoutSeconds}s`, {
      command: gate.command,
      log: gate.log,
    });
  } else if (gate && gate.exitCode !== 0 && contract.resultType === 'implementation') {
    failure = new LifecycleError('repo/verification_failed', `${gate.command} exited ${gate.exitCode}`, {
      exit_code: gate.exitCode,
      log: gate.log,
    });
  }

  if (contract.resultType === 'qa') {
    try {
      const read = readScenarios(paths);
      declaredScenarios = read.declared;
      scenarioByPath = read.scenarioByPath;
      scenarios.push(...read.declared.scenarios);
    } catch (error) {
      failure ??= classify(error);
    }
  }

  const summary = summariseScenarios(scenarios);
  const files = listArtifactFiles(paths);
  const scan = scanForSecrets(paths.root, files, secretValues(process.env));
  const exposed = new Set(scan.findings.map((finding) => finding.path));
  if (scan.findings.length > 0) {
    failure = new LifecycleError('repo/secret_exposure', 'an injected secret value appears in preserved evidence', {
      affected_artifacts: scan.findings.map((finding) => finding.path),
    });
  }

  const qaScenarioFailure =
    contract.resultType === 'qa' &&
    process.env.AGENT_QA_FAIL_ON_SCENARIO_FAILURE === '1' &&
    summary.failed + summary.blocked > 0;
  if (qaScenarioFailure && !failure) {
    failure = new LifecycleError('repo/verification_failed', 'a declared qa scenario failed', {
      failed: summary.failed,
      blocked: summary.blocked,
    });
  }

  const manifest = {
    schema: MANIFEST_SCHEMA,
    task_id: contract.taskId,
    attempt_id: contract.attemptId,
    attempt_number: contract.attemptNumber,
    retry_budget: contract.retryBudget,
    result_type: contract.resultType,
    submission_target: contract.submissionTarget,
    merge_authority: contract.mergeAuthority,
    allowed_branch: allowedBranch(contract),
    revision: describeRevision(repoRoot, contract),
    build: { id: contract.targetBuild, source: contract.targetBuild ? 'runner' : null },
    environment: describeEnvironment(contract, repoRoot),
    started_at: startedAt,
    finished_at: new Date().toISOString(),
    outcome: failure ? 'failed' : 'passed',
    failure_class: failure?.failureClass ?? null,
    retryable: failure ? failure.retryable : false,
    recovery: failure?.recovery ?? null,
    gate,
    scenario_summary: summary,
    scenarios,
    findings_present: summary.failed + summary.blocked > 0,
    qa_report: declaredScenarios
      ? { title: declaredScenarios.title ?? null, notes: declaredScenarios.notes ?? null }
      : null,
    redaction: { ...scan, status: scan.findings.length === 0 ? 'clean' : 'secret-detected' },
    artifacts: describeArtifacts(paths, {
      producer: 'pnpm verify',
      scenarioByPath,
      redactionFor: (rel) =>
        exposed.has(rel) ? REDACTION.exposed : classifyFormat(rel).text ? REDACTION.clean : REDACTION.unscanned,
    }),
  };
  writeManifest(paths, manifest);

  const primaryStatus = failure ? failure.exitCode : EXIT.ok;
  printEvent({
    event: 'lifecycle.verify',
    operation: 'verify',
    resource_id: `${contract.taskId}/${contract.attemptId}`,
    outcome: manifest.outcome,
    error_kind: manifest.failure_class,
    result_type: contract.resultType,
    scenarios: summary,
    manifest: relative(repoRoot, paths.manifest),
  });
  if (failure) printFailure('verify', failure);

  const teardown = cleanup(failure ? 'failure' : 'success', primaryStatus);
  if (!teardown.clean) {
    printFailure(
      'verify teardown',
      new LifecycleError('repo/teardown_failed', 'the gate process tree survived teardown', {
        record: relative(repoRoot, paths.teardown),
      }),
    );
  }
  return teardown.exitCode;
}

main()
  .then((code) => process.exit(code))
  .catch((error) => {
    const failure = printFailure('verify', error);
    printEvent({
      event: 'lifecycle.verify',
      operation: 'verify',
      resource_id: `${process.env.AGENT_TASK_ID ?? 'unknown'}/${process.env.AGENT_ATTEMPT_ID ?? 'unknown'}`,
      outcome: 'failed',
      error_kind: failure.failureClass,
      retryable: failure.retryable,
    });
    process.exit(failure.exitCode);
  });
