import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/svelte'
import RunDetail from './RunDetail.svelte'
import { stubApi } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const run = {
  id: 'run-1',
  mode: 'serve',
  started_at: '2026-08-05T10:00:00Z',
  finished_at: '2026-08-05T10:05:00Z',
  searches: 1,
  urls: 2,
  scrapes: 1,
}

const search = {
  id: 'search-1',
  run_id: 'run-1',
  term: 'fixture term',
  engine: 'google',
  search_mode: 'typed',
  http_status: 200,
  blocked: false,
  anchor_count: 12,
  duration_ms: 1500,
  created_at: '2026-08-05T10:01:00Z',
}

function stubFullRun(): void {
  stubApi({
    '/api/runs/run-1': { json: run },
    '/api/runs/run-1/urls': {
      json: [{ id: 'url-1', url: 'https://example.com/one', domain: 'example.com', rank: 1 }],
    },
    '/api/runs/run-1/searches': { json: [search] },
    '/api/runs/run-1/scrapes': { json: { scrape_ids: ['scrape-1'] } },
  })
}

describe('RunDetail', () => {
  it('renders the run summary', async () => {
    stubFullRun()
    render(RunDetail, { props: { id: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('run-summary')).toBeTruthy())
    expect(screen.getByTestId('run-summary').textContent).toContain('serve')
    expect(screen.getByTestId('run-summary').textContent).toContain('1 searches · 2 URLs · 1 scrapes')
  })

  it('renders all three child lists and links onward', async () => {
    stubFullRun()
    render(RunDetail, { props: { id: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('searches-list')).toBeTruthy())
    expect(screen.getByTestId('search-link').getAttribute('href')).toBe('/searches/search-1')
    expect(screen.getByTestId('urls-table').textContent).toContain('example.com')
    expect(screen.getByTestId('scrape-link').getAttribute('href')).toBe('/scrapes/scrape-1')
    expect(screen.getByTestId('all-searches-link').getAttribute('href')).toBe('/runs/run-1/searches')
  })

  it('renders empty children gracefully', async () => {
    stubApi({
      '/api/runs/run-1': { json: { ...run, searches: 0, urls: 0, scrapes: 0 } },
      '/api/runs/run-1/urls': { json: null },
      '/api/runs/run-1/searches': { json: null },
      '/api/runs/run-1/scrapes': { json: { scrape_ids: null } },
    })
    render(RunDetail, { props: { id: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('searches-empty')).toBeTruthy())
    expect(screen.getByTestId('urls-empty')).toBeTruthy()
    expect(screen.getByTestId('scrapes-empty')).toBeTruthy()
  })

  it('marks a blocked search', async () => {
    stubApi({
      '/api/runs/run-1': { json: run },
      '/api/runs/run-1/urls': { json: null },
      '/api/runs/run-1/searches': { json: [{ ...search, blocked: true }] },
      '/api/runs/run-1/scrapes': { json: { scrape_ids: null } },
    })
    render(RunDetail, { props: { id: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('search-blocked')).toBeTruthy())
  })

  it('surfaces a missing run', async () => {
    stubApi({ '/api/runs/nope': { status: 404, json: { error: 'sql: no rows in result set' } } })
    render(RunDetail, { props: { id: 'nope' } })

    await waitFor(() => expect(screen.getByTestId('run-error')).toBeTruthy())
  })
})
