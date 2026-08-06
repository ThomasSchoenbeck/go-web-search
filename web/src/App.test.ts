import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/svelte'
import App from './App.svelte'
import { resetUIConfig } from './lib/uiconfig'

afterEach(() => {
  cleanup()
  resetUIConfig()
  vi.unstubAllGlobals()
})

// Covers the component side of the harness: a Svelte 5 component mounts under
// jsdom and renders what the read layer returns.
function stubApi(overrides: Record<string, unknown> = {}): void {
  const bodies: Record<string, unknown> = {
    '/api/ui-config': { poll_interval_ms: 5000, poll_enabled: false, projection_sample_cap: 2000 },
    '/api/stats': { runs: 7, searches: 3, urls: 12 },
    ...overrides,
  }
  vi.stubGlobal(
    'fetch',
    vi.fn(async (url: string) => {
      const path = new URL(url).pathname
      const body = bodies[path]
      if (body === undefined) return { ok: false, status: 404, statusText: 'Not Found', json: async () => ({}) }
      return { ok: true, status: 200, json: async () => body }
    }),
  )
}

describe('App', () => {
  it('renders the shell', () => {
    stubApi()
    render(App)
    expect(screen.getByRole('heading', { name: 'Observability UI', level: 1 })).toBeTruthy()
  })

  it('renders stats and the config-seeded settings', async () => {
    stubApi()
    render(App)

    await waitFor(() => expect(screen.getByTestId('stats').textContent).toContain('"runs": 7'))
    // Settings load after the first stats fetch, so wait for them separately.
    await waitFor(() => expect(screen.getByTestId('settings').textContent).toContain('5000ms'))
  })

  it('surfaces an API failure instead of rendering nothing', async () => {
    stubApi({ '/api/stats': undefined })
    render(App)

    await waitFor(() => expect(screen.getByTestId('stats-error').textContent).toContain('/api/stats'))
  })
})
