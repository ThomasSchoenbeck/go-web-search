import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import SearchCacheBrowser from './SearchCacheBrowser.svelte'
import { stubApi, requestedPaths } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function entry(overrides: Record<string, unknown> = {}) {
  return {
    id: 'c1',
    query: 'fixture term',
    query_norm: 'fixture term',
    tier: 'short',
    hit_count: 3,
    result_count: 2,
    results_chars: 120,
    expires_at: '2026-08-15T10:00:00Z',
    fetched_at: '2026-08-06T10:00:00Z',
    created_at: '2026-08-06T10:00:00Z',
    updated_at: '2026-08-06T10:00:00Z',
    ...overrides,
  }
}

const firstPage = '/api/cache/searches?limit=25&offset=0'

describe('SearchCacheBrowser', () => {
  it('renders tier, hits and the results summary', async () => {
    stubApi({ [firstPage]: { json: { count: 1, entries: [entry()] } } })
    render(SearchCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('search-cache-table')).toBeTruthy())
    const table = screen.getByTestId('search-cache-table').textContent ?? ''
    expect(table).toContain('fixture term')
    expect(table).toContain('short')
    expect(table).toContain('3')
    expect(table).toContain('2') // the results summary, not the URLs themselves
  })

  it('shows a permanent row as never expiring', async () => {
    stubApi({ [firstPage]: { json: { count: 1, entries: [entry({ tier: 'permanent', expires_at: undefined })] } } })
    render(SearchCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('search-cache-table').textContent).toContain('never'))
  })

  it('filters by tier', async () => {
    const fetchMock = stubApi({
      [firstPage]: { json: { count: 1, entries: [entry()] } },
      '/api/cache/searches?tier=long&limit=25&offset=0': { json: { count: 1, entries: [entry({ tier: 'long' })] } },
    })
    render(SearchCacheBrowser)
    await waitFor(() => expect(screen.getByTestId('search-cache-table')).toBeTruthy())

    await fireEvent.change(screen.getByTestId('search-cache-tier'), { target: { value: 'long' } })
    await waitFor(() =>
      expect(requestedPaths(fetchMock)).toContain('/api/cache/searches?tier=long&limit=25&offset=0'),
    )
  })

  it('filters by query text and clears back', async () => {
    // URLSearchParams encodes a space as +, so the stub key must match exactly.
    const filtered = '/api/cache/searches?q=fixture+term&limit=25&offset=0'
    const fetchMock = stubApi({
      [firstPage]: { json: { count: 1, entries: [entry()] } },
      [filtered]: { json: { count: 0, entries: null } },
    })
    render(SearchCacheBrowser)
    await waitFor(() => expect(screen.getByTestId('search-cache-table')).toBeTruthy())

    await fireEvent.input(screen.getByTestId('search-cache-filter'), { target: { value: 'fixture term' } })
    await fireEvent.click(screen.getByTestId('search-cache-submit'))
    await waitFor(() => expect(requestedPaths(fetchMock)).toContain(filtered))
    expect(screen.getByTestId('search-cache-empty').textContent).toContain('No cached queries match')

    await fireEvent.click(screen.getByTestId('search-cache-clear'))
    await waitFor(() => expect(screen.getByTestId('search-cache-table')).toBeTruthy())
  })

  it('pages with limit and offset', async () => {
    const fullPage = Array.from({ length: 25 }, (_, i) => entry({ id: `c${i}` }))
    const fetchMock = stubApi({
      [firstPage]: { json: { count: 25, entries: fullPage } },
      '/api/cache/searches?limit=25&offset=25': { json: { count: 1, entries: [entry({ id: 'last' })] } },
    })
    render(SearchCacheBrowser)
    await waitFor(() => expect(screen.getByTestId('search-cache-table')).toBeTruthy())

    expect(screen.getByTestId('search-cache-prev')).toHaveProperty('disabled', true)
    await fireEvent.click(screen.getByTestId('search-cache-next'))

    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/cache/searches?limit=25&offset=25'))
    expect(screen.getByTestId('search-cache-page').textContent).toContain('Page 2')
  })

  it('renders an empty cache cleanly', async () => {
    stubApi({ [firstPage]: { json: { count: 0, entries: null } } })
    render(SearchCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('search-cache-empty').textContent).toContain('Nothing has been cached'))
  })

  it('surfaces a failure', async () => {
    stubApi({ [firstPage]: { status: 500, json: { error: 'boom' } } })
    render(SearchCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('search-cache-error').textContent).toContain('boom'))
  })

  it('links across to the scrape cache', async () => {
    stubApi({ [firstPage]: { json: { count: 1, entries: [entry()] } } })
    render(SearchCacheBrowser)

    await waitFor(() =>
      expect(screen.getByTestId('to-scrape-cache').getAttribute('href')).toBe('/cache/scrapes'),
    )
  })
})
