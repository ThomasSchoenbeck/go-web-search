import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import ScrapeCacheBrowser from './ScrapeCacheBrowser.svelte'
import { stubApi, requestedPaths } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function entry(overrides: Record<string, unknown> = {}) {
  return {
    id: 's1',
    url: 'https://example.com/fixture-one',
    http_status: 200,
    content_type: 'text/html',
    fetched_with: 'http',
    title: 'Fixture One',
    robots_allowed: true,
    content_hash: 'fixturehash',
    tier: 'short',
    hit_count: 3,
    text_chars: 11,
    clean_html_chars: 22,
    raw_html_chars: 46,
    expires_at: '2026-08-15T10:00:00Z',
    fetched_at: '2026-08-06T10:00:00Z',
    created_at: '2026-08-06T10:00:00Z',
    updated_at: '2026-08-06T10:00:00Z',
    ...overrides,
  }
}

const firstPage = '/api/cache/scrapes?limit=25&offset=0'

describe('ScrapeCacheBrowser', () => {
  it('renders cache metadata and content sizes, not bodies', async () => {
    stubApi({ [firstPage]: { json: { count: 1, entries: [entry()] } } })
    render(ScrapeCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('scrape-cache-table')).toBeTruthy())
    const table = screen.getByTestId('scrape-cache-table').textContent ?? ''
    expect(table).toContain('example.com/fixture-one')
    expect(table).toContain('text/html')
    expect(table).toContain('short')
    expect(table).toContain('allowed')
    expect(table).toContain('46') // raw size
  })

  it('shows a failed fetch with its error and blocked robots', async () => {
    stubApi({
      [firstPage]: {
        json: {
          count: 1,
          entries: [entry({ http_status: 404, error: 'not found', robots_allowed: false, text_chars: 0 })],
        },
      },
    })
    render(ScrapeCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('scrape-cache-table').textContent).toContain('not found'))
    expect(screen.getByTestId('scrape-cache-table').textContent).toContain('blocked')
  })

  it('links each row to its scrape detail and its provenance', async () => {
    stubApi({ [firstPage]: { json: { count: 1, entries: [entry()] } } })
    render(ScrapeCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('scrape-cache-detail-link')).toBeTruthy())
    expect(screen.getByTestId('scrape-cache-detail-link').getAttribute('href')).toBe('/scrapes/s1')
    expect(screen.getByTestId('scrape-cache-provenance-link').getAttribute('href')).toBe(
      `/provenance?url=${encodeURIComponent('https://example.com/fixture-one')}`,
    )
  })

  it('filters by tier and by URL', async () => {
    const fetchMock = stubApi({
      [firstPage]: { json: { count: 1, entries: [entry()] } },
      '/api/cache/scrapes?tier=long&limit=25&offset=0': { json: { count: 1, entries: [entry({ tier: 'long' })] } },
      '/api/cache/scrapes?tier=long&q=example.org&limit=25&offset=0': { json: { count: 0, entries: null } },
    })
    render(ScrapeCacheBrowser)
    await waitFor(() => expect(screen.getByTestId('scrape-cache-table')).toBeTruthy())

    await fireEvent.change(screen.getByTestId('scrape-cache-tier'), { target: { value: 'long' } })
    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/cache/scrapes?tier=long&limit=25&offset=0'))

    await fireEvent.input(screen.getByTestId('scrape-cache-filter'), { target: { value: 'example.org' } })
    await fireEvent.click(screen.getByTestId('scrape-cache-submit'))
    await waitFor(() =>
      expect(requestedPaths(fetchMock)).toContain('/api/cache/scrapes?tier=long&q=example.org&limit=25&offset=0'),
    )
    expect(screen.getByTestId('scrape-cache-empty').textContent).toContain('No cached pages match')

    await fireEvent.click(screen.getByTestId('scrape-cache-clear'))
    await waitFor(() => expect(screen.getByTestId('scrape-cache-table')).toBeTruthy())
  })

  it('pages with limit and offset', async () => {
    const fullPage = Array.from({ length: 25 }, (_, i) => entry({ id: `s${i}` }))
    const fetchMock = stubApi({
      [firstPage]: { json: { count: 25, entries: fullPage } },
      '/api/cache/scrapes?limit=25&offset=25': { json: { count: 1, entries: [entry({ id: 'last' })] } },
    })
    render(ScrapeCacheBrowser)
    await waitFor(() => expect(screen.getByTestId('scrape-cache-table')).toBeTruthy())

    await fireEvent.click(screen.getByTestId('scrape-cache-next'))
    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/cache/scrapes?limit=25&offset=25'))
    expect(screen.getByTestId('scrape-cache-page').textContent).toContain('Page 2')
  })

  it('renders an empty cache cleanly', async () => {
    stubApi({ [firstPage]: { json: { count: 0, entries: null } } })
    render(ScrapeCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('scrape-cache-empty').textContent).toContain('Nothing has been cached'))
  })

  it('surfaces a failure', async () => {
    stubApi({ [firstPage]: { status: 500, json: { error: 'boom' } } })
    render(ScrapeCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('scrape-cache-error').textContent).toContain('boom'))
  })

  it('links back to the search cache', async () => {
    stubApi({ [firstPage]: { json: { count: 1, entries: [entry()] } } })
    render(ScrapeCacheBrowser)

    await waitFor(() => expect(screen.getByTestId('to-search-cache').getAttribute('href')).toBe('/cache/searches'))
  })
})
