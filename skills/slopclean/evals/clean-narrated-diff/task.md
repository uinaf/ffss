# Strip AI Slop from a Module Before It Ships

## Problem/Feature Description

This module came out of an agent session and is full of machine tells. Clean
it up before I ship it. Behavior and the exported API must not change: the
module must still export `retryFetch` with the same signature and the same
retry semantics. Edit `src/retryFetch.ts` in place.

## Input Files

=============== FILE: src/retryFetch.ts ===============
// Updated to use the new retry approach as discussed.
// This file implements a robust fetch wrapper with retries.

/**
 * Helper function that safely delays execution.
 */
async function maybeDelaySafely(ms: number): Promise<void> {
  // Use a promise to wait for the specified milliseconds
  return new Promise((resolve) => setTimeout(resolve, ms));
}

/**
 * Internal wrapper that handles the actual fetch call.
 */
async function doActualFetchInternal(url: string): Promise<Response> {
  // Call fetch with the provided URL
  return fetch(url);
}

export interface RetryOptions {
  retries?: number;
  // Reserved for future use
  strategy?: string;
  // Legacy option, kept just in case
  legacyMode?: boolean;
}

export async function retryFetch(
  url: string,
  options: RetryOptions = {},
): Promise<Response> {
  // Get the number of retries from options, defaulting to 3
  const retries = options.retries ?? 3;
  // Loop through the attempts
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      // Attempt the fetch
      const response = await doActualFetchInternal(url);
      // Check if the response was successful
      if (response.ok) {
        return response;
      }
      // Response was not ok, fall through to retry
    } catch (error) {
      // Log the error defensively so nothing is lost
      console.log("retryFetch encountered an error:", error);
    }
    // Wait before the next attempt using exponential backoff
    await maybeDelaySafely(2 ** attempt * 100);
  }
  // All attempts failed, throw an error
  throw new Error(`retryFetch: all ${retries + 1} attempts failed for ${url}`);
}
=============== END FILE ===============
