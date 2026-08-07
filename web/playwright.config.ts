import { defineConfig, devices } from '@playwright/test'

// The base URL is not known when this file is evaluated: globalSetup picks a
// free port, starts the Go binary against throwaway databases, and publishes
// the URL through process.env, which worker processes inherit. Specs read it
// via baseUrl() from ./tests/e2e/fixtures.
export default defineConfig({
  testDir: './tests/e2e',
  globalSetup: './tests/e2e/fixtures.ts',
  fullyParallel: true,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  reporter: process.env.CI ? 'line' : 'list',
  use: {
    trace: 'on-first-retry',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
})
