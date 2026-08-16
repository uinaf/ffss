#!/usr/bin/env node
// Skill-eval CLI. See README.md for usage; scenario.ts does the fixture work.
//
//   node evals.ts run <scenario-dir> [--agent MODEL] [--judge MODEL] [--harness claude|codex]
//   node evals.ts sweep [--all]
//   node evals.ts summarize

import { execFileSync, spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { generateRun, type Harness, type RunOptions } from "./scenario.ts";

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, "../..");
const resultsDir = path.join(here, "results");

function fail(msg: string): never {
  console.error(msg);
  process.exit(1);
}

function parseArgs(argv: string[]): { positional: string[]; flags: Map<string, string | true> } {
  const positional: string[] = [];
  const flags = new Map<string, string | true>();
  const takesValue = new Set(["--agent", "--judge", "--harness"]);
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (!a.startsWith("--")) {
      positional.push(a);
    } else if (takesValue.has(a)) {
      const v = argv[++i];
      if (v === undefined) fail(`${a} needs a value`);
      flags.set(a, v);
    } else if (a === "--all") {
      flags.set(a, true);
    } else {
      fail(`unknown flag: ${a}`);
    }
  }
  return { positional, flags };
}

function runOptions(flags: Map<string, string | true>): RunOptions {
  const harness = (flags.get("--harness") ?? "claude") as string;
  if (harness !== "claude" && harness !== "codex") fail(`--harness must be claude or codex, got ${harness}`);
  const agent = flags.get("--agent") as string | undefined;
  return {
    harness: harness as Harness,
    // claude defaults in scenario.ts; codex undefined = current Codex CLI default
    agentModel: agent,
    judgeModel: (flags.get("--judge") as string | undefined) ?? "claude-opus-5",
  };
}

interface RunOutcome {
  name: string;
  rc: number;
  resultPath: string;
  score?: number;
  pass?: boolean;
}

function runScenario(scenarioDir: string, opts: RunOptions): RunOutcome {
  const { name, configPath } = generateRun(path.resolve(scenarioDir), opts, here);
  fs.mkdirSync(resultsDir, { recursive: true });
  const resultPath = path.join(resultsDir, `${name}.json`);
  const r = spawnSync(
    "npx",
    ["promptfoo", "eval", "--no-cache", "--no-progress-bar", "-c", configPath, "-o", resultPath],
    { cwd: here, stdio: "inherit" },
  );
  const rc = r.status ?? 1; // null status (signal) counts as failure
  const outcome: RunOutcome = { name, rc, resultPath };
  try {
    const res = JSON.parse(fs.readFileSync(resultPath, "utf8")).results.results[0];
    outcome.score = res.score;
    outcome.pass = res.success;
  } catch {
    // no result written — leave score/pass undefined; caller reports ERROR
  }
  return outcome;
}

function discoverScenarios(): string[] {
  const roots = [path.join(repoRoot, "skills")];
  const cliDir = path.join(repoRoot, "cli");
  if (fs.existsSync(cliDir)) {
    for (const e of fs.readdirSync(cliDir, { withFileTypes: true })) {
      if (e.isDirectory()) roots.push(path.join(cliDir, e.name, "skills"));
    }
  }
  const found: string[] = [];
  for (const root of roots) {
    if (!fs.existsSync(root)) continue;
    for (const skill of fs.readdirSync(root, { withFileTypes: true })) {
      const evalsDir = path.join(root, skill.name, "evals");
      if (!skill.isDirectory() || !fs.existsSync(evalsDir)) continue;
      for (const sc of fs.readdirSync(evalsDir, { withFileTypes: true })) {
        const dir = path.join(evalsDir, sc.name);
        if (sc.isDirectory() && fs.existsSync(path.join(dir, "task.md")) && fs.existsSync(path.join(dir, "criteria.json"))) {
          found.push(dir);
        }
      }
    }
  }
  return found.sort();
}

function cmdRun(argv: string[]): void {
  const { positional, flags } = parseArgs(argv);
  if (positional.length !== 1) fail("usage: evals.ts run <scenario-dir> [--agent MODEL] [--judge MODEL] [--harness claude|codex]");
  const o = runScenario(positional[0], runOptions(flags));
  if (o.score === undefined) fail(`ERROR ${o.name}: promptfoo rc=${o.rc}, no result written`);
  console.log(`${o.pass ? "PASS" : "FAIL"} ${o.name} score=${o.score.toFixed(4)} (results: ${o.resultPath})`);
  process.exit(o.pass ? 0 : Math.max(o.rc, 1));
}

function cmdSweep(argv: string[]): void {
  const { positional, flags } = parseArgs(argv);
  if (positional.length > 0) fail("usage: evals.ts sweep [--all]");
  const opts = runOptions(flags);
  const all = flags.get("--all") === true;
  let passed = 0, failed = 0, errored = 0, skipped = 0;
  for (const dir of discoverScenarios()) {
    const name = dir.replace(/^.*skills\/([^/]+)\/evals\//, "$1--");
    const resultPath = path.join(resultsDir, `${name}.json`);
    if (!all && fs.existsSync(resultPath)) {
      skipped++;
      console.log(`SKIP  ${name} (results exist; use --all to rerun)`);
      continue;
    }
    const o = runScenario(dir, opts);
    if (o.score === undefined) {
      errored++;
      console.log(`ERROR ${o.name} promptfoo rc=${o.rc}, no result written`);
    } else if (o.pass) {
      passed++;
      console.log(`PASS  ${o.name} score=${o.score.toFixed(4)}`);
    } else {
      failed++;
      console.log(`FAIL  ${o.name} score=${o.score.toFixed(4)}`);
    }
  }
  console.log(`\nsweep: ${passed} passed, ${failed} failed, ${errored} errored, ${skipped} skipped`);
  process.exit(errored > 0 ? 1 : 0);
}

interface ScorecardEntry {
  skill: string;
  scenario: string;
  harness: Harness;
  score: number;
  pass: boolean;
  agent_model: string;
  judge_model: string;
  latency_ms: number;
  tokens: number;
}

function cmdSummarize(): void {
  if (!fs.existsSync(resultsDir)) fail("no results/ directory — run some evals first");
  const entries: ScorecardEntry[] = [];
  for (const f of fs.readdirSync(resultsDir).filter((f) => f.endsWith(".json")).sort()) {
    const raw = JSON.parse(fs.readFileSync(path.join(resultsDir, f), "utf8"));
    const res = raw.results.results[0];
    const provider = raw.config.providers[0];
    const judge = raw.config.defaultTest?.options?.provider;
    const base = f.replace(/\.json$/, "");
    const harness: Harness = base.endsWith("--codex") ? "codex" : "claude";
    const [skill, ...rest] = base.replace(/--codex$/, "").split("--");
    entries.push({
      skill,
      scenario: rest.join("--"),
      harness,
      score: res.score,
      pass: res.success,
      agent_model: provider?.config?.model ?? "codex-default",
      judge_model: typeof judge === "string" ? judge.replace(/^anthropic:messages:/, "") : (judge?.config?.model ?? "unknown"),
      latency_ms: res.latencyMs,
      tokens: (res.tokenUsage?.total ?? 0) + (res.tokenUsage?.assertions?.total ?? 0),
    });
  }
  const sha = execFileSync("git", ["rev-parse", "HEAD"], { cwd: repoRoot, encoding: "utf8" }).trim();
  const scorecard = { ran_at: new Date().toISOString(), skills_tree_sha: sha, scenarios: entries };
  const outDir = path.join(here, "scorecards");
  fs.mkdirSync(outDir, { recursive: true });
  const out = path.join(outDir, `${new Date().toISOString().slice(0, 10)}.json`);
  fs.writeFileSync(out, JSON.stringify(scorecard, null, 2) + "\n");
  console.log(`${out}: ${entries.length} scenario(s), ${entries.filter((e) => e.pass).length} passing`);
}

const [cmd, ...rest] = process.argv.slice(2);
if (cmd === "run") cmdRun(rest);
else if (cmd === "sweep") cmdSweep(rest);
else if (cmd === "summarize") cmdSummarize();
else fail("usage: evals.ts <run|sweep|summarize> ...");
