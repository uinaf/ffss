/**
 * Ownership ledger for runtime resources an attempt raises.
 *
 * Rules this module enforces:
 *  - only resources recorded by this attempt are released
 *  - resources present in the pre-existing snapshot are never released
 *  - release happens by recorded id or owned process group, never by name
 *  - absence is verified after release; cleanup exit status is not trusted
 */

import { appendFileSync, existsSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { spawnSync } from 'node:child_process';

import { LifecycleError } from './failure.mjs';

export const RESOURCE_KINDS = [
  'process',
  'process_group',
  'container',
  'simulator',
  'emulator',
  'vm',
  'service',
  'temp_dir',
];

/** Which stage raised the resource; teardown scopes releases by this. */
export const SCOPES = ['bootstrap', 'verify', 'attempt'];

const SIGNAL_GRACE_MS = 5000;
const COMMAND_TIMEOUT_MS = 60_000;

function run(argv, { timeout = COMMAND_TIMEOUT_MS } = {}) {
  const [command, ...args] = argv;
  const result = spawnSync(command, args, { encoding: 'utf8', timeout });
  return {
    status: result.status,
    stdout: (result.stdout ?? '').trim(),
    stderr: (result.stderr ?? '').trim(),
    spawnError: result.error ? result.error.code ?? result.error.message : null,
  };
}

function has(command) {
  return run(['/usr/bin/env', 'which', command], { timeout: 5000 }).status === 0;
}

function sleepMs(ms) {
  // Bounded synchronous wait; teardown must not depend on an event loop turn.
  const shared = new SharedArrayBuffer(4);
  Atomics.wait(new Int32Array(shared), 0, 0, ms);
}

/**
 * @param {object} entry
 * @returns {object} normalised ledger entry
 */
export function normaliseEntry(entry) {
  if (!RESOURCE_KINDS.includes(entry.kind)) {
    throw new LifecycleError('repo/unreleasable_resource', `unknown resource kind: ${entry.kind}`, {
      known_kinds: RESOURCE_KINDS,
    });
  }
  if (entry.id === undefined || String(entry.id).trim() === '') {
    throw new LifecycleError('repo/unreleasable_resource', 'a ledger entry needs an exact resource id', {
      kind: entry.kind,
    });
  }
  const scope = entry.scope ?? 'attempt';
  if (!SCOPES.includes(scope)) {
    throw new LifecycleError('repo/unreleasable_resource', `unknown scope: ${scope}`, { known_scopes: SCOPES });
  }
  const needsCustom = ['emulator', 'vm', 'service'].includes(entry.kind);
  if (needsCustom && !Array.isArray(entry.release)) {
    throw new LifecycleError(
      'repo/unreleasable_resource',
      `kind ${entry.kind} has no built-in handler; record release (and verify) argv with it`,
      { kind: entry.kind },
    );
  }
  return {
    kind: entry.kind,
    id: String(entry.id),
    pgid: entry.pgid ?? null,
    label: entry.label ?? null,
    scope,
    runtime: entry.runtime ?? (entry.kind === 'container' ? 'docker' : null),
    release: Array.isArray(entry.release) ? entry.release : null,
    verify: Array.isArray(entry.verify) ? entry.verify : null,
    pre_existing: entry.pre_existing === true,
    acquired_at: entry.acquired_at ?? new Date().toISOString(),
    released_at: entry.released_at ?? null,
  };
}

export function recordResource(ledgerPath, entry) {
  const normalised = normaliseEntry(entry);
  mkdirSync(dirname(ledgerPath), { recursive: true });
  appendFileSync(ledgerPath, `${JSON.stringify(normalised)}\n`, 'utf8');
  return normalised;
}

export function readLedger(ledgerPath) {
  if (!existsSync(ledgerPath)) return [];
  return readFileSync(ledgerPath, 'utf8')
    .split('\n')
    .filter((line) => line.trim() !== '')
    .map((line) => JSON.parse(line));
}

/**
 * Latest state per (kind, id): a released entry supersedes its acquisition, which
 * makes teardown idempotent across retries.
 */
export function currentOwnership(entries) {
  const byKey = new Map();
  for (const entry of entries) {
    byKey.set(`${entry.kind}:${entry.id}`, { ...(byKey.get(`${entry.kind}:${entry.id}`) ?? {}), ...entry });
  }
  return [...byKey.values()];
}

/**
 * Resources this attempt must release: recorded, not pre-existing, not already
 * released, and in scope.
 */
export function releasable(entries, { scope = 'attempt', preexisting = [] } = {}) {
  const preexistingKeys = new Set(preexisting.map((item) => `${item.kind}:${item.id}`));
  return currentOwnership(entries).filter((entry) => {
    if (entry.pre_existing) return false;
    if (preexistingKeys.has(`${entry.kind}:${entry.id}`)) return false;
    if (entry.released_at) return false;
    if (scope !== 'attempt' && entry.scope !== scope) return false;
    return true;
  });
}

function releaseProcess(entry) {
  const target = entry.kind === 'process_group' ? -Number(entry.pgid ?? entry.id) : Number(entry.id);
  for (const signal of ['SIGTERM', 'SIGKILL']) {
    try {
      process.kill(target, signal);
    } catch (error) {
      if (error.code === 'ESRCH') return { status: 0, note: 'already gone' };
      if (error.code === 'EPERM') {
        return { status: 1, note: 'not permitted; the attempt does not own this process' };
      }
      throw error;
    }
    sleepMs(SIGNAL_GRACE_MS);
    if (!processAlive(target)) return { status: 0, note: `released with ${signal}` };
  }
  return { status: 1, note: 'survived SIGKILL' };
}

function processAlive(target) {
  try {
    process.kill(target, 0);
    return true;
  } catch (error) {
    return error.code === 'EPERM';
  }
}

function releaseContainer(entry) {
  const runtime = entry.runtime ?? 'docker';
  if (!has(runtime)) return { status: 1, note: `${runtime} is not on PATH` };
  const stopped = run([runtime, 'stop', entry.id]);
  return { status: stopped.status === 0 ? 0 : 1, note: stopped.stderr || stopped.stdout || 'stop issued' };
}

function containerRunning(entry) {
  const runtime = entry.runtime ?? 'docker';
  if (!has(runtime)) return false;
  const inspected = run([runtime, 'inspect', '-f', '{{.State.Running}}', entry.id]);
  return inspected.status === 0 && inspected.stdout === 'true';
}

function releaseSimulator(entry) {
  if (!has('xcrun')) return { status: 1, note: 'xcrun is not on PATH' };
  const shutdown = run(['xcrun', 'simctl', 'shutdown', entry.id]);
  const gone = shutdown.status === 0 || /current state: Shutdown/i.test(shutdown.stderr);
  return { status: gone ? 0 : 1, note: shutdown.stderr || 'shutdown issued' };
}

function simulatorBooted(entry) {
  if (!has('xcrun')) return false;
  const listed = run(['xcrun', 'simctl', 'list', 'devices', 'booted']);
  return listed.status === 0 && listed.stdout.includes(entry.id);
}

function releaseTempDir(entry) {
  rmSync(entry.id, { recursive: true, force: true });
  return { status: 0, note: 'removed' };
}

/** Release one entry. Returns a machine-readable outcome; never throws for a live resource. */
export function releaseEntry(entry) {
  const started = new Date().toISOString();
  let outcome;
  try {
    if (entry.release) {
      const result = run(entry.release);
      outcome = { status: result.status === 0 ? 0 : 1, note: result.stderr || result.spawnError || 'custom release' };
    } else if (entry.kind === 'process' || entry.kind === 'process_group') {
      outcome = releaseProcess(entry);
    } else if (entry.kind === 'container') {
      outcome = releaseContainer(entry);
    } else if (entry.kind === 'simulator') {
      outcome = releaseSimulator(entry);
    } else if (entry.kind === 'temp_dir') {
      outcome = releaseTempDir(entry);
    } else {
      outcome = { status: 1, note: 'no handler and no recorded release argv' };
    }
  } catch (error) {
    outcome = { status: 1, note: error.code ?? error.message };
  }
  const absent = verifyAbsent(entry);
  return {
    kind: entry.kind,
    id: entry.id,
    label: entry.label,
    scope: entry.scope,
    release_status: outcome.status,
    note: outcome.note,
    absent,
    released_at: absent ? new Date().toISOString() : null,
    attempted_at: started,
  };
}

/** Read-only absence check. Trusted over the release command's exit status. */
export function verifyAbsent(entry) {
  if (entry.verify) return run(entry.verify).status === 0;
  switch (entry.kind) {
    case 'process':
      return !processAlive(Number(entry.id));
    case 'process_group':
      return !processAlive(-Number(entry.pgid ?? entry.id));
    case 'container':
      return !containerRunning(entry);
    case 'simulator':
      return !simulatorBooted(entry);
    case 'temp_dir':
      return !existsSync(entry.id);
    default:
      return false;
  }
}

/**
 * Snapshot of resources already running before the attempt touched anything.
 * Only kinds this repository can enumerate cheaply are recorded; the snapshot is
 * a preservation guard, not an inventory.
 */
export function snapshotPreexisting() {
  const items = [];
  if (has('docker')) {
    const listed = run(['docker', 'ps', '-q']);
    if (listed.status === 0) {
      for (const id of listed.stdout.split('\n').filter(Boolean)) {
        items.push({ kind: 'container', id, runtime: 'docker' });
      }
    }
  }
  if (has('xcrun')) {
    const listed = run(['xcrun', 'simctl', 'list', 'devices', 'booted']);
    if (listed.status === 0) {
      for (const match of listed.stdout.matchAll(/\(([0-9A-F-]{36})\)/g)) {
        items.push({ kind: 'simulator', id: match[1] });
      }
    }
  }
  return { captured_at: new Date().toISOString(), items };
}

export function writeSnapshot(path, snapshot) {
  mkdirSync(dirname(path), { recursive: true });
  writeFileSync(path, `${JSON.stringify(snapshot, null, 2)}\n`, 'utf8');
}

export function readSnapshot(path) {
  if (!existsSync(path)) return { captured_at: null, items: [] };
  return JSON.parse(readFileSync(path, 'utf8'));
}

/**
 * Confirms pre-existing resources are still present. A preserved resource that
 * vanished means teardown reached past its ownership boundary.
 */
export function verifyPreservation(snapshot) {
  return snapshot.items
    .map((item) => ({ ...item, still_present: !verifyAbsent(normaliseEntry({ ...item, pre_existing: true })) }))
    .filter((item) => !item.still_present);
}
