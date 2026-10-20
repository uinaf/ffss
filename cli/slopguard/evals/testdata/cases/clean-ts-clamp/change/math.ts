export function sum(values: number[]): number {
  return values.reduce((total, value) => total + value, 0);
}

export function clamp(value: number, min: number, max: number): number {
  if (min > max) {
    throw new RangeError("min must not exceed max");
  }
  return Math.min(Math.max(value, min), max);
}
