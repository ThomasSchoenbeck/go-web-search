import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import RunsList from './RunsList.svelte'
import { stubApi, requestedPaths } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const runs = [
  {
    id: 'run-1',
    mode: 'serve',
    started_at: '2026-08-05T10:00:00Z',
    finished_at: '2026-08-05T10:05:00Z',
    searches: 2,
    urls: 4,
    scrapes: 1,
  },
  {
    id: 'run-2',
    mode: 'browse',
    started_at: '2026-08-05T11:00:00Z',
    searches: 0,
    urls: 0,
    scrapes: 0,
  },
]

describe('RunsList', () => {
  it('renders a row per run, linking each to its detail', async () => {
    stubApi({ '/api/runs': { json: runs } })
    render(RunsList)

    await waitFor(() => expect(screen.getByTestId('runs-table')).toBeTruthy())
    const links = screen.getAllByTestId('run-link')
    expect(links).toHaveLength(2)
    expect(links[0].getAttribute('href')).toBe('/runs/run-1')
    expect(screen.getByText('serve')).toBeTruthy()
  })

  it('shows "running" for a run with no finish time', async () => {
    stubApi({ '/api/runs': { json: runs } })
    render(RunsList)

    await waitFor(() => expect(screen.getByText('running')).toBeTruthy())
  })

  it('refetches with the chosen limit', async () => {
    const fetchMock = stubApi({ '/api/runs': { json: runs }, '/api/runs?limit=100': { json: [runs[0]] } })
    render(RunsList)

    await waitFor(() => expect(screen.getAllByTestId('run-link')).toHaveLength(2))

    await fireEvent.change(screen.getByTestId('run-limit'), { target: { value: '100' } })

    await waitFor(() => expect(screen.getAllByTestId('run-link')).toHaveLength(1))
    expect(requestedPaths(fetchMock)).toContain('/api/runs?limit=100')
  })

  it('refetches when reload is clicked', async () => {
    const fetchMock = stubApi({ '/api/runs': { json: runs } })
    render(RunsList)

    await waitFor(() => expect(screen.getByTestId('runs-table')).toBeTruthy())
    const before = fetchMock.mock.calls.length

    await fireEvent.click(screen.getByTestId('runs-reload'))

    await waitFor(() => expect(fetchMock.mock.calls.length).toBeGreaterThan(before))
  })

  it('renders an empty state when the API returns null', async () => {
    // Go marshals an empty slice as null, which must not crash the view.
    stubApi({ '/api/runs': { json: null } })
    render(RunsList)

    await waitFor(() => expect(screen.getByTestId('runs-empty')).toBeTruthy())
  })

  it('surfaces a failure', async () => {
    stubApi({ '/api/runs': { status: 500, json: { error: 'database is gone' } } })
    render(RunsList)

    await waitFor(() => expect(screen.getByTestId('runs-error').textContent).toContain('database is gone'))
  })
})
