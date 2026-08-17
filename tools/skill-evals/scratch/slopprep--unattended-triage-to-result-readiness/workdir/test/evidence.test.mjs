import assert from 'node:assert/strict';
import test, { after } from 'node:test';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import {
  REDACTION,
  SCENARIOS_SCHEMA,
  artifactPaths,
  classifyFormat,
  describeArtifacts,
  listArtifactFiles,
  readScenarios,
  scanForSecrets,
} from '../scripts/lifecycle/artifacts.mjs';

const scratch = [];
after(() => {
  for (const dir of scratch) rmSync(dir, { recursive: true, force: true });
});

function attempt(scenarios, files = {}) {
  const root = mkdtempSync(join(tmpdir(), 'attempt-'));
  scratch.push(root);
  const paths = artifactPaths({ artifactDir: 'artifacts/t/a' }, root);
  mkdirSync(paths.evidence, { recursive: true });
  mkdirSync(paths.logs, { recursive: true });
  for (const [rel, content] of Object.entries(files)) {
    writeFileSync(join(paths.root, rel), content, 'utf8');
  }
  if (scenarios) writeFileSync(paths.scenarios, JSON.stringify(scenarios), 'utf8');
  return paths;
}

const VALID = {
  schema: SCENARIOS_SCHEMA,
  title: 'Playback on the set-top build',
  scenarios: [
    {
      id: 'playback-resumes',
      result: 'passed',
      producer: 'playwright@1.55 chromium',
      evidence: [{ path: 'evidence/playback.png', captured_at: '2026-08-17T09:00:00Z' }],
    },
  ],
};

test('artifact paths stay inside the workspace', () => {
  assert.throws(
    () => artifactPaths({ artifactDir: '../escape' }, '/tmp/workspace'),
    /escaped the workspace/,
  );
});

test('a valid qa scenario file resolves its evidence, producer, and capture time', () => {
  const paths = attempt(VALID, { 'evidence/playback.png': 'png-bytes' });
  const { declared, scenarioByPath } = readScenarios(paths);
  assert.equal(declared.scenarios.length, 1);
  assert.deepEqual(scenarioByPath.get('evidence/playback.png'), {
    scenario: 'playback-resumes',
    producer: 'playwright@1.55 chromium',
    captured_at: '2026-08-17T09:00:00Z',
  });
});

test('a qa attempt with no scenario file is missing evidence, not passing', () => {
  const paths = attempt(null);
  assert.throws(() => readScenarios(paths), (error) => error.failureClass === 'repo/evidence_missing');
});

test('a scenario that claims evidence it never captured fails', () => {
  const paths = attempt(VALID);
  assert.throws(
    () => readScenarios(paths),
    (error) => error.failureClass === 'repo/evidence_missing' && /playback.png/.test(error.message),
  );
});

test('scenario schema, result vocabulary, and producer are enforced', () => {
  const cases = [
    [{ ...VALID, schema: 'something-else' }, 'repo/evidence_invalid'],
    [{ ...VALID, scenarios: [] }, 'repo/evidence_missing'],
    [{ ...VALID, scenarios: [{ ...VALID.scenarios[0], result: 'mostly ok' }] }, 'repo/evidence_invalid'],
    [{ ...VALID, scenarios: [{ ...VALID.scenarios[0], producer: undefined }] }, 'repo/evidence_invalid'],
    [
      { ...VALID, scenarios: [{ ...VALID.scenarios[0], evidence: ['../../../etc/hosts'] }] },
      'repo/evidence_invalid',
    ],
  ];
  for (const [declared, expected] of cases) {
    const paths = attempt(declared, { 'evidence/playback.png': 'png-bytes' });
    assert.throws(() => readScenarios(paths), (error) => error.failureClass === expected, JSON.stringify(declared));
  }
});

test('a skipped scenario may carry no evidence', () => {
  const paths = attempt({
    ...VALID,
    scenarios: [{ id: 'offline-mode', result: 'skipped', producer: 'manual', evidence: [] }],
  });
  assert.equal(readScenarios(paths).declared.scenarios.length, 1);
});

test('an injected secret in preserved text evidence is detected and never echoed', () => {
  const secret = 'st.machine.abcdefghijklmnop';
  const paths = attempt(VALID, { 'logs/verify.log': `authorization: Bearer ${secret}\n` });
  const scan = scanForSecrets(paths.root, listArtifactFiles(paths), [secret]);
  assert.equal(scan.findings.length, 1);
  assert.equal(scan.findings[0].path, 'logs/verify.log');
  assert.ok(!JSON.stringify(scan).includes(secret));
});

test('binary evidence is described as unscanned rather than claimed clean', () => {
  const paths = attempt(VALID, { 'evidence/playback.png': 'png-bytes', 'logs/verify.log': 'ok\n' });
  const described = describeArtifacts(paths, {
    producer: 'pnpm verify',
    scenarioByPath: readScenarios(paths).scenarioByPath,
    redactionFor: (rel) => (classifyFormat(rel).text ? REDACTION.clean : REDACTION.unscanned),
  });
  const byPath = new Map(described.map((item) => [item.path, item]));
  assert.equal(byPath.get('evidence/playback.png').redaction, REDACTION.unscanned);
  assert.equal(byPath.get('evidence/playback.png').format, 'image/png');
  assert.equal(byPath.get('evidence/playback.png').producer, 'playwright@1.55 chromium');
  assert.equal(byPath.get('logs/verify.log').redaction, REDACTION.clean);
  assert.equal(byPath.get('logs/verify.log').producer, 'pnpm verify');
  for (const item of described) {
    assert.match(item.sha256, /^[0-9a-f]{64}$/);
    assert.ok(item.captured_at);
  }
});
