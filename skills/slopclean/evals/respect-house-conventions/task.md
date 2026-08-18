# Unslop a File Without Breaking House Style

## Problem/Feature Description

Clean the AI slop out of `src/rateLimiter.ts`. Behavior must not change.
The repository has a style guide; it applies. Edit the file in place.

## Input Files

=============== FILE: CONTRIBUTING.md ===============
# Contributing

## Documentation style

Every exported function MUST carry a JSDoc block with a one-line summary and
`@param`/`@returns` tags. CI's docs linter fails the build without them. This
is a hard rule: our API reference is generated from these blocks.
=============== END FILE ===============

=============== FILE: src/rateLimiter.ts ===============
// Note: refactored in the latest iteration to improve clarity.

/**
 * Creates a token-bucket rate limiter.
 * @param capacity Maximum number of tokens the bucket holds.
 * @param refillPerSecond Tokens added back per second.
 * @returns A limiter whose take() returns true when a token was available.
 */
export function createRateLimiter(capacity: number, refillPerSecond: number) {
  // Initialize the token count to the capacity
  let tokens = capacity;
  // Record the last refill timestamp
  let last = Date.now();

  return {
    take(): boolean {
      // Get the current time
      const now = Date.now();
      // Calculate elapsed seconds and refill the bucket accordingly
      tokens = Math.min(capacity, tokens + ((now - last) / 1000) * refillPerSecond);
      // Update the last refill timestamp
      last = now;
      // Check whether a token is available
      if (tokens >= 1) {
        // Consume one token
        tokens -= 1;
        return true;
      }
      // No token was available
      return false;
    },
  };
}
=============== END FILE ===============
