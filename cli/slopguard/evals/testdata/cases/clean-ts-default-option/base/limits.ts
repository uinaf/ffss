export const MAX_TIMEOUT_MS = 60000;

export function validateTimeout(ms: number): number {
  if (ms < 1 || ms > MAX_TIMEOUT_MS) {
    throw new RangeError(`timeout must be between 1 and ${MAX_TIMEOUT_MS}`);
  }
  return ms;
}
