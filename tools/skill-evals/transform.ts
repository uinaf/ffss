// promptfoo output transform: append files the agent created or changed in the
// scenario workdir, so llm-rubric grades the deliverables, not just chat text.
import { createHash } from "node:crypto";
import fs from "node:fs";
import path from "node:path";

interface TransformContext {
  vars: { workdir: string; manifest: string; [key: string]: string };
}

export default function transform(output: string, context: TransformContext): string {
  const workdir = context.vars.workdir;
  const manifest: Record<string, string> = JSON.parse(fs.readFileSync(context.vars.manifest, "utf8"));
  const sections: string[] = [];
  const visited = new Set<string>();
  const walk = (dir: string): void => {
    for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
      const p = path.join(dir, e.name);
      if (e.isDirectory()) {
        if (e.name !== ".claude") walk(p);
      } else if (e.isFile()) {
        const rel = path.relative(workdir, p);
        visited.add(rel);
        let body;
        try {
          body = fs.readFileSync(p);
        } catch (err) {
          const detail = err instanceof Error ? err.message : String(err);
          sections.push(`=== UNREADABLE FILE: ${rel} === (${detail})`);
          continue;
        }
        const hash = createHash("sha256").update(body).digest("hex");
        if (manifest[rel] !== hash) {
          sections.push(`=== OUTPUT FILE: ${rel} ===\n${body.toString("utf8")}\n=== END OUTPUT FILE ===`);
        }
      } else {
        // Symlinks, FIFOs, sockets: name them for the judge, never read them.
        sections.push(`=== NON-REGULAR FILE: ${path.relative(workdir, p)} ===`);
      }
    }
  };
  walk(workdir);
  for (const rel of Object.keys(manifest)) {
    if (!visited.has(rel)) sections.push(`=== DELETED FILE: ${rel} === (input file removed by the agent)`);
  }
  if (sections.length === 0) return output;
  return `${output}\n\n${sections.join("\n\n")}`;
}
