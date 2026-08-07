import { defineConfig } from 'vitest/config'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  // Vitest runs tests in Node, where Svelte would otherwise resolve to its
  // server build and refuse to mount components. Force the browser condition.
  resolve: { conditions: ['browser'] },
  test: {
    // jsdom gives the read layer a window.location to resolve API paths against
    // and lets components mount without a browser.
    environment: 'jsdom',
    globals: true,
    // Unit tests live next to the code they cover; e2e specs are Playwright's.
    include: ['src/**/*.test.ts'],
    exclude: ['tests/e2e/**'],
    restoreMocks: true,
  },
})
