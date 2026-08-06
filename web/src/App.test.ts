import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/svelte'
import App from './App.svelte'
import { stubApi } from './lib/apiStub'

beforeEach(() => {
  window.history.replaceState({}, '', '/')
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  window.history.replaceState({}, '', '/')
})

const run = {
  id: 'run-1',
  mode: 'serve',
  started_at: '2026-08-05T10:00:00Z',
  finished_at: '2026-08-05T10:05:00Z',
  searches: 1,
  urls: 1,
  scrapes: 1,
}

function at(path: string): void {
  window.history.replaceState({}, '', path)
}

describe('App routing', () => {
  it('renders the runs list at the root', async () => {
    stubApi({ '/api/runs': { json: [run] } })
    render(App)

    await waitFor(() => expect(screen.getByTestId('runs-table')).toBeTruthy())
  })

  it('renders the runs list at /runs', async () => {
    at('/runs')
    stubApi({ '/api/runs': { json: [run] } })
    render(App)

    await waitFor(() => expect(screen.getByTestId('runs-table')).toBeTruthy())
  })

  it('renders run detail at /runs/:id', async () => {
    at('/runs/run-1')
    stubApi({
      '/api/runs/run-1': { json: run },
      '/api/runs/run-1/urls': { json: null },
      '/api/runs/run-1/searches': { json: null },
      '/api/runs/run-1/scrapes': { json: { scrape_ids: null } },
    })
    render(App)

    await waitFor(() => expect(screen.getByTestId('run-summary')).toBeTruthy())
  })

  it('prefers the more specific searches route over run detail', async () => {
    at('/runs/run-1/searches')
    stubApi({ '/api/runs/run-1/searches': { json: null } })
    render(App)

    await waitFor(() => expect(screen.getByTestId('searches-empty')).toBeTruthy())
    expect(screen.queryByTestId('run-summary')).toBeNull()
  })

  it('renders the SERP viewer at /searches/:id', async () => {
    at('/searches/search-1')
    stubApi({ '/api/searches/search-1/raw': { text: '<html><body>serp</body></html>' } })
    render(App)

    await waitFor(() => expect(screen.getByTestId('serp-frame')).toBeTruthy())
  })

  it('renders scrape detail at /scrapes/:id', async () => {
    at('/scrapes/scrape-1')
    stubApi({
      '/api/scrapes/scrape-1': {
        json: { id: 'scrape-1', url: 'https://example.com/x', robots_allowed: true, hit_count: 0, duration_ms: 1, created_at: '2026-08-05T10:00:00Z' },
      },
    })
    render(App)

    await waitFor(() => expect(screen.getByTestId('scrape-metadata')).toBeTruthy())
  })

  it('renders a not-found view for an unknown route', async () => {
    at('/nope/whatever/deep')
    stubApi({})
    render(App)

    await waitFor(() => expect(screen.getByTestId('not-found')).toBeTruthy())
    expect(screen.getByTestId('not-found-home').getAttribute('href')).toBe('/runs')
  })

  it('navigates in-page when an internal link is clicked', async () => {
    stubApi({
      '/api/runs': { json: [run] },
      '/api/runs/run-1': { json: run },
      '/api/runs/run-1/urls': { json: null },
      '/api/runs/run-1/searches': { json: null },
      '/api/runs/run-1/scrapes': { json: { scrape_ids: null } },
    })
    render(App)

    const link = await waitFor(() => screen.getByTestId('run-link'))
    link.click()

    await waitFor(() => expect(screen.getByTestId('run-summary')).toBeTruthy())
    expect(window.location.pathname).toBe('/runs/run-1')
  })
})
