import assert from 'node:assert/strict';
import test, { after } from 'node:test';
import { existsSync, mkdtempSync, rmSync } from 'node:fs';
import { spawn } from 'node:child_process';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import {
  currentOwnership,
  normaliseEntry,
  readLedger,
  recordResource,
  releasable,
  releaseEntry,
  verifyAbsent,
} from '../scripts/lifecycle/resources.mjs';

const scratch = [];
after(() => {
  for (const dir of scratch) rmSync(dir, { recursive: true, force: true });
});

function ledger() {
  const dir = mkdtempSync(join(tmpdir(), 'ledger-'));
  scratch.push(dir);
  return join(dir, 'owned-resources.jsonl');
}

test('a recorded resource round-trips through the ledger', () => {
  const path = ledger();
  recordResource(path, { kind: 'process_group', id: '4321', pgid: 4321, label: 'dev-server', scope: 'verify' });
  const entries = readLedger(path);
  assert.equal(entries.length, 1);
  assert.equal(entries[0].kind, 'process_group');
  assert.equal(entries[0].released_at, null);
  assert.ok(entries[0].acquired_at);
});

test('release supersedes acquisition, so teardown is idempotent', () => {
  const path = ledger();
  recordResource(path, { kind: 'process', id: '11', scope: 'verify' });
  assert.equal(releasable(readLedger(path)).length, 1);
  recordResource(path, { kind: 'process', id: '11', scope: 'verify', released_at: new Date().toISOString() });
  assert.equal(releasable(readLedger(path)).length, 0);
  assert.equal(currentOwnership(readLedger(path)).length, 1);
});

test('pre-existing resources are never releasable', () => {
  const path = ledger();
  recordResource(path, { kind: 'container', id: 'abc123' });
  const preexisting = [{ kind: 'container', id: 'abc123' }];
  assert.equal(releasable(readLedger(path), { preexisting }).length, 0);
  assert.equal(releasable(readLedger(path), { preexisting: [] }).length, 1);
});

test('scope narrows what a stage releases', () => {
  const path = ledger();
  recordResource(path, { kind: 'process', id: '1', scope: 'verify' });
  recordResource(path, { kind: 'process', id: '2', scope: 'attempt' });
  assert.deepEqual(
    releasable(readLedger(path), { scope: 'verify' }).map((entry) => entry.id),
    ['1'],
  );
  assert.equal(releasable(readLedger(path), { scope: 'attempt' }).length, 2);
});

test('kinds without a built-in handler must carry release argv', () => {
  assert.throws(() => normaliseEntry({ kind: 'vm', id: 'builder-1' }), /no built-in handler/);
  assert.deepEqual(normaliseEntry({ kind: 'vm', id: 'builder-1', release: ['true'], verify: ['true'] }).release, ['true']);
});

test('an unknown kind or a missing id is rejected rather than guessed', () => {
  assert.throws(() => normaliseEntry({ kind: 'kubernetes_pod', id: 'x' }), /unknown resource kind/);
  assert.throws(() => normaliseEntry({ kind: 'process', id: '  ' }), /exact resource id/);
});

test('releasing a temp dir the attempt owns verifies absence', () => {
  const dir = mkdtempSync(join(tmpdir(), 'owned-'));
  const entry = normaliseEntry({ kind: 'temp_dir', id: dir, scope: 'verify' });
  assert.equal(verifyAbsent(entry), false);
  const result = releaseEntry(entry);
  assert.equal(result.absent, true);
  assert.ok(result.released_at);
  assert.equal(existsSync(dir), false);
});

test('a real owned process tree is released and its absence verified', () => {
  // A launcher plus a descendant: releasing the direct pid alone would leak the child.
  const child = spawn('/bin/sh', ['-c', 'sleep 45 & sleep 45'], { detached: true, stdio: 'ignore' });
  child.unref();
  const entry = normaliseEntry({ kind: 'process_group', id: String(child.pid), pgid: child.pid, scope: 'verify' });
  assert.equal(verifyAbsent(entry), false, 'the launched group should be alive before release');

  const result = releaseEntry(entry);
  assert.equal(result.absent, true, result.note);
  assert.equal(verifyAbsent(entry), true);
});

test('a process the attempt never recorded is invisible to teardown', () => {
  const child = spawn('/bin/sh', ['-c', 'sleep 30'], { detached: true, stdio: 'ignore' });
  child.unref();
  const path = ledger();
  recordResource(path, { kind: 'temp_dir', id: mkdtempSync(join(tmpdir(), 'kept-')), scope: 'verify' });
  const targets = releasable(readLedger(path));
  assert.ok(!targets.some((entry) => entry.id === String(child.pid)));
  try {
    process.kill(-child.pid, 'SIGKILL');
  } catch {
    /* already gone */
  }
});
