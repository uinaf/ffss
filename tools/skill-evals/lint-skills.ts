#!/usr/bin/env node
// Lint every skill package: frontmatter contract, name/dir agreement, and
// resolvable relative links. Zero dependencies; node 24+ runs it natively.
//
// Usage: node tools/skill-evals/lint-skills.ts   (from the repo root)

import fs from "node:fs";
import path from "node:path";

const ALLOWED_KEYS = new Set(["name", "description", "disable-model-invocation"]);
const errors: string[] = [];

function lintSkill(dir: string): void {
  const skillMd = path.join(dir, "SKILL.md");
  if (!fs.existsSync(skillMd)) {
    errors.push(`${dir}: missing SKILL.md`);
    return;
  }
  const text = fs.readFileSync(skillMd, "utf8");
  const lines = text.split("\n");
  if (lines[0] !== "---") {
    errors.push(`${skillMd}: frontmatter must open with --- on line 1`);
    return;
  }
  const close = lines.indexOf("---", 1);
  if (close === -1) {
    errors.push(`${skillMd}: frontmatter never closes`);
    return;
  }

  // Raw tokens are kept alongside unquoted values: scalar type matters
  // (disable-model-invocation must be the bare YAML boolean, not "true").
  const fields = new Map<string, { raw: string; value: string }>();
  for (const line of lines.slice(1, close)) {
    const m = line.match(/^([a-z-]+):\s*(.*)$/);
    if (!m) {
      errors.push(`${skillMd}: unparseable frontmatter line: ${line}`);
      continue;
    }
    const [, key, raw] = m;
    if (!ALLOWED_KEYS.has(key)) {
      errors.push(`${skillMd}: unknown frontmatter key: ${key}`);
      continue;
    }
    if (fields.has(key)) {
      errors.push(`${skillMd}: duplicate frontmatter key: ${key}`);
      continue;
    }
    const value = raw.replace(/^"(.*)"$/s, "$1").replace(/^'(.*)'$/s, "$1").trim();
    fields.set(key, { raw: raw.trim(), value });
  }

  const name = fields.get("name")?.value ?? "";
  if (name !== path.basename(dir)) {
    errors.push(`${skillMd}: frontmatter name ${JSON.stringify(name)} != directory ${path.basename(dir)}`);
  }
  if (!fields.get("description")?.value) {
    errors.push(`${skillMd}: description is required and must be non-empty`);
  }
  const dmi = fields.get("disable-model-invocation");
  if (dmi !== undefined && dmi.raw !== "true") {
    errors.push(`${skillMd}: disable-model-invocation must be the literal boolean true, got ${JSON.stringify(dmi.raw)}`);
  }

  // Relative links in the body must resolve; external and anchor links pass.
  // Code spans and fences are stripped first so example links never lint, and
  // optional link titles ("...") are parsed rather than hiding the target.
  const body = lines
    .slice(close + 1)
    .join("\n")
    .replace(/^```[\s\S]*?^```/gm, "")
    .replace(/`[^`\n]*`/g, "");
  for (const link of body.matchAll(/\]\(\s*(<[^>\n]*>|[^)\s]+)(?:\s+"[^"\n]*")?\s*\)/g)) {
    const target = link[1].replace(/^<(.*)>$/, "$1");
    if (/^[a-z][a-z+.-]*:/.test(target) || target.startsWith("#") || target === "") continue;
    const resolved = path.join(dir, target.split("#")[0]);
    if (!fs.existsSync(resolved)) {
      errors.push(`${skillMd}: link target does not exist: ${target}`);
    }
  }
}

const roots = ["skills", ...fs.globSync("cli/*/skills")];
let count = 0;
for (const root of roots) {
  if (!fs.existsSync(root)) continue;
  for (const entry of fs.readdirSync(root, { withFileTypes: true })) {
    // Dot-dirs (.claude-plugin) are plugin metadata, not skill packages.
    if (!entry.isDirectory() || entry.name.startsWith(".")) continue;
    lintSkill(path.join(root, entry.name));
    count++;
  }
}

if (errors.length > 0) {
  for (const e of errors) console.error(e);
  console.error(`skill lint: ${errors.length} error(s) across ${count} package(s)`);
  process.exit(1);
}
console.log(`skill lint: ${count} package(s) clean`);
