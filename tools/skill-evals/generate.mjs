#!/usr/bin/env node
// Convert one skill-eval scenario (task.md + criteria.json) into a promptfoo run.
//
// Usage: node generate.mjs <scenario-dir> [agent-model] [judge-model]
//   e.g. node generate.mjs ../../skills/slopspec/evals/single-item-minimality
//
// Produces scratch/<skill>--<scenario>/ containing:
//   workdir/            input files from task.md + the skill at .claude/skills/<skill>/
//   manifest.json       hashes of pre-existing workdir files (transform.mjs diffs against it)
//   promptfooconfig.json  one llm-rubric assertion per checklist item, weight = max_score
//
// Run: npx promptfoo eval -c scratch/<name>/promptfooconfig.json --no-cache -o results/<name>.json

import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const scenarioDir = path.resolve(process.argv[2] ?? "");
const agentModel = process.argv[3] ?? "claude-opus-5";
const judgeModel = process.argv[4] ?? "claude-opus-5";

const match = scenarioDir.match(/skills\/([^/]+)\/evals\/([^/]+)$/);
if (!match) {
  console.error("usage: generate.mjs <repo>/skills/<skill>/evals/<scenario>");
  process.exit(1);
}
const [, skill, scenario] = match;
const skillDir = path.resolve(scenarioDir, "../..");

const taskMd = fs.readFileSync(path.join(scenarioDir, "task.md"), "utf8");
const criteria = JSON.parse(fs.readFileSync(path.join(scenarioDir, "criteria.json"), "utf8"));
if (
  criteria.type !== "weighted_checklist" ||
  !Array.isArray(criteria.checklist) ||
  criteria.checklist.length === 0
) {
  console.error(`unsupported or empty criteria in ${scenarioDir}`);
  process.exit(1);
}
for (const item of criteria.checklist) {
  const ok =
    typeof item?.name === "string" && item.name.trim() !== "" &&
    typeof item?.description === "string" && item.description.trim() !== "" &&
    Number.isFinite(item?.max_score) && item.max_score > 0;
  if (!ok) {
    console.error(`invalid checklist item in ${scenarioDir}: ${JSON.stringify(item)}`);
    process.exit(1);
  }
}

// Extract embedded input files; replace each block with a pointer to the file on disk.
const files = [];
const fileBlock = /^=+ FILE: (.+?) =+\n([\s\S]*?)\n=+ END FILE =+$/gm;
const prompt = taskMd.replace(fileBlock, (_, name, content) => {
  files.push({ name: name.trim(), content: content + "\n" });
  return `(Input file \`${name.trim()}\` is available in your working directory.)`;
});

// Materialize the scratch run directory.
const runDir = path.join(here, "scratch", `${skill}--${scenario}`);
const workdir = path.join(runDir, "workdir");
fs.rmSync(runDir, { recursive: true, force: true });
fs.mkdirSync(workdir, { recursive: true });

// Validate every embedded filename before writing anything: destinations must
// stay strictly below workdir, must not land under .claude/ (a fixture could
// inject project settings that setting_sources: ['project'] would load), and
// must not collide.
const seen = new Set();
const planned = files.map((f) => {
  const dest = path.resolve(workdir, f.name);
  const rel = path.relative(workdir, dest);
  if (rel === "" || rel.startsWith("..") || path.isAbsolute(rel)) {
    console.error(`embedded file escapes workdir: ${f.name}`);
    process.exit(1);
  }
  if (rel.split(path.sep)[0] === ".claude") {
    console.error(`embedded file targets .claude/: ${f.name}`);
    process.exit(1);
  }
  if (seen.has(dest)) {
    console.error(`duplicate embedded file: ${f.name}`);
    process.exit(1);
  }
  seen.add(dest);
  return { dest, content: f.content };
});
for (const { dest, content } of planned) {
  fs.mkdirSync(path.dirname(dest), { recursive: true });
  fs.writeFileSync(dest, content);
}

// Install the skill under test at .claude/skills/<skill>/, excluding its evals
// (criteria must not leak into the agent's context).
const skillDest = path.join(workdir, ".claude", "skills", skill);
fs.cpSync(skillDir, skillDest, {
  recursive: true,
  filter: (src) => path.basename(src) !== "evals",
});

// Manifest of pre-existing files so transform.mjs can find what the agent wrote.
const manifest = {};
const walk = (dir) => {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (e.name !== ".claude") walk(p);
    } else {
      manifest[path.relative(workdir, p)] = createHash("sha256").update(fs.readFileSync(p)).digest("hex");
    }
  }
};
walk(workdir);
fs.writeFileSync(path.join(runDir, "manifest.json"), JSON.stringify(manifest, null, 2));

const config = {
  description: `${skill}/${scenario}`,
  prompts: ["{{task}}"],
  providers: [
    {
      id: "anthropic:claude-agent-sdk",
      config: {
        model: agentModel,
        // No ANTHROPIC_API_KEY in this environment; use the local Claude Code
        // session (documented promptfoo path for subscription auth).
        apiKeyRequired: false,
        working_dir: workdir,
        setting_sources: ["project"],
        skills: [skill],
        permission_mode: "acceptEdits",
        append_allowed_tools: ["Read", "Write", "Edit", "Glob", "Grep"],
        max_turns: 50,
      },
    },
  ],
  defaultTest: {
    options: {
      // With ANTHROPIC_API_KEY set, grade over the plain messages API;
      // otherwise grade through the agent SDK provider with local Claude
      // Code session auth. The SDK judge needs a forced verdict schema —
      // the messages judge relies on promptfoo's own rubric JSON prompt.
      provider: process.env.ANTHROPIC_API_KEY
        ? `anthropic:messages:${judgeModel}`
        : {
            id: "anthropic:claude-agent-sdk",
            config: {
              model: judgeModel,
              apiKeyRequired: false,
              max_turns: 3,
              // llm-rubric parses a JSON verdict; force the judge to emit exactly that.
              output_format: {
                type: "json_schema",
                schema: {
                  type: "object",
                  additionalProperties: false,
                  required: ["reason", "pass", "score"],
                  properties: {
                    reason: { type: "string" },
                    pass: { type: "boolean" },
                    score: { type: "number", minimum: 0, maximum: 1 },
                  },
                },
              },
            },
          },
      transform: `file://${path.join(here, "transform.mjs")}`,
    },
  },
  tests: [
    {
      description: criteria.context,
      vars: { task: prompt, workdir, manifest: path.join(runDir, "manifest.json") },
      // No test-level threshold: the test passes only if every top-level
      // assertion passes — the checklist assert-set (weighted score >= its own
      // threshold) AND the mandatory skill-used routing check, which stays
      // outside the weighted aggregate so it can neither dilute the checklist
      // denominator nor be outvoted by a perfect checklist.
      assert: [
        {
          type: "assert-set",
          threshold: 0.7,
          assert: criteria.checklist.map((item) => ({
            type: "llm-rubric",
            value: `${item.name}: ${item.description}`,
            weight: item.max_score,
          })),
        },
        { type: "skill-used", value: skill },
      ],
    },
  ],
};

const configPath = path.join(runDir, "promptfooconfig.json");
fs.writeFileSync(configPath, JSON.stringify(config, null, 2));
console.log(configPath);
