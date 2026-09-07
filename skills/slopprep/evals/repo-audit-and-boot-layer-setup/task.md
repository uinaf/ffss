# Getting a Legacy Node.js API Ready for Autonomous Development

## Problem/Feature Description

Your team inherited a Node.js REST API from a contractor. Nobody has run it
locally; the README says only "run npm start." There is no health endpoint, no
reliable way to know the app is alive after starting, and CI has been broken
for weeks. Agents should start working on features autonomously, but none can
tell whether its changes broke anything.

Assess the repo and add the minimum infrastructure for an agent to reliably
start the app and verify it is alive. Keep lifecycle logic in the project's
JavaScript runtime and expose it through the package manifest; do not invent a
shell validation framework.

## Output Specification

Produce the following files:

1. `audit-report.md` — Assessment of the repository from an autonomous-agent
   perspective. The declared runner is a clean local Node.js environment with
   network access for package installation; no external credentials are
   required. Grade repository and runner readiness separately across
   legibility, executability, feedback, safety, durability, and scale. For each
   applicable capability, cite concrete evidence and describe the gap and
   owner. Include an E0-through-E4 evidence level and derive each headline
   grade from its lowest applicable capability.

2. `app/scripts/init.mjs` — A Node.js lifecycle program that starts the
   application as a child process, polls a real readiness signal with a bounded
   deadline, and exits non-zero after cleaning up if startup fails.

3. `app/package.json` — Preserve the existing scripts and add one package-owned
   command for the lifecycle program.

4. `improvements.md` — Which readiness capabilities you addressed, which
   remain missing toward B and A, and recommended next steps. Do not treat
   reaching C as completion.

## Input Files

The inherited repository. Extract before beginning.

=============== FILE: app/package.json ===============
{
  "name": "legacy-inventory-api",
  "version": "1.0.0",
  "description": "Inventory management REST API",
  "main": "src/index.js",
  "scripts": {
    "start": "node src/index.js",
    "test": "jest"
  },
  "dependencies": {
    "express": "^4.18.2"
  },
  "devDependencies": {
    "jest": "^29.0.0"
  }
}

=============== FILE: app/src/index.js ===============
const express = require('express');
const app = express();
const PORT = process.env.PORT || 3000;

app.use(express.json());

const items = [];

app.get('/items', (req, res) => {
  res.json(items);
});

app.post('/items', (req, res) => {
  const item = { id: Date.now(), ...req.body };
  items.push(item);
  res.status(201).json(item);
});

app.listen(PORT, () => {
  console.log(`Server running on port ${PORT}`);
});

=============== FILE: app/test/items.test.js ===============
const { createItem, getItems } = require('../src/items');

jest.mock('../src/items');

test('createItem returns an object', () => {
  createItem.mockReturnValue({ id: 1, name: 'test' });
  const result = createItem({ name: 'test' });
  expect(result).toEqual({ id: 1, name: 'test' });
});

test('getItems returns array', () => {
  getItems.mockReturnValue([]);
  expect(getItems()).toEqual([]);
});

=============== FILE: app/README.md ===============
# Inventory API

Run `npm start` to start the server.
