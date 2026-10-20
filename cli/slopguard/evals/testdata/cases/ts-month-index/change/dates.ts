// parseIsoDate turns "2026-03-14" into a local Date.
export function parseIsoDate(value: string): Date {
  const [year, month, day] = value.split("-").map(Number);
  return new Date(year, month, day);
}
