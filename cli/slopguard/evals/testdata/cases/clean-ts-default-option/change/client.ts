import { validateTimeout } from "./limits";

export const DEFAULT_TIMEOUT_MS = 30000;

export function timeout(ms: number = DEFAULT_TIMEOUT_MS): number {
  return validateTimeout(ms);
}
