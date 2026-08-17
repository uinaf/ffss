import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';

// This suite consumes the shipped artifact only: it spawns `dist/cli.js` as a
// real child process against real files. It must never import or mock parseCsv,
// because a test that reaches inside the module cannot notice a broken build,
// a bad bin path, or a compile error.

const repoRoot = resolve(__dirname, '..');
const cliPath = join(repoRoot, 'dist', 'cli.js');
const sampleCsvPath = join(__dirname, 'fixtures', 'sample.csv');
const expectedRows: unknown = JSON.parse(
  readFileSync(join(__dirname, 'fixtures', 'expected.json'), 'utf-8'),
);

interface RunResult {
  status: number | null;
  stdout: string;
  stderr: string;
}

function runCli(args: string[], cwd: string): RunResult {
  const result = spawnSync(process.execPath, [cliPath, ...args], {
    cwd,
    encoding: 'utf-8',
    timeout: 10_000,
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  if (result.error) {
    throw result.error;
  }
  return { status: result.status, stdout: result.stdout, stderr: result.stderr };
}

let workDir: string;

beforeAll(() => {
  if (!existsSync(cliPath)) {
    throw new Error(
      `built CLI missing at ${cliPath}. Run \`npm run build\` first, ` +
        'or use `npm run verify`, which builds before testing.',
    );
  }
});

beforeEach(() => {
  // Every case gets its own scratch directory so output-file cases cannot
  // observe each other and nothing is written into the checkout.
  workDir = mkdtempSync(join(tmpdir(), 'csv2json-e2e-'));
});

afterEach(() => {
  rmSync(workDir, { recursive: true, force: true });
});

describe('csv2json (built CLI, real process)', () => {
  test('prints the parsed rows to stdout and exits 0', () => {
    const { status, stdout, stderr } = runCli([sampleCsvPath], workDir);

    expect({ status, stderr }).toEqual({ status: 0, stderr: '' });
    expect(JSON.parse(stdout)).toEqual(expectedRows);
    // Header whitespace trimmed, and the short third row is padded, not dropped.
    expect(stdout.trim()).toBe(JSON.stringify(expectedRows, null, 2));
  });

  test('writes the same structure to the requested output file', () => {
    const outputPath = join(workDir, 'out.json');

    const { status, stdout, stderr } = runCli([sampleCsvPath, outputPath], workDir);

    expect({ status, stdout, stderr }).toEqual({ status: 0, stdout: '', stderr: '' });
    expect(existsSync(outputPath)).toBe(true);

    const written = readFileSync(outputPath, 'utf-8');
    expect(JSON.parse(written)).toEqual(expectedRows);
    expect(written).toBe(JSON.stringify(expectedRows, null, 2));
  });

  test('exits 1 with usage on stderr when no input file is given', () => {
    const { status, stdout, stderr } = runCli([], workDir);

    expect(status).toBe(1);
    expect(stderr).toContain('Usage: csv2json <input.csv> [output.json]');
    expect(stdout).toBe('');
  });

  test('exits non-zero and names the missing path, writing no output file', () => {
    const missingInput = join(workDir, 'does-not-exist.csv');
    const outputPath = join(workDir, 'out.json');

    const { status, stdout, stderr } = runCli([missingInput, outputPath], workDir);

    expect(status).not.toBe(0);
    expect(stderr).toContain('ENOENT');
    expect(stderr).toContain(missingInput);
    expect(stdout).toBe('');
    // A failed read must not leave a partial or empty artifact behind.
    expect(existsSync(outputPath)).toBe(false);
  });
});
