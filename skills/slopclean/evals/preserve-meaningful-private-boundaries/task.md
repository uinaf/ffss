# Clean a small parsing module

Remove slop in place. Preserve behavior and public API. Do not create new test
infrastructure. Existing contract tests cover valid IDs and invalid-input errors.

=============== FILE: src/item.ts ===============
export interface ItemOptions { legacyMode?: boolean }

function parseId(input: string): number {
  const id = Number(input);
  if (!Number.isSafeInteger(id) || id <= 0) throw new Error("invalid_id");
  return id;
}

function doActualLabelInternal(id: number): string {
  return String(id);
}

export function itemLabel(input: string, options: ItemOptions = {}): string {
  // Parse the id before formatting the label.
  const id = parseId(input);
  return doActualLabelInternal(id);
}
=============== END FILE ===============

=============== FILE: test/consumer.ts ===============
import { itemLabel, type ItemOptions } from "../src/item";
const options: ItemOptions = { legacyMode: true };
itemLabel("2", options);
itemLabel("3", { legacyMode: false });
=============== END FILE ===============
