# Prune a Retry Test Suite

## Problem/Feature Description

Our retry tests grew one agent-written case at a time. Remove the least useful
50% of test cases in `tests/` while keeping line coverage within 2 points and
every real contract proven. `npm test -- --coverage` measures coverage. The
package is private, so you may delete code that only tests use; otherwise leave
`src/` behavior unchanged. On the pinned baseline, `rejects negative delays`
fails and every other test passes.

## Input Files

=============== FILE: package.json ===============
{
  "name": "retry-client",
  "private": true,
  "type": "module",
  "scripts": {
    "test": "vitest run"
  },
  "devDependencies": {
    "@vitest/coverage-v8": "^3.2.4",
    "typescript": "^5.9.2",
    "vitest": "^3.2.4"
  }
}
=============== END FILE ===============

=============== FILE: vitest.config.ts ===============
import { defineConfig } from "vitest/config";

export default defineConfig({
  test: { coverage: { include: ["src/**"], reportOnFailure: true } },
});
=============== END FILE ===============

=============== FILE: src/retry.ts ===============
export const RETRY_HEADER = "retry-after";

const cache = new Map<string, number | null>();

export function parseRetryAfter(value: string, now = Date.now()): number | null {
  const hit = cache.get(value);
  if (hit !== undefined) return hit;
  let delay: number | null = null;
  if (/^-?\d+$/.test(value)) {
    delay = Number(value) * 1000;
  } else {
    const at = Date.parse(value);
    if (!Number.isNaN(at)) delay = Math.max(0, at - now);
  }
  cache.set(value, delay);
  return delay;
}

export function __resetCacheForTests(): void {
  cache.clear();
}
=============== END FILE ===============

=============== FILE: src/client.ts ===============
import { RETRY_HEADER, parseRetryAfter } from "./retry";

export async function fetchWithRetry(
  url: string,
  send: (url: string) => Promise<Response>,
  sleep: (ms: number) => Promise<void>,
): Promise<Response> {
  const first = await send(url);
  if (first.status !== 429) return first;
  const delay = parseRetryAfter(first.headers.get(RETRY_HEADER) ?? "");
  if (delay === null) return first;
  await sleep(delay);
  return send(url);
}
=============== END FILE ===============

=============== FILE: tests/retry.test.ts ===============
import { describe, it, expect, beforeEach } from "vitest";
import { RETRY_HEADER, parseRetryAfter, __resetCacheForTests } from "../src/retry";

describe("parseRetryAfter", () => {
  beforeEach(() => __resetCacheForTests());

  it("parses delay seconds", () => {
    expect(parseRetryAfter("5")).toBe(5000);
  });

  it("parses delay seconds for 30", () => {
    expect(parseRetryAfter("30")).toBe(30000);
  });

  it("parses an HTTP date", () => {
    const now = Date.parse("Wed, 21 Oct 2026 07:28:00 GMT");
    expect(parseRetryAfter("Wed, 21 Oct 2026 07:28:10 GMT", now)).toBe(10000);
  });

  it("returns a number", () => {
    expect(typeof parseRetryAfter("1")).toBe("number");
  });

  it("uses the standard header name", () => {
    expect(RETRY_HEADER).toBe("retry-after");
  });

  it("resets the cache", () => {
    parseRetryAfter("2");
    __resetCacheForTests();
    expect(parseRetryAfter("2")).toBe(2000);
  });

  it("rejects negative delays", () => {
    expect(parseRetryAfter("-5")).toBeNull();
  });
});
=============== END FILE ===============

=============== FILE: tests/client.test.ts ===============
import { describe, it, expect, vi } from "vitest";
import { fetchWithRetry } from "../src/client";

const response = (status: number, retryAfter?: string) =>
  new Response(null, { status, headers: retryAfter ? { "retry-after": retryAfter } : {} });

describe("fetchWithRetry", () => {
  it("waits the server delay and retries a 429 once", async () => {
    const send = vi.fn()
      .mockResolvedValueOnce(response(429, "3"))
      .mockResolvedValueOnce(response(200));
    const sleep = vi.fn().mockResolvedValue(undefined);
    const res = await fetchWithRetry("https://api.test", send, sleep);
    expect(sleep).toHaveBeenCalledWith(3000);
    expect(send).toHaveBeenCalledTimes(2);
    expect(res.status).toBe(200);
  });

  it("parses a 5 second delay", async () => {
    const send = vi.fn()
      .mockResolvedValueOnce(response(429, "5"))
      .mockResolvedValueOnce(response(200));
    const sleep = vi.fn().mockResolvedValue(undefined);
    await fetchWithRetry("https://api.test", send, sleep);
    expect(sleep).toHaveBeenCalledWith(5000);
  });

  it("parses a 30 second delay", async () => {
    const send = vi.fn()
      .mockResolvedValueOnce(response(429, "30"))
      .mockResolvedValueOnce(response(200));
    const sleep = vi.fn().mockResolvedValue(undefined);
    await fetchWithRetry("https://api.test", send, sleep);
    expect(sleep).toHaveBeenCalledWith(30000);
  });
});
=============== END FILE ===============
