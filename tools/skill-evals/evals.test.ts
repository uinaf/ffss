// Unit tests for the pure seams: flag parsing, run naming, scorecard reduction.
// Run: npm test (node --test; no framework).
import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import test from "node:test";
import { parseArgs, parseMaxTurns, reduceResults } from "./evals.ts";
import { runNameFor } from "./scenario.ts";

test("parseArgs: positionals, value flags, booleans", () => {
  const { positional, flags } = parseArgs(["dir", "--agent", "m1", "--judge", "m2", "--harness", "codex", "--all"]);
  assert.deepEqual(positional, ["dir"]);
  assert.equal(flags.get("--agent"), "m1");
  assert.equal(flags.get("--judge"), "m2");
  assert.equal(flags.get("--harness"), "codex");
  assert.equal(flags.get("--all"), true);
});

test("parseArgs: a flag cannot consume a following flag as its value", () => {
  assert.throws(() => parseArgs(["--agent", "--all"]), /--agent needs a value/);
  assert.throws(() => parseArgs(["--judge"]), /--judge needs a value/);
});

test("parseArgs: unknown flags are rejected", () => {
  assert.throws(() => parseArgs(["--nope"]), /unknown flag: --nope/);
});

test("runNameFor: harness-aware result names", () => {
  const dir = "/repo/skills/slopspec/evals/single-item-minimality";
  assert.equal(runNameFor(dir, "claude"), "slopspec--single-item-minimality");
  assert.equal(runNameFor(dir, "codex"), "slopspec--single-item-minimality--codex");
  assert.throws(() => runNameFor("/repo/not-a-scenario", "claude"), /not a scenario dir/);
});

function writeResult(dir: string, name: string, score: number, success: boolean, sha?: string): void {
  const result = {
    results: { results: [{ score, success, latencyMs: 1200, tokenUsage: { total: 100, assertions: { total: 40 } } }] },
    config: {
      providers: [{ config: { model: "agent-model" } }],
      defaultTest: { options: { provider: "anthropic:messages:judge-model" } },
    },
  };
  fs.writeFileSync(path.join(dir, `${name}.json`), JSON.stringify(result));
  if (sha !== undefined) {
    fs.writeFileSync(path.join(dir, `${name}.meta.json`), JSON.stringify({ skills_tree_sha: sha }));
  }
}

test("reduceResults: valid, malformed, and unattested results", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "skill-evals-test-"));
  writeResult(dir, "skillx--scen-a", 0.9, true, "sha1");
  writeResult(dir, "skillx--scen-b--codex", 0.4, false); // no sidecar → unattested... but mixed with sha1
  fs.writeFileSync(path.join(dir, "broken.json"), "{not json");
  fs.writeFileSync(path.join(dir, "not-a-result.json"), JSON.stringify({ foo: 1 }));

  // sha1 + unattested = mixed → throws without allowMixed
  assert.throws(() => reduceResults(dir, false), /multiple skills-tree revisions/);

  const mixed = reduceResults(dir, true);
  assert.equal(mixed.treeSha, "mixed");
  assert.deepEqual(mixed.skipped.sort(), ["broken.json", "not-a-result.json"]);
  assert.equal(mixed.entries.length, 2);

  const a = mixed.entries.find((e) => e.scenario === "scen-a");
  assert.deepEqual(a, {
    skill: "skillx",
    scenario: "scen-a",
    harness: "claude",
    skills_tree_sha: "sha1",
    score: 0.9,
    pass: true,
    agent_model: "agent-model",
    judge_model: "judge-model",
    latency_ms: 1200,
    tokens: 140,
  });
  const b = mixed.entries.find((e) => e.scenario === "scen-b");
  assert.equal(b?.harness, "codex");
  assert.equal(b?.skills_tree_sha, "unattested");

  // Uniform shas reduce cleanly without allowMixed
  fs.writeFileSync(path.join(dir, "skillx--scen-b--codex.meta.json"), JSON.stringify({ skills_tree_sha: "sha1" }));
  const uniform = reduceResults(dir, false);
  assert.equal(uniform.treeSha, "sha1");
  assert.equal(uniform.entries.length, 2);

  fs.rmSync(dir, { recursive: true, force: true });
});

test("reduceResults: empty directory", () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "skill-evals-test-"));
  const r = reduceResults(dir, false);
  assert.equal(r.treeSha, "none");
  assert.deepEqual(r.entries, []);
  assert.deepEqual(r.skipped, []);
  fs.rmSync(dir, { recursive: true, force: true });
});
test("parseMaxTurns validates values", () => {
  assert.equal(parseMaxTurns("80"), 80);
  assert.throws(() => parseMaxTurns("abc"));
  assert.throws(() => parseMaxTurns("-3"));
  assert.throws(() => parseMaxTurns("Infinity"));
  assert.throws(() => parseMaxTurns("2.5"));
});
