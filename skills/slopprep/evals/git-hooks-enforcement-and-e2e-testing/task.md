# Hardening a TypeScript CLI Tool for Agent Collaboration

## Problem/Feature Description

A small team uses a TypeScript CLI that converts CSV files to JSON. Agents keep
pushing broken builds because the existing test mocks the implementation and
never runs the built CLI. The team wants one repository-owned verification
entrypoint, a pre-push hook that delegates to it, honest end-to-end coverage,
and dead-code detection.

Use the project's existing package scripts and Jest surface instead of building
a second validation framework in shell. The Git hook may be a tiny executable
adapter, but it must not duplicate the command graph or implement assertions,
parsing, retries, or policy.

## Output Specification

Produce the following files:

1. `package.json` — Keep the existing commands and add repository-owned,
   version-pinned dead-code detection plus one canonical verification command.
2. `.git-hooks/pre-push` — An executable thin adapter that delegates to the
   canonical verification command.
3. `e2e/cli.test.ts` — A Jest integration test that invokes the actual built CLI
   as a child process against real files and compares structured output with the
   expected fixture. Do not import or mock `parseCsv`.
4. `e2e/fixtures/sample.csv` — Sample CSV input used by the end-to-end test.
5. `e2e/fixtures/expected.json` — Expected JSON output for the sample.
6. Any focused Jest or TypeScript configuration required to run the test.
7. `setup-notes.md` — Briefly explain hook activation, the canonical gate, the
   dead-code owner, and what the real-process test proves.

Do not create a shell test, a second command graph, or a wrapper whose only job
is to replay package scripts.

## Input Files

The following files represent the current state of the project. Extract them before beginning.

=============== FILE: package.json ===============
{
  "name": "csv-to-json-cli",
  "version": "2.1.0",
  "description": "Convert CSV files to JSON",
  "bin": {
    "csv2json": "./dist/cli.js"
  },
  "scripts": {
    "build": "tsc",
    "lint": "eslint src/",
    "test": "jest"
  },
  "dependencies": {},
  "devDependencies": {
    "typescript": "^5.0.0",
    "eslint": "^8.0.0",
    "@typescript-eslint/parser": "^6.0.0",
    "jest": "^29.0.0",
    "ts-jest": "^29.0.0"
  }
}

=============== FILE: src/cli.ts ===============
#!/usr/bin/env node
import { readFileSync, writeFileSync } from 'fs';
import { parseCsv } from './parser';

const [,, inputFile, outputFile] = process.argv;

if (!inputFile) {
  console.error('Usage: csv2json <input.csv> [output.json]');
  process.exit(1);
}

const csv = readFileSync(inputFile, 'utf-8');
const result = parseCsv(csv);

if (outputFile) {
  writeFileSync(outputFile, JSON.stringify(result, null, 2));
} else {
  console.log(JSON.stringify(result, null, 2));
}

=============== FILE: src/parser.ts ===============
export function parseCsv(csv: string): Record<string, string>[] {
  const lines = csv.trim().split('\n');
  const headers = lines[0].split(',').map(h => h.trim());
  return lines.slice(1).map(line => {
    const values = line.split(',').map(v => v.trim());
    return Object.fromEntries(headers.map((h, i) => [h, values[i] ?? '']));
  });
}

export function legacyParseCsv(csv: string): string[][] {
  // Old implementation kept for reference but no longer used
  return csv.split('\n').map(line => line.split(','));
}

=============== FILE: src/parser.test.ts ===============
import { parseCsv } from './parser';

jest.mock('./parser', () => ({
  parseCsv: jest.fn(),
  legacyParseCsv: jest.fn()
}));

test('parseCsv is a function', () => {
  expect(typeof parseCsv).toBe('function');
});

=============== FILE: tsconfig.json ===============
{
  "compilerOptions": {
    "target": "ES2020",
    "module": "commonjs",
    "outDir": "./dist",
    "strict": true
  },
  "include": ["src/**/*"]
}
