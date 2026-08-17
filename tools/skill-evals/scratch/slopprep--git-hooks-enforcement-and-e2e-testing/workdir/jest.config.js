/** @type {import('jest').Config} */
const tsTransform = {
  '^.+\\.ts$': ['ts-jest', { tsconfig: '<rootDir>/tsconfig.test.json' }],
};

module.exports = {
  projects: [
    {
      displayName: 'unit',
      rootDir: __dirname,
      testEnvironment: 'node',
      testMatch: ['<rootDir>/src/**/*.test.ts'],
      transform: tsTransform,
    },
    {
      displayName: 'e2e',
      rootDir: __dirname,
      testEnvironment: 'node',
      testMatch: ['<rootDir>/e2e/**/*.test.ts'],
      transform: tsTransform,
      // Child-process cases are slower than in-process ones, but still bounded.
      testTimeout: 30_000,
    },
  ],
};
