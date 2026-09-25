Agents keep leaving slop in this TypeScript service: `as any` casts, empty
catch blocks, comments narrating what the next line does, and barrel files
re-exporting everything. Review comments aren't stopping it. Set up
enforcement so it stops landing, and write `enforcement-notes.md` saying what
now catches what and how it rolls out. Don't push or open anything.

=============== FILE: package.json ===============
{
  "name": "ledgerline",
  "private": true,
  "type": "module",
  "scripts": {
    "build": "tsc -p tsconfig.json",
    "lint": "oxlint",
    "test": "vitest run",
    "verify": "npm run lint && npm run build && npm test"
  },
  "devDependencies": {
    "oxlint": "1.19.0",
    "typescript": "5.9.3",
    "vitest": "3.2.4"
  }
}
=============== END FILE ===============

=============== FILE: .oxlintrc.json ===============
{
  "$schema": "./node_modules/oxlint/configuration_schema.json",
  "categories": { "correctness": "error" }
}
=============== END FILE ===============

=============== FILE: src/index.ts ===============
export * from "./ledger";
export * from "./format";
=============== END FILE ===============

=============== FILE: src/ledger.ts ===============
import { formatAmount } from "./format";

export function total(entries: unknown[]): number {
  // Loop over the entries and add each amount
  let sum = 0;
  for (const e of entries) {
    sum += (e as any).amount;
  }
  return sum;
}

export function describe(entries: unknown[]): string {
  try {
    return formatAmount(total(entries));
  } catch {}
  return "";
}
=============== END FILE ===============

=============== FILE: src/format.ts ===============
export function formatAmount(cents: number): string {
  // Divide by 100 to get dollars
  return (cents / 100).toFixed(2);
}
=============== END FILE ===============
