import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import ScrapeDetail from './ScrapeDetail.svelte'
import { stubApi, requestedPaths } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const scrape = {
  id: 'scrape-1',
  url: 'https://example.com/fixture-one',
  run_id: 'run-1',
  http_status: 200,
  content_type: 'text/html',
  fetched_with: 'http',
  robots_allowed: true,
  title: 'Fixture One',
  clean_html: '<h1>Fixture One</h1>',
  text: 'Fixture One',
  duration_ms: 120,
  created_at: '2026-08-05T10:03:00Z',
  images: [{ id: 'img-1', url: 'https://example.com/one.png', alt: 'Fixture image', width: 320, height: 200 }],
  content_hash: 'fixturehash',
  etag: 'W/"fixture-etag"',
  last_modified: 'Wed, 05 Aug 2026 10:00:00 GMT',
  tier: 'short',
  hit_count: 3,
  expires_at: '2026-08-15T10:00:00Z',
  fetched_at: '2026-08-05T10:03:00Z',
}

const rawHtml = '<html><body><h1>Fixture One</h1><script>alert(1)</script></body></html>'

describe('ScrapeDetail', () => {
  it('renders every fetch-metadata field the task requires', async () => {
    stubApi({ '/api/scrapes/scrape-1': { json: scrape } })
    render(ScrapeDetail, { props: { id: 'scrape-1' } })

    await waitFor(() => expect(screen.getByTestId('scrape-metadata')).toBeTruthy())
    const table = screen.getByTestId('scrape-metadata').textContent ?? ''

    for (const label of [
      'URL',
      'HTTP status',
      'Content type',
      'Fetched with',
      'Title',
      'Robots allowed',
      'Content hash',
      'ETag',
      'Last modified',
      'Tier',
      'Hit count',
      'Expires at',
      'Fetched at',
      'Duration',
      'Error',
    ]) {
      expect(table).toContain(label)
    }
    expect(table).toContain('fixturehash')
    expect(table).toContain('W/"fixture-etag"')
    expect(table).toContain('short')
    expect(table).toContain('120ms')
  })

  it('shows text content by default and does not request the raw HTML', async () => {
    const fetchMock = stubApi({ '/api/scrapes/scrape-1': { json: scrape } })
    render(ScrapeDetail, { props: { id: 'scrape-1' } })

    await waitFor(() => expect(screen.getByTestId('scrape-text')).toBeTruthy())
    expect(requestedPaths(fetchMock)).toEqual(['/api/scrapes/scrape-1'])
  })

  it('fetches ?raw=1 only when the raw tab is opened, and only once', async () => {
    const fetchMock = stubApi({
      '/api/scrapes/scrape-1': { json: scrape },
      '/api/scrapes/scrape-1?raw=1': { json: { ...scrape, raw_html: rawHtml } },
    })
    render(ScrapeDetail, { props: { id: 'scrape-1' } })
    await waitFor(() => expect(screen.getByTestId('scrape-text')).toBeTruthy())

    await fireEvent.click(screen.getByTestId('tab-raw'))

    const frame = await waitFor(() => screen.getByTestId('raw-frame'))
    expect(frame.getAttribute('sandbox')).toBe('')
    expect(frame.getAttribute('srcdoc')).toBe(rawHtml)

    // Switching away and back must not refetch a body that can be megabytes.
    await fireEvent.click(screen.getByTestId('tab-text'))
    await fireEvent.click(screen.getByTestId('tab-raw'))
    await waitFor(() => expect(screen.getByTestId('raw-frame')).toBeTruthy())

    const rawCalls = requestedPaths(fetchMock).filter((p) => p.includes('raw=1'))
    expect(rawCalls).toHaveLength(1)
  })

  it('renders clean HTML sandboxed rather than injected', async () => {
    stubApi({ '/api/scrapes/scrape-1': { json: scrape } })
    const { container } = render(ScrapeDetail, { props: { id: 'scrape-1' } })
    await waitFor(() => expect(screen.getByTestId('scrape-text')).toBeTruthy())

    await fireEvent.click(screen.getByTestId('tab-clean'))

    const frame = screen.getByTestId('clean-frame')
    expect(frame.getAttribute('sandbox')).toBe('')
    expect(frame.getAttribute('srcdoc')).toBe('<h1>Fixture One</h1>')
    // The cleaned markup must not have become part of the app document.
    expect(container.querySelectorAll('h1')).toHaveLength(1)
  })

  it('renders stored images', async () => {
    stubApi({ '/api/scrapes/scrape-1': { json: scrape } })
    render(ScrapeDetail, { props: { id: 'scrape-1' } })

    await waitFor(() => expect(screen.getByTestId('images-list')).toBeTruthy())
    const list = screen.getByTestId('images-list')
    expect(list.textContent).toContain('320×200')
    expect(list.querySelector('img')?.getAttribute('referrerpolicy')).toBe('no-referrer')
  })

  it('renders a scrape with no images and no content gracefully', async () => {
    stubApi({
      '/api/scrapes/scrape-2': {
        json: {
          id: 'scrape-2',
          url: 'https://example.org/fixture-two',
          robots_allowed: false,
          http_status: 404,
          error: 'not found',
          hit_count: 0,
          duration_ms: 0,
          created_at: '2026-08-05T10:04:00Z',
        },
      },
    })
    render(ScrapeDetail, { props: { id: 'scrape-2' } })

    await waitFor(() => expect(screen.getByTestId('images-empty')).toBeTruthy())
    expect(screen.getByTestId('text-empty')).toBeTruthy()
    expect(screen.getByTestId('scrape-metadata').textContent).toContain('not found')

    await fireEvent.click(screen.getByTestId('tab-clean'))
    expect(screen.getByTestId('clean-empty')).toBeTruthy()
  })

  it('surfaces a missing scrape as not found', async () => {
    stubApi({ '/api/scrapes/nope': { status: 404, json: { error: 'sql: no rows in result set' } } })
    render(ScrapeDetail, { props: { id: 'nope' } })

    await waitFor(() => expect(screen.getByTestId('scrape-missing')).toBeTruthy())
    expect(screen.queryByTestId('scrape-error')).toBeNull()
  })
})
