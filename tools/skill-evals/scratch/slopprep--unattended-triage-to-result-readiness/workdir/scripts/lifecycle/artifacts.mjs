/**
 * Artifact directory layout, evidence description, and the machine-readable
 * manifest an orchestrator reconciles against.
 */

import { createHash } from 'node:crypto';
import { existsSync, mkdirSync, readFileSync, readdirSync, statSync, writeFileSync } from 'node:fs';
import { join, relative, resolve, sep } from 'node:path';
import { spawnSync } from 'node:child_process';
import { arch, platform, release } from 'node:os';

import { LifecycleError } from './failure.mjs';

export const MANIFEST_SCHEMA = 'agent-artifact-manifest/1';
export const SCENARIOS_SCHEMA = 'agent-qa-scenarios/1';
export const SCENARIO_RESULTS = ['passed', 'failed', 'blocked', 'skipped'];

const TEXT_FORMATS = new Map([
  ['.log', 'text/plain'],
  ['.txt', 'text/plain'],
  ['.md', 'text/markdown'],
  ['.json', 'application/json'],
  ['.jsonl', 'application/x-ndjson'],
  ['.har', 'application/json'],
  ['.csv', 'text/csv'],
  ['.xml', 'text/xml'],
]);

const BINARY_FORMATS = new Map([
  ['.png', 'image/png'],
  ['.jpg', 'image/jpeg'],
  ['.jpeg', 'image/jpeg'],
  ['.webp', 'image/webp'],
  ['.gif', 'image/gif'],
  ['.mp4', 'video/mp4'],
  ['.webm', 'video/webm'],
  ['.mov', 'video/quicktime'],
  ['.zip', 'application/zip'],
]);

export function artifactPaths(contract, repoRoot) {
  const root = resolve(repoRoot, contract.artifactDir);
  if (!root.startsWith(resolve(repoRoot) + sep)) {
    throw new LifecycleError('repo/contract_violation', 'artifact directory escaped the workspace', {
      artifact_dir: contract.artifactDir,
    });
  }
  return {
    root,
    logs: join(root, 'logs'),
    evidence: join(root, 'evidence'),
    manifest: join(root, 'manifest.json'),
    scenarios: join(root, 'scenarios.json'),
    ledger: join(root, 'owned-resources.jsonl'),
    snapshot: join(root, 'preexisting-resources.json'),
    teardown: join(root, 'teardown.json'),
    bootstrap: join(root, 'bootstrap.json'),
    doctor: join(root, 'doctor.json'),
  };
}

export function ensureArtifactDirs(paths) {
  for (const dir of [paths.root, paths.logs, paths.evidence]) {
    try {
      mkdirSync(dir, { recursive: true });
    } catch (error) {
      throw new LifecycleError('runner/workspace_not_writable', `cannot create ${relative(process.cwd(), dir)}`, {
        code: error.code,
      });
    }
  }
  return paths;
}

function git(repoRoot, args) {
  const result = spawnSync('git', args, { cwd: repoRoot, encoding: 'utf8', timeout: 15_000 });
  return result.status === 0 ? result.stdout.trim() : null;
}

export function describeRevision(repoRoot, contract) {
  const head = git(repoRoot, ['rev-parse', 'HEAD']);
  const status = git(repoRoot, ['status', '--porcelain']);
  return {
    git_sha: head,
    dirty: status === null ? null : status !== '',
    base_ref: contract.baseRef,
    /** what the evidence is about: HEAD for implementation, the dispatched revision for qa */
    under_test: contract.resultType === 'qa' ? contract.targetRevision : head,
  };
}

export function describeEnvironment(contract, repoRoot) {
  const pkg = JSON.parse(readFileSync(join(repoRoot, 'package.json'), 'utf8'));
  const pnpm = spawnSync('pnpm', ['--version'], { encoding: 'utf8', timeout: 30_000 });
  return {
    name: contract.environmentName,
    runner: contract.runner,
    os: `${platform()} ${release()}`,
    arch: arch(),
    node: process.version,
    package_manager_declared: pkg.packageManager ?? null,
    package_manager_active: pnpm.status === 0 ? `pnpm@${pnpm.stdout.trim()}` : null,
    ci: process.env.CI === 'true' || process.env.CI === '1',
  };
}

export function classifyFormat(path) {
  const dot = path.lastIndexOf('.');
  const ext = dot === -1 ? '' : path.slice(dot).toLowerCase();
  if (TEXT_FORMATS.has(ext)) return { format: TEXT_FORMATS.get(ext), text: true };
  if (BINARY_FORMATS.has(ext)) return { format: BINARY_FORMATS.get(ext), text: false };
  return { format: 'application/octet-stream', text: false };
}

function walk(dir, root = dir) {
  if (!existsSync(dir)) return [];
  const found = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) found.push(...walk(full, root));
    else if (entry.isFile()) found.push(relative(root, full));
  }
  return found;
}

export function hashFile(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex');
}

/**
 * Scans text artifacts for injected secret values. Reports the artifact path and
 * a count; never the matched value.
 */
export function scanForSecrets(rootDir, relativePaths, secrets) {
  const findings = [];
  let scanned = 0;
  if (secrets.length === 0) return { scanned, findings, policy: 'no-secret-values-in-environment' };
  for (const rel of relativePaths) {
    const { text } = classifyFormat(rel);
    if (!text) continue;
    scanned += 1;
    const content = readFileSync(join(rootDir, rel), 'utf8');
    const matches = secrets.filter((value) => content.includes(value)).length;
    if (matches > 0) findings.push({ path: rel, matched_secret_count: matches });
  }
  return { scanned, findings, policy: 'reject-injected-secret-values' };
}

/** Every preserved file under the attempt directory, excluding the manifest itself. */
export function listArtifactFiles(paths) {
  return walk(paths.root)
    .filter((rel) => rel !== 'manifest.json')
    .sort();
}

export const REDACTION = {
  clean: 'scanned-clean',
  unscanned: 'unscanned-binary',
  exposed: 'secret-detected',
};

/**
 * Describes every preserved file. `producer`, `scenario`, and `captured_at` come
 * from the scenario file when the evidence is claimed there.
 */
export function describeArtifacts(paths, { producer, scenarioByPath = new Map(), redactionFor }) {
  return listArtifactFiles(paths).map((rel) => {
    const full = join(paths.root, rel);
    const claim = scenarioByPath.get(rel);
    const stat = statSync(full);
    return {
      path: rel,
      format: classifyFormat(rel).format,
      bytes: stat.size,
      sha256: hashFile(full),
      producer: claim?.producer ?? producer,
      captured_at: claim?.captured_at ?? stat.mtime.toISOString(),
      scenario: claim?.scenario ?? null,
      redaction: redactionFor(rel),
    };
  });
}

/**
 * Validates the qa scenario file. Its evidence paths are relative to the attempt
 * artifact directory and must already exist.
 */
export function readScenarios(paths) {
  if (!existsSync(paths.scenarios)) {
    throw new LifecycleError(
      'repo/evidence_missing',
      `a qa attempt must write ${relative(process.cwd(), paths.scenarios)} describing each scenario it ran`,
      { expected_schema: SCENARIOS_SCHEMA },
    );
  }
  let parsed;
  try {
    parsed = JSON.parse(readFileSync(paths.scenarios, 'utf8'));
  } catch (error) {
    throw new LifecycleError('repo/evidence_invalid', 'scenarios.json is not valid JSON', { detail: error.message });
  }
  if (parsed.schema !== SCENARIOS_SCHEMA) {
    throw new LifecycleError('repo/evidence_invalid', `scenarios.json schema must be ${SCENARIOS_SCHEMA}`, {
      received: parsed.schema ?? null,
    });
  }
  if (!Array.isArray(parsed.scenarios) || parsed.scenarios.length === 0) {
    throw new LifecycleError('repo/evidence_missing', 'scenarios.json declares no scenarios', {});
  }

  const scenarioByPath = new Map();
  for (const [index, scenario] of parsed.scenarios.entries()) {
    const at = `scenarios[${index}]`;
    if (!scenario.id) {
      throw new LifecycleError('repo/evidence_invalid', `${at} needs an id`, {});
    }
    if (!SCENARIO_RESULTS.includes(scenario.result)) {
      throw new LifecycleError('repo/evidence_invalid', `${at}.result must be one of ${SCENARIO_RESULTS.join(', ')}`, {
        scenario: scenario.id,
        received: scenario.result ?? null,
      });
    }
    if (!scenario.producer) {
      throw new LifecycleError('repo/evidence_invalid', `${at}.producer must name what captured the evidence`, {
        scenario: scenario.id,
      });
    }
    if (!Array.isArray(scenario.evidence) || scenario.evidence.length === 0) {
      if (scenario.result === 'skipped') continue;
      throw new LifecycleError('repo/evidence_missing', `${at} records no evidence`, { scenario: scenario.id });
    }
    for (const item of scenario.evidence ?? []) {
      const rel = typeof item === 'string' ? item : item.path;
      if (!rel || rel.startsWith('/') || rel.includes('..')) {
        throw new LifecycleError('repo/evidence_invalid', `${at} evidence paths must be inside the attempt directory`, {
          scenario: scenario.id,
        });
      }
      if (!existsSync(join(paths.root, rel))) {
        throw new LifecycleError('repo/evidence_missing', `${at} references missing evidence ${rel}`, {
          scenario: scenario.id,
        });
      }
      scenarioByPath.set(rel, {
        scenario: scenario.id,
        producer: typeof item === 'string' ? scenario.producer : item.producer ?? scenario.producer,
        captured_at: typeof item === 'string' ? null : item.captured_at ?? null,
      });
    }
  }
  return { declared: parsed, scenarioByPath };
}

export function writeManifest(paths, manifest) {
  writeFileSync(paths.manifest, `${JSON.stringify(manifest, null, 2)}\n`, 'utf8');
  return paths.manifest;
}

export function writeJson(path, value) {
  mkdirSync(join(path, '..'), { recursive: true });
  writeFileSync(path, `${JSON.stringify(value, null, 2)}\n`, 'utf8');
  return path;
}
