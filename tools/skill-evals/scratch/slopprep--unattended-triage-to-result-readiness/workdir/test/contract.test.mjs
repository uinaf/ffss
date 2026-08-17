import assert from 'node:assert/strict';
import test from 'node:test';

import { allowedBranch, parseRunnerContract, secretValues } from '../scripts/lifecycle/contract.mjs';

const IDENTITY = {
  INFISICAL_TOKEN: 'st.machine.aaaaaaaaaaaaaaaa',
  INFISICAL_API_URL: 'https://secrets.example.com',
  INFISICAL_PROJECT_ID: 'proj_1234',
};

const IMPLEMENTATION = {
  ...IDENTITY,
  AGENT_TASK_ID: 'task-123',
  AGENT_ATTEMPT_ID: 'attempt-1',
  AGENT_RESULT_TYPE: 'implementation',
};

const QA = {
  ...IDENTITY,
  AGENT_TASK_ID: 'task-123',
  AGENT_ATTEMPT_ID: 'attempt-1',
  AGENT_RESULT_TYPE: 'qa',
  AGENT_TARGET_REVISION: 'deadbeef',
};

function failsWith(env, failureClass) {
  try {
    parseRunnerContract(env);
  } catch (error) {
    assert.equal(error.failureClass, failureClass);
    return error;
  }
  assert.fail(`expected ${failureClass}`);
}

test('implementation defaults to a pull request, no merge authority, main base', () => {
  const contract = parseRunnerContract(IMPLEMENTATION);
  assert.equal(contract.submissionTarget, 'pull_request');
  assert.equal(contract.mergeAuthority, 'none');
  assert.equal(contract.baseRef, 'main');
  assert.equal(contract.artifactDir, 'artifacts/task-123/attempt-1');
  assert.equal(allowedBranch(contract), 'agent/task-123/attempt-1');
});

test('qa defaults to a qa report and never gets a branch', () => {
  const contract = parseRunnerContract(QA);
  assert.equal(contract.submissionTarget, 'qa_report');
  assert.equal(contract.baseRef, null);
  assert.equal(contract.targetRevision, 'deadbeef');
  assert.equal(allowedBranch(contract), null);
});

test('a qa task may not be dispatched at a pull request', () => {
  failsWith({ ...QA, AGENT_SUBMISSION_TARGET: 'pull_request' }, 'runner/invalid_submission_target');
});

test('merge authority is only grantable on implementation tasks', () => {
  failsWith({ ...QA, AGENT_MERGE_AUTHORITY: 'merge' }, 'runner/invalid_authority');
  assert.equal(parseRunnerContract({ ...IMPLEMENTATION, AGENT_MERGE_AUTHORITY: 'merge' }).mergeAuthority, 'merge');
});

test('missing task identity, machine identity, and target revision are runner failures', () => {
  failsWith({ ...IMPLEMENTATION, AGENT_TASK_ID: '' }, 'runner/missing_task_identity');
  failsWith({ ...IMPLEMENTATION, INFISICAL_TOKEN: undefined }, 'runner/missing_identity');
  failsWith({ ...QA, AGENT_TARGET_REVISION: '' }, 'runner/missing_target_revision');
  const error = failsWith({ ...IMPLEMENTATION, INFISICAL_API_URL: 'http://secrets.example.com' }, 'runner/invalid_identity');
  assert.equal(error.owner, 'runner');
  assert.equal(error.exitCode, 2);
});

test('identifiers that would escape the artifact directory are rejected', () => {
  failsWith({ ...IMPLEMENTATION, AGENT_TASK_ID: '../../etc' }, 'runner/missing_task_identity');
  failsWith({ ...IMPLEMENTATION, AGENT_ATTEMPT_ID: 'a/b' }, 'runner/missing_task_identity');
});

test('no failure context or contract field carries a secret value', () => {
  const error = failsWith({ ...IMPLEMENTATION, INFISICAL_API_URL: 'not-a-url' }, 'runner/invalid_identity');
  const serialised = JSON.stringify(error.toJSON());
  assert.ok(!serialised.includes(IDENTITY.INFISICAL_TOKEN));

  const contract = parseRunnerContract(IMPLEMENTATION);
  assert.ok(!JSON.stringify(contract).includes(IDENTITY.INFISICAL_TOKEN));
  assert.equal(contract.identity.tokenPresent, true);
});

test('secret value collection covers token-shaped names and spares the non-secret ones', () => {
  const values = secretValues({ ...IDENTITY, SOME_API_KEY: 'abcdefghij', SHORT_TOKEN: 'abc' });
  assert.ok(values.includes(IDENTITY.INFISICAL_TOKEN));
  assert.ok(values.includes('abcdefghij'));
  assert.ok(!values.includes(IDENTITY.INFISICAL_API_URL));
  assert.ok(!values.includes(IDENTITY.INFISICAL_PROJECT_ID));
  assert.ok(!values.includes('abc'), 'values too short to be secrets would cause false positives');
});
