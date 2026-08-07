import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/svelte'
import SearchesList from './SearchesList.svelte'
import { stubApi } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const searches = [
  {
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
  },
  {
    id: 'search-2',
    run_id: 'run-1',
    term: 'fixture term',
    engine: 'bing',
    search_mode: 'direct',
    http_status: 429,
    blocked: true,
    anchor_count: 0,
    error: 'challenged by engine',
    duration_ms: 91,
    created_at: '2026-08-05T10:02:00Z',
  },
]

describe('SearchesList', () => {
  it('renders every metadata field the task requires', async () => {
    stubApi({ '/api/runs/run-1/searches': { json: searches } })
    render(SearchesList, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('searches-table')).toBeTruthy())
    const table = screen.getByTestId('searches-table').textContent ?? ''

    expect(table).toContain('google')
    expect(table).toContain('typed')
    expect(table).toContain('200')
    expect(table).toContain('12')
    expect(table).toContain('1.5s')
    expect(table).toContain('429')
    expect(table).toContain('challenged by engine')
  })

  it('renders blocked as a yes/no rather than a raw boolean', async () => {
    stubApi({ '/api/runs/run-1/searches': { json: searches } })
    render(SearchesList, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getAllByTestId('search-row')).toHaveLength(2))
    const cells = screen.getAllByTestId('search-row').map((row) => row.textContent ?? '')
    expect(cells[0]).toContain('no')
    expect(cells[1]).toContain('yes')
  })

  it('links each search to its SERP and back to the run', async () => {
    stubApi({ '/api/runs/run-1/searches': { json: searches } })
    render(SearchesList, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getAllByTestId('serp-link')).toHaveLength(2))
    expect(screen.getAllByTestId('serp-link')[0].getAttribute('href')).toBe('/searches/search-1')
    expect(screen.getByTestId('back-to-run').getAttribute('href')).toBe('/runs/run-1')
  })

  it('renders an empty state', async () => {
    stubApi({ '/api/runs/run-1/searches': { json: null } })
    render(SearchesList, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('searches-empty')).toBeTruthy())
  })

  it('surfaces a failure', async () => {
    stubApi({ '/api/runs/run-1/searches': { status: 500, json: { error: 'boom' } } })
    render(SearchesList, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('searches-error').textContent).toContain('boom'))
  })
})
