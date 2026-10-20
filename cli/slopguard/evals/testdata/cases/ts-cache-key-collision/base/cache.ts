const cache = new Map<string, string>();

export function cached(key: string, build: () => string): string {
  const hit = cache.get(key);
  if (hit !== undefined) return hit;
  const value = build();
  cache.set(key, value);
  return value;
}
