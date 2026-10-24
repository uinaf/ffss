we ripped out TanStack Query last month (plain fetch + a tiny cache now) but AGENTS.md still talks about it. clean that up so the guide matches the code

=============== FILE: AGENTS.md ===============
# Agent Guide

Internal dashboard for order operations. `pnpm dev` runs it; `pnpm test` is
the gate.

## Data fetching

All server state goes through TanStack Query. Define query keys in
`src/queries/keys.ts` and invalidate after every mutation.

## Skills

- Query cache invalidation: tanstack-query
- React diagnostics after UI changes: react-doctor
=============== END FILE ===============

=============== FILE: package.json ===============
{
  "name": "orders-dashboard",
  "private": true,
  "type": "module",
  "dependencies": {
    "react": "^19.0.0",
    "react-dom": "^19.0.0"
  }
}
=============== END FILE ===============

=============== FILE: src/api/cache.ts ===============
const cache = new Map<string, { at: number; value: unknown }>();

export async function cachedFetch<T>(url: string, maxAgeMs = 30_000): Promise<T> {
  const hit = cache.get(url);
  if (hit && Date.now() - hit.at < maxAgeMs) return hit.value as T;
  const value = (await (await fetch(url)).json()) as T;
  cache.set(url, { at: Date.now(), value });
  return value;
}

export function invalidate(url: string): void {
  cache.delete(url);
}
=============== END FILE ===============
