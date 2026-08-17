#!/usr/bin/env node
// Boot the inventory API, prove it answers on a real route, and release what it started.
//
//   node scripts/init.mjs           start, wait for ready, stop, exit 0
//   node scripts/init.mjs --keep    start, wait for ready, hand off pid/pgid and teardown command
//
// Env: PORT (default: a free ephemeral port), HOST, READY_PATH, READY_TIMEOUT_MS,
//      POLL_INTERVAL_MS, STOP_TIMEOUT_MS.

import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { readFileSync, existsSync, openSync } from 'node:fs';
import { connect, createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import process from 'node:process';

const PKG_ROOT = dirname(dirname(fileURLToPath(import.meta.url)));
const KEEP = process.argv.includes('--keep');
const HOST = process.env.HOST || '127.0.0.1';
const READY_PATH = process.env.READY_PATH || '/items';
const READY_TIMEOUT_MS = num(process.env.READY_TIMEOUT_MS, 30_000);
const POLL_INTERVAL_MS = num(process.env.POLL_INTERVAL_MS, 250);
const REQUEST_TIMEOUT_MS = num(process.env.REQUEST_TIMEOUT_MS, 2_000);
const STOP_TIMEOUT_MS = num(process.env.STOP_TIMEOUT_MS, 5_000);

// Failure taxonomy: repo/* is fixed in this checkout, runner/* on the machine,
// app/* by the application under boot.
const RECOVERY = {
  'repo/manifest_unreadable': 'restore app/package.json',
  'repo/entrypoint_missing': 'restore the file named by "main" in package.json',
  'repo/dependencies_missing': 'run: npm ci (or npm install) in app/',
  'runner/port_unavailable': 'free the port or set PORT to an unused one',
  'runner/start_command_unavailable': 'install npm / put it on PATH for this runner',
  'app/exited_early': 'read the child output above; the app crashed before serving',
  'app/readiness_timeout': `raise READY_TIMEOUT_MS or fix why ${READY_PATH} never answered`,
  'app/readiness_contract': `${READY_PATH} answered but not with the expected JSON array`,
  'app/teardown_failed': 'kill the reported pgid manually before the next run',
};

/** Owned runtime state; only what this attempt raised is ever released. */
const owned = { child: null, pgid: null, port: null, exit: null, spawnError: null, logPath: null };
let stopping = null;

function num(value, fallback) {
  const n = Number.parseInt(value ?? '', 10);
  return Number.isFinite(n) && n > 0 ? n : fallback;
}

function log(event, fields = {}) {
  process.stdout.write(JSON.stringify({ event, ts: new Date().toISOString(), ...fields }) + '\n');
}

class BootError extends Error {
  constructor(kind, detail) {
    super(detail);
    this.kind = kind;
  }
}

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

/** Prerequisites that make booting pointless if absent. Read-only. */
function preflight() {
  const manifestPath = join(PKG_ROOT, 'package.json');
  let manifest;
  try {
    manifest = JSON.parse(readFileSync(manifestPath, 'utf8'));
  } catch (cause) {
    throw new BootError('repo/manifest_unreadable', `${manifestPath}: ${cause.message}`);
  }

  const entrypoint = join(PKG_ROOT, manifest.main ?? 'index.js');
  if (!existsSync(entrypoint)) {
    throw new BootError('repo/entrypoint_missing', entrypoint);
  }

  const missing = Object.keys(manifest.dependencies ?? {}).filter(
    (dep) => !existsSync(join(PKG_ROOT, 'node_modules', dep)),
  );
  if (missing.length > 0) {
    throw new BootError('repo/dependencies_missing', `not installed: ${missing.join(', ')}`);
  }
  return manifest;
}

/** Reserve a port the child can own. Explicit PORT wins; otherwise take an ephemeral one. */
async function choosePort() {
  if (process.env.PORT) {
    const port = num(process.env.PORT, 0);
    if (!port) throw new BootError('runner/port_unavailable', `PORT=${process.env.PORT} is not a port`);
    if (await isListening(port)) {
      throw new BootError('runner/port_unavailable', `${HOST}:${port} is already in use`);
    }
    return port;
  }
  const probe = createServer();
  try {
    probe.listen(0, HOST);
    await once(probe, 'listening');
  } catch (cause) {
    throw new BootError('runner/port_unavailable', `cannot bind on ${HOST}: ${cause.message}`);
  }
  const { port } = probe.address();
  await new Promise((resolve) => probe.close(resolve));
  return port;
}

function isListening(port) {
  return new Promise((resolve) => {
    const socket = connect({ host: HOST, port });
    const done = (result) => {
      socket.destroy();
      resolve(result);
    };
    socket.setTimeout(REQUEST_TIMEOUT_MS);
    socket.once('connect', () => done(true));
    socket.once('timeout', () => done(false));
    socket.once('error', () => done(false));
  });
}

/**
 * Start the app through the package's own `start` script, detached so the npm
 * wrapper and the node server it spawns form one owned process group.
 *
 * With --keep the parent exits while the app lives on, so its output goes to a
 * file: a piped stdio pair would break the moment this process leaves.
 */
function startApp(port) {
  const npm = process.platform === 'win32' ? 'npm.cmd' : 'npm';
  if (KEEP) owned.logPath = join(tmpdir(), `inventory-api-boot-${port}.log`);
  const sink = owned.logPath ? openSync(owned.logPath, 'a') : null;

  const child = spawn(npm, ['run', '--silent', 'start'], {
    cwd: PKG_ROOT,
    env: { ...process.env, PORT: String(port) },
    detached: process.platform !== 'win32',
    stdio: sink ? ['ignore', sink, sink] : ['ignore', 'pipe', 'pipe'],
  });

  owned.child = child;
  owned.port = port;
  owned.pgid = child.pid ?? null;

  child.stdout?.on('data', (chunk) => forward('app.stdout', chunk));
  child.stderr?.on('data', (chunk) => forward('app.stderr', chunk));
  child.once('error', (error) => {
    owned.spawnError = error;
    log('app.spawn_error', { code: error.code, message: error.message });
  });
  child.once('exit', (code, signal) => {
    owned.exit = { code, signal };
    log('app.exit', { pid: child.pid, code, signal });
  });

  log('app.spawn', {
    pid: child.pid,
    pgid: owned.pgid,
    port,
    cwd: PKG_ROOT,
    command: 'npm run start',
    log_path: owned.logPath,
  });
  return child;
}

function forward(event, chunk) {
  for (const line of chunk.toString().split('\n')) {
    if (line.trim()) log(event, { line: line.trimEnd() });
  }
}

/** Poll the shipped route until it answers the readiness contract or the deadline passes. */
async function waitForReady(port) {
  const url = `http://${HOST}:${port}${READY_PATH}`;
  const deadline = Date.now() + READY_TIMEOUT_MS;
  let attempts = 0;
  let lastError = 'no attempt completed';

  while (Date.now() < deadline) {
    if (owned.spawnError) {
      const kind = owned.spawnError.code === 'ENOENT' ? 'runner/start_command_unavailable' : 'app/exited_early';
      throw new BootError(kind, `could not run "npm run start": ${owned.spawnError.message}`);
    }
    if (owned.exit) {
      const { code, signal } = owned.exit;
      throw new BootError('app/exited_early', `child exited (code=${code}, signal=${signal}) before ${READY_PATH} answered`);
    }
    attempts += 1;
    try {
      const res = await fetch(url, { signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS) });
      if (!res.ok) {
        lastError = `HTTP ${res.status}`;
      } else {
        const body = await res.json();
        if (!Array.isArray(body)) {
          throw new BootError('app/readiness_contract', `${READY_PATH} returned ${typeof body}, expected an array`);
        }
        log('app.ready', { url, attempts, status: res.status, items: body.length });
        return { url, attempts };
      }
    } catch (error) {
      if (error instanceof BootError) throw error;
      lastError = error.message;
    }
    await sleep(POLL_INTERVAL_MS);
  }
  throw new BootError('app/readiness_timeout', `${url} not ready within ${READY_TIMEOUT_MS}ms (last: ${lastError})`);
}

/** Signal the owned process group only — never by name — then confirm it is gone. */
async function stopApp() {
  if (stopping) return stopping;
  stopping = (async () => {
    const { child, pgid, port } = owned;
    if (!child || child.pid == null) return { released: true };
    if (owned.exit) return verifyReleased(child.pid, port);

    signalOwned(pgid, child.pid, 'SIGTERM');
    const deadline = Date.now() + STOP_TIMEOUT_MS;
    while (!owned.exit && Date.now() < deadline) await sleep(50);
    if (!owned.exit) {
      log('app.stop.escalate', { pid: child.pid, signal: 'SIGKILL' });
      signalOwned(pgid, child.pid, 'SIGKILL');
      const hard = Date.now() + STOP_TIMEOUT_MS;
      while (!owned.exit && Date.now() < hard) await sleep(50);
    }
    return verifyReleased(child.pid, port);
  })();
  return stopping;
}

function signalOwned(pgid, pid, signal) {
  for (const target of pgid ? [-pgid, pid] : [pid]) {
    try {
      process.kill(target, signal);
      if (target < 0) return; // the group covers the direct child too
    } catch (error) {
      if (error.code !== 'ESRCH' && error.code !== 'EPERM') throw error;
    }
  }
}

/** Grade final state instead of trusting the kill's exit status. */
async function verifyReleased(pid, port) {
  let alive = false;
  try {
    process.kill(pid, 0);
    alive = true;
  } catch {
    alive = false;
  }
  const portBusy = port ? await isListening(port) : false;
  const released = !alive && !portBusy;
  log('app.stop', { pid, port, released, process_alive: alive, port_listening: portBusy });
  return { released, alive, portBusy };
}

function finish(outcome, extra = {}) {
  log('lifecycle', { outcome, keep: KEEP, port: owned.port, ...extra });
}

async function failAndExit(error, startedAt) {
  const kind = error instanceof BootError ? error.kind : 'app/unknown';
  const cleanup = await stopApp().catch((e) => ({ released: false, alive: true, portBusy: false, error: e.message }));
  finish('failed', {
    error_kind: kind,
    detail: error.message,
    recovery: RECOVERY[kind] ?? 'inspect the output above',
    duration_ms: Date.now() - startedAt,
    cleanup_released: cleanup.released !== false,
  });
  process.exit(1);
}

async function main() {
  const startedAt = Date.now();

  // Register teardown before anything can be raised, so every exit path releases.
  for (const signal of ['SIGINT', 'SIGTERM']) {
    process.on(signal, async () => {
      await stopApp().catch(() => {});
      finish('signaled', { signal, duration_ms: Date.now() - startedAt });
      process.exit(128 + (signal === 'SIGINT' ? 2 : 15));
    });
  }

  let port;
  try {
    preflight();
    port = await choosePort();
  } catch (error) {
    return failAndExit(error, startedAt);
  }

  let child;
  try {
    child = startApp(port);
    const ready = await waitForReady(port);

    if (KEEP) {
      child.unref();
      const teardown = owned.pgid ? `kill -TERM -${owned.pgid}` : `kill -TERM ${child.pid}`;
      finish('ready', {
        handoff: { pid: child.pid, pgid: owned.pgid, url: ready.url, log_path: owned.logPath, teardown },
        duration_ms: Date.now() - startedAt,
      });
      process.exit(0);
    }

    const cleanup = await stopApp();
    if (!cleanup.released) {
      // Primary work succeeded; a leaked resource still fails the command.
      finish('failed', {
        error_kind: 'app/teardown_failed',
        detail: `pid ${child.pid} alive=${cleanup.alive} port_listening=${cleanup.portBusy}`,
        recovery: RECOVERY['app/teardown_failed'],
        duration_ms: Date.now() - startedAt,
      });
      process.exit(1);
    }
    finish('ready', { url: ready.url, attempts: ready.attempts, duration_ms: Date.now() - startedAt });
    process.exit(0);
  } catch (error) {
    return failAndExit(error, startedAt);
  }
}

main();
