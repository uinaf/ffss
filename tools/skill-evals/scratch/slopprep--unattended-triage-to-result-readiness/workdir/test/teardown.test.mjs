import assert from 'node:assert/strict';
import test, { after } from 'node:test';
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { spawn } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { EXIT } from '../scripts/lifecycle/failure.mjs';
import { artifactPaths, ensureArtifactDirs } from '../scripts/lifecycle/artifacts.mjs';
import { recordResource, writeSnapshot } from '../scripts/lifecycle/resources.mjs';
import { runTeardown } from '../scripts/lifecycle/teardown.mjs';

const scratch = [];
after(() => {
  for (const dir of scratch) rmSync(dir, { recursive: true, force: true });
});

const CONTRACT = { taskId: 'task-1', attemptId: 'attempt-1', artifactDir: 'artifacts/task-1/attempt-1' };

function attempt() {
  const root = mkdtempSync(join(tmpdir(), 'teardown-'));
  scratch.push(root);
  return ensureArtifactDirs(artifactPaths(CONTRACT, root));
}

test('teardown with nothing owned is clean and repeatable', () => {
  const paths = attempt();
  const first = runTeardown({ contract: CONTRACT, paths, reason: 'success' });
  const second = runTeardown({ contract: CONTRACT, paths, reason: 'retry' });
  assert.equal(first.clean, true);
  assert.equal(first.exitCode, EXIT.ok);
  assert.equal(second.clean, true);
  assert.equal(JSON.parse(readFileSync(paths.teardown, 'utf8')).runs.length, 2, 'each run appends to the record');
});

test('teardown releases the owned process tree and verifies its absence', () => {
  const paths = attempt();
  const child = spawn('/bin/sh', ['-c', 'sleep 45 & sleep 45'], { detached: true, stdio: 'ignore' });
  child.unref();
  recordResource(paths.ledger, { kind: 'process_group', id: String(child.pid), pgid: child.pid, scope: 'verify' });

  const result = runTeardown({ contract: CONTRACT, paths, scope: 'verify', reason: 'success' });
  assert.equal(result.clean, true, JSON.stringify(result.survivors));
  const record = JSON.parse(readFileSync(paths.teardown, 'utf8')).runs.at(-1);
  assert.equal(record.released.length, 1);
  assert.equal(record.released[0].id, String(child.pid));

  // Idempotent: the second call has nothing left to release.
  const again = runTeardown({ contract: CONTRACT, paths, scope: 'verify', reason: 'retry' });
  assert.equal(again.clean, true);
  assert.equal(JSON.parse(readFileSync(paths.teardown, 'utf8')).runs.at(-1).released.length, 0);
});

test('a primary failure status survives a clean teardown', () => {
  const paths = attempt();
  const result = runTeardown({ contract: CONTRACT, paths, reason: 'failure', primaryStatus: EXIT.repository });
  assert.equal(result.clean, true);
  assert.equal(result.exitCode, EXIT.repository, 'cleanup must not overwrite the primary failure');
});

test('an incomplete teardown after successful work exits non-zero', () => {
  const paths = attempt();
  // A resource whose release reports success but whose absence check refuses.
  recordResource(paths.ledger, { kind: 'vm', id: 'builder-1', release: ['true'], verify: ['false'] });
  const result = runTeardown({ contract: CONTRACT, paths, reason: 'success' });
  assert.equal(result.clean, false);
  assert.equal(result.exitCode, EXIT.teardown);
  assert.equal(result.survivors[0].id, 'builder-1');
});

test('an incomplete teardown reports separately instead of masking a signal status', () => {
  const paths = attempt();
  recordResource(paths.ledger, { kind: 'vm', id: 'builder-1', release: ['true'], verify: ['false'] });
  const result = runTeardown({ contract: CONTRACT, paths, reason: 'cancelled', primaryStatus: 130 });
  assert.equal(result.clean, false);
  assert.equal(result.exitCode, 130);
  assert.equal(JSON.parse(readFileSync(paths.teardown, 'utf8')).runs.at(-1).outcome, 'incomplete');
});

test('pre-existing resources are preserved and captured evidence survives teardown', () => {
  const paths = attempt();
  const preexisting = mkdtempSync(join(tmpdir(), 'preexisting-'));
  scratch.push(preexisting);
  writeSnapshot(paths.snapshot, { captured_at: new Date().toISOString(), items: [{ kind: 'temp_dir', id: preexisting }] });
  // The same id also appears in the ledger; the snapshot still wins.
  recordResource(paths.ledger, { kind: 'temp_dir', id: preexisting, scope: 'attempt' });
  writeFileSync(join(paths.logs, 'verify.log'), 'gate output\n', 'utf8');

  const result = runTeardown({ contract: CONTRACT, paths, reason: 'success' });
  assert.equal(result.clean, true);
  assert.deepEqual(result.preservationViolations, []);
  assert.equal(existsSync(preexisting), true, 'a resource that predates the attempt is never released');
  assert.equal(existsSync(join(paths.logs, 'verify.log')), true, 'cleanup must not eat the evidence');
});
