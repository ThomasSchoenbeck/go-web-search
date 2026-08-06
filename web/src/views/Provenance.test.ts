import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import Provenance from './Provenance.svelte'
import { stubApi, requestedPaths } from '../lib/apiStub'

beforeEach(() => window.history.replaceState({}, '', '/provenance'))

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  window.history.replaceState({}, '', '/')
})

const url = 'https://example.com/fixture-one'

const chain = {
  url,
  url_id: 'u1',
  known: true,
  found_by: [
    {
      search_id: 's1',
      run_id: 'run-1',
      term: 'fixture term',
      engine: 'google',
      search_mode: 'typed',
      rank: 1,
      created_at: '2026-08-05T10:01:00Z',
    },
  ],
  scrape: {
    scrape_id: 'c1',
    url,
    title: 'Fixture One',
    http_status: 200,
    fetched_with: 'http',
    text_chars: 11,
    clean_html_chars: 20,
    raw_html_chars: 46,
    created_at: '2026-08-05T10:03:00Z',
  },
  facts: [
    {
      id: 'f1',
      text: 'The fixture page says Fixture One.',
      source_url: url,
      volatility: 'stable',
      tier: 'short',
      has_vector: true,
      created_at: '2026-08-05T10:04:00Z',
    },
  ],
  vectors_available: true,
}

const path = `/api/provenance?url=${encodeURIComponent(url)}`

describe('Provenance', () => {
  it('prompts for a URL when none is pivoted on', () => {
    stubApi({})
    render(Provenance, { props: { url: '' } })

    expect(screen.getByTestId('provenance-prompt')).toBeTruthy()
  })

  it('renders the backward chain with rank and run links', async () => {
    stubApi({ [path]: { json: chain } })
    render(Provenance, { props: { url } })

    await waitFor(() => expect(screen.getByTestId('found-by-table')).toBeTruthy())
    const table = screen.getByTestId('found-by-table').textContent ?? ''
    expect(table).toContain('google')
    expect(table).toContain('fixture term')
    expect(screen.getByTestId('found-run-link').getAttribute('href')).toBe('/runs/run-1')
    expect(screen.getByTestId('found-search-link').getAttribute('href')).toBe('/searches/s1')
  })

  it('renders the forward chain: scrape then facts', async () => {
    stubApi({ [path]: { json: chain } })
    render(Provenance, { props: { url } })

    await waitFor(() => expect(screen.getByTestId('provenance-scrape')).toBeTruthy())
    expect(screen.getByTestId('provenance-scrape-link').getAttribute('href')).toBe('/scrapes/c1')
    expect(screen.getByTestId('fact-link').getAttribute('href')).toBe('/facts/f1')
    expect(screen.getByTestId('fact-vector').textContent).toContain('embedded')
  })

  it('offers the whole-run causality graph', async () => {
    stubApi({ [path]: { json: chain } })
    render(Provenance, { props: { url } })

    await waitFor(() =>
      expect(screen.getByTestId('causality-link').getAttribute('href')).toBe('/runs/run-1/causality'),
    )
  })

  it('reports vector presence as unknown when the store is unavailable', async () => {
    stubApi({
      [path]: {
        json: { ...chain, vectors_available: false, note: 're-embed in progress', facts: [{ ...chain.facts[0], has_vector: false }] },
      },
    })
    render(Provenance, { props: { url } })

    await waitFor(() => expect(screen.getByTestId('vectors-unavailable').textContent).toContain('re-embed in progress'))
    // "not embedded" would be a claim the backend explicitly did not make.
    expect(screen.getByTestId('fact-vector-unknown')).toBeTruthy()
    expect(screen.queryByTestId('fact-vector')).toBeNull()
  })

  it('renders the degraded cases: no scrape, no facts, unknown url', async () => {
    stubApi({
      [path]: { json: { url, known: false, found_by: [], facts: [], vectors_available: true } },
    })
    render(Provenance, { props: { url } })

    await waitFor(() => expect(screen.getByTestId('provenance-unknown')).toBeTruthy())
    expect(screen.getByTestId('found-by-empty')).toBeTruthy()
    expect(screen.getByTestId('scrape-none')).toBeTruthy()
    expect(screen.getByTestId('facts-empty')).toBeTruthy()
  })

  it('pivots to a new URL through the address bar', async () => {
    const fetchMock = stubApi({ [path]: { json: chain } })
    render(Provenance, { props: { url: '' } })

    const input = screen.getByTestId('provenance-input')
    await fireEvent.input(input, { target: { value: 'https://other.example/page' } })
    await fireEvent.click(screen.getByTestId('provenance-submit'))

    // The pivot lives in the URL so the chain can be linked to.
    expect(window.location.search).toContain(encodeURIComponent('https://other.example/page'))
    expect(requestedPaths(fetchMock)).not.toContain(path)
  })

  it('surfaces a failure', async () => {
    stubApi({ [path]: { status: 500, json: { error: 'boom' } } })
    render(Provenance, { props: { url } })

    await waitFor(() => expect(screen.getByTestId('provenance-error').textContent).toContain('boom'))
  })
})
