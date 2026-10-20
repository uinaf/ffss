export interface Options {
  retries: number;
  timeoutMs: number;
}

const defaults: Options = { retries: 2, timeoutMs: 5000 };

export function resolveOptions(input: Partial<Options>): Options {
  return { retries: input.retries ?? defaults.retries, timeoutMs: input.timeoutMs ?? defaults.timeoutMs };
}
