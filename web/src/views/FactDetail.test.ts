import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/svelte'
import FactDetail from './FactDetail.svelte'
import { stubApi } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const path = '/api/memory/facts/f1'

const detail = {
  fact: {
    id: 'f1',
    text: 'The fixture page says Fixture One.',
    text_chars: 34,
    source_url: 'https://example.com/fixture-one',
    volatility: 'stable',
    tier: 'short',
    hit_count: 2,
    expires_at: '2026-08-15T10:00:00Z',
  },
  source: {
    scrape_id: 'c1',
    url: 'https://example.com/fixture-one',
    title: 'Fixture One',
    http_status: 200,
    fetched_with: 'http',
    text_chars: 11,
    clean_html_chars: 20,
    raw_html_chars: 46,
    created_at: '2026-08-05T10:03:00Z',
  },
  read_raw: '/api/scrapes/c1?raw=1',
}

describe('FactDetail', () => {
  it('renders the fact and its metadata', async () => {
    stubApi({ [path]: { json: detail } })
    render(FactDetail, { props: { id: 'f1' } })

    await waitFor(() => expect(screen.getByTestId('fact-text').textContent).toContain('Fixture One'))
    const table = screen.getByTestId('fact-metadata').textContent ?? ''
    expect(table).toContain('stable')
    expect(table).toContain('short')
    expect(table).toContain('34 characters')
  })

  it('links the source URL to the provenance pivot — the reverse fact→sources path', async () => {
    stubApi({ [path]: { json: detail } })
    render(FactDetail, { props: { id: 'f1' } })

    await waitFor(() =>
      expect(screen.getByTestId('fact-provenance-link').getAttribute('href')).toBe(
        `/provenance?url=${encodeURIComponent('https://example.com/fixture-one')}`,
      ),
    )
  })

  it('shows the source scrape sizes and the raw link', async () => {
    stubApi({ [path]: { json: detail } })
    render(FactDetail, { props: { id: 'f1' } })

    await waitFor(() => expect(screen.getByTestId('fact-source')).toBeTruthy())
    expect(screen.getByTestId('fact-source').textContent).toContain('46 raw')
    expect(screen.getByTestId('fact-scrape-link').getAttribute('href')).toBe('/scrapes/c1')
    expect(screen.getByTestId('fact-read-raw').getAttribute('href')).toBe('/api/scrapes/c1?raw=1')
  })

  it('shows the endpoint note when the source is no longer cached', async () => {
    stubApi({
      [path]: {
        json: {
          fact: detail.fact,
          note: "source page is no longer cached, so the raw material can't be retrieved",
        },
      },
    })
    render(FactDetail, { props: { id: 'f1' } })

    await waitFor(() =>
      expect(screen.getByTestId('fact-source-missing').textContent).toContain('no longer cached'),
    )
    expect(screen.queryByTestId('fact-read-raw')).toBeNull()
  })

  it('handles a fact with no source url', async () => {
    stubApi({ [path]: { json: { fact: { ...detail.fact, source_url: undefined } } } })
    render(FactDetail, { props: { id: 'f1' } })

    await waitFor(() => expect(screen.getByTestId('fact-metadata')).toBeTruthy())
    expect(screen.queryByTestId('fact-provenance-link')).toBeNull()
  })

  it('reports a missing fact as not found', async () => {
    stubApi({ '/api/memory/facts/nope': { status: 404, json: { error: 'fact not found' } } })
    render(FactDetail, { props: { id: 'nope' } })

    await waitFor(() => expect(screen.getByTestId('fact-missing')).toBeTruthy())
    expect(screen.queryByTestId('fact-error')).toBeNull()
  })
})
