#!/usr/bin/env node
/**
 * `pnpm own` — register a runtime resource this attempt raised, so teardown can
 * release exactly it and nothing else.
 *
 *   pnpm own record --kind process_group --id 4321 --label dev-server
 *   pnpm own record --kind container --id 9ab3f1 --runtime docker
 *   pnpm own record --kind vm --id builder-1 \
 *     --release '["vmctl","stop","builder-1"]' --verify '["vmctl","is-stopped","builder-1"]'
 *   pnpm own list
 *
 * A persistent resource must be recorded before it is driven. Anything not in the
 * ledger is treated as pre-existing and is preserved.
 */

import { EXIT, LifecycleError, printEvent, printFailure } from './failure.mjs';
import { parseRunnerContract } from './contract.mjs';
import { artifactPaths, ensureArtifactDirs } from './artifacts.mjs';
import { RESOURCE_KINDS, readLedger, recordResource, releasable, readSnapshot } from './resources.mjs';

const FLAGS = ['--kind', '--id', '--pgid', '--label', '--scope', '--runtime', '--release', '--verify'];

function parseArgs(argv) {
  const options = {};
  for (let index = 0; index < argv.length; index += 1) {
    const token = argv[index];
    // Split on the first '=' only: --release='["a","b=c"]' must survive intact.
    const split = token.indexOf('=');
    const flag = split === -1 ? token : token.slice(0, split);
    const inline = split === -1 ? undefined : token.slice(split + 1);
    if (!FLAGS.includes(flag)) {
      throw new LifecycleError('repo/contract_violation', `unknown flag: ${flag}`, { known_flags: FLAGS });
    }
    const value = inline ?? argv[index + 1];
    if (inline === undefined) index += 1;
    if (value === undefined) {
      throw new LifecycleError('repo/contract_violation', `${flag} needs a value`, {});
    }
    options[flag.slice(2)] = value;
  }
  return options;
}

function jsonArgv(raw, flag) {
  if (raw === undefined) return undefined;
  let parsed;
  try {
    parsed = JSON.parse(raw);
  } catch {
    throw new LifecycleError('repo/contract_violation', `${flag} must be a JSON array of argv strings`, {});
  }
  if (!Array.isArray(parsed) || parsed.some((item) => typeof item !== 'string')) {
    throw new LifecycleError('repo/contract_violation', `${flag} must be a JSON array of argv strings`, {});
  }
  return parsed;
}

function main() {
  const [subcommand, ...rest] = process.argv.slice(2);
  const contract = parseRunnerContract(process.env);
  const paths = ensureArtifactDirs(artifactPaths(contract, process.cwd()));

  if (subcommand === 'list') {
    const outstanding = releasable(readLedger(paths.ledger), { preexisting: readSnapshot(paths.snapshot).items });
    process.stdout.write(`${JSON.stringify({ outstanding }, null, 2)}\n`);
    return EXIT.ok;
  }

  if (subcommand !== 'record') {
    throw new LifecycleError('repo/contract_violation', 'usage: pnpm own record --kind <kind> --id <id> | pnpm own list', {
      known_kinds: RESOURCE_KINDS,
    });
  }

  const options = parseArgs(rest);
  const entry = recordResource(paths.ledger, {
    kind: options.kind,
    id: options.id,
    pgid: options.pgid ? Number(options.pgid) : null,
    label: options.label ?? null,
    scope: options.scope ?? 'attempt',
    runtime: options.runtime,
    release: jsonArgv(options.release, '--release'),
    verify: jsonArgv(options.verify, '--verify'),
  });
  printEvent({
    event: 'lifecycle.resource_recorded',
    operation: 'own.record',
    resource_id: `${entry.kind}:${entry.id}`,
    outcome: 'recorded',
    error_kind: null,
    scope: entry.scope,
    teardown: 'pnpm teardown',
  });
  return EXIT.ok;
}

try {
  process.exit(main());
} catch (error) {
  const failure = printFailure('own', error);
  process.exit(failure.exitCode);
}
