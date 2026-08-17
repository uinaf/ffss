/**
 * Drift guard: the guide, the contract document, and the code must not disagree.
 * A stale command or an invented failure class is a readiness failure, so it
 * fails the gate rather than waiting to be noticed during an unattended run.
 */

import assert from 'node:assert/strict';
import test from 'node:test';
import { existsSync, readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

import { FAILURE_CLASSES } from '../scripts/lifecycle/failure.mjs';
import { RESOURCE_KINDS } from '../scripts/lifecycle/resources.mjs';
import { RESULT_TYPES } from '../scripts/lifecycle/contract.mjs';
import { SCENARIOS_SCHEMA, SCENARIO_RESULTS } from '../scripts/lifecycle/artifacts.mjs';

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const DOCS = ['AGENTS.md', 'README.md', 'docs/automation.md'];

function read(relativePath) {
  return readFileSync(join(repoRoot, relativePath), 'utf8');
}

const pkg = JSON.parse(read('package.json'));

test('every pnpm command the docs name is a declared script', () => {
  for (const doc of DOCS) {
    for (const match of read(doc).matchAll(/pnpm (?:run )?([a-z][a-z-]*)/g)) {
      assert.ok(
        Object.hasOwn(pkg.scripts, match[1]),
        `${doc} references "pnpm ${match[1]}" but package.json declares no such script`,
      );
    }
  }
});

test('the declared lifecycle entrypoints exist and point at real files', () => {
  for (const name of ['bootstrap', 'doctor', 'verify', 'teardown', 'own', 'test']) {
    assert.ok(pkg.scripts[name], `package.json must declare the ${name} script`);
  }
  for (const [name, command] of Object.entries(pkg.scripts)) {
    const script = command.match(/scripts\/[\w./-]+/)?.[0];
    if (!script) continue;
    assert.ok(existsSync(join(repoRoot, script)), `the ${name} script points at missing ${script}`);
  }
});

test('the canonical gate is still reached through pnpm verify', () => {
  const verify = read('scripts/lifecycle/verify.mjs');
  assert.match(verify, /GATE_COMMAND = \['pnpm', 'test'\]/, 'verify must extend the repository test gate, not replace it');
});

test('every failure class named in the docs exists in the taxonomy', () => {
  const known = new Set(Object.keys(FAILURE_CLASSES));
  for (const doc of DOCS) {
    for (const match of read(doc).matchAll(/`((?:runner|repo|internal)\/[a-z_]+)`/g)) {
      assert.ok(known.has(match[1]), `${doc} names unknown failure class ${match[1]}`);
    }
  }
});

test('every failure class carries an owner, retryability, and a recovery action', () => {
  for (const [name, meta] of Object.entries(FAILURE_CLASSES)) {
    assert.ok(['runner', 'repo', 'internal'].includes(meta.owner), `${name} has no valid owner`);
    assert.equal(typeof meta.retryable, 'boolean', `${name} does not declare retryability`);
    assert.ok(meta.recovery?.length > 20, `${name} has no actionable recovery text`);
    assert.equal(name.split('/')[0], meta.owner, `${name} prefix contradicts its owner`);
  }
});

test('every vocabulary the code enforces is documented', () => {
  const guidance = `${read('AGENTS.md')}\n${read('docs/automation.md')}`;
  for (const value of [...RESULT_TYPES, ...SCENARIO_RESULTS, ...RESOURCE_KINDS, SCENARIOS_SCHEMA]) {
    assert.ok(guidance.includes(value), `neither AGENTS.md nor docs/automation.md mentions ${value}`);
  }
});

test('every repository path the guide links resolves', () => {
  for (const doc of DOCS) {
    for (const match of read(doc).matchAll(/\]\((?!https?:)([^)#]+)/g)) {
      const target = match[1].startsWith('docs/') || !doc.startsWith('docs/') ? match[1] : join('docs', match[1]);
      assert.ok(existsSync(join(repoRoot, target)), `${doc} links missing path ${target}`);
    }
  }
});

test('the interactive setup path is gone and not resurrected in the docs', () => {
  assert.equal(existsSync(join(repoRoot, 'scripts/current-agent-setup.sh')), false);
  for (const doc of DOCS) {
    const content = read(doc);
    assert.ok(!content.includes('current-agent-setup'), `${doc} still points at the removed interactive script`);
    assert.ok(!/infisical login/i.test(content), `${doc} still instructs an interactive login`);
  }
});

test('the shipped qa example matches the schema the gate enforces', () => {
  const example = JSON.parse(read('docs/examples/qa-scenarios.json'));
  assert.equal(example.schema, SCENARIOS_SCHEMA);
  assert.ok(example.scenarios.length > 0);
  for (const scenario of example.scenarios) {
    assert.ok(scenario.id, 'each example scenario needs an id');
    assert.ok(SCENARIO_RESULTS.includes(scenario.result));
    assert.ok(scenario.producer, 'each example scenario names its producer');
    assert.ok(scenario.result === 'skipped' || scenario.evidence.length > 0);
  }
});

test('CI drives the same lifecycle commands and preserves the artifacts', () => {
  const workflow = read('.github/workflows/verify.yml');
  for (const command of ['pnpm bootstrap', 'pnpm doctor', 'pnpm verify', 'pnpm run teardown']) {
    assert.ok(workflow.includes(command), `CI must run ${command}`);
  }
  assert.match(workflow, /node-version-file: package\.json/, 'CI must read the node version from the repository');
  assert.match(workflow, /path: artifacts\//, 'CI must preserve the attempt artifacts');
});
