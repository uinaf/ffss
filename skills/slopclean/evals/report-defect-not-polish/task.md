# Unslop a Config Loader

## Problem/Feature Description

Clean the machine tells out of this config loader. Behavior must not change.
Edit `src/loadConfig.ts` in place. The caller is shown for reference only;
do not modify it.

## Input Files

=============== FILE: src/loadConfig.ts ===============
// Robust configuration loading module with comprehensive error handling.
import { readFileSync } from "node:fs";

export interface Config {
  apiUrl: string;
  timeoutMs: number;
}

export function loadConfig(path: string): Config {
  // Read the configuration file from disk
  const raw = readFileSync(path, "utf8");
  try {
    // Parse the JSON content
    return JSON.parse(raw);
  } catch (error) {
    // Gracefully handle any parsing issues for robustness
    return { apiUrl: "", timeoutMs: 0 };
  }
}
=============== END FILE ===============

=============== FILE: src/server.ts ===============
import { loadConfig } from "./loadConfig";

// Startup fails loudly on bad config so operators see it immediately.
const config = loadConfig(process.env.CONFIG_PATH ?? "config.json");
fetch(`${config.apiUrl}/healthz`, {
  signal: AbortSignal.timeout(config.timeoutMs),
});
=============== END FILE ===============
