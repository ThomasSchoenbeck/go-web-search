import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import LogsViewer from './LogsViewer.svelte'
import { stubApi, requestedPaths, type StubRoute } from '../lib/apiStub'
import { resetUIConfig } from '../lib/uiconfig'

beforeEach(() => resetUIConfig())

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function line(overrides: Record<string, unknown> = {}) {
  return {
    id: 'l1',
    run_id: '00000000-0000-7000-8000-000000000001',
    level: 'info',
    source: 'harvester',
    message: 'fixture run started',
    created_at: '2026-08-06T10:00:00Z',
    ...overrides,
  }
}

const firstPage = '/api/logs?limit=50&offset=0'

function stubLogs(routes: Record<string, StubRoute>) {
  return stubApi({
    '/api/ui-config': { json: { poll_interval_ms: 5000, poll_enabled: false, projection_sample_cap: 10 } },
    ...routes,
  })
}

describe('LogsViewer', () => {
  it('renders a row per line with every field', async () => {
    stubLogs({ [firstPage]: { json: { count: 1, entries: [line()] } } })
    render(LogsViewer)

    await waitFor(() => expect(screen.getByTestId('logs-table')).toBeTruthy())
    const table = screen.getByTestId('logs-table').textContent ?? ''
    expect(table).toContain('info')
    expect(table).toContain('harvester')
    expect(table).toContain('fixture run started')
    expect(screen.getByTestId('log-run-link').getAttribute('href')).toBe(
      '/runs/00000000-0000-7000-8000-000000000001',
    )
  })

  it('distinguishes levels so a scan finds the errors', async () => {
    stubLogs({
      [firstPage]: {
        json: {
          count: 2,
          entries: [line({ id: 'l2', level: 'error', message: 'ERROR: boom', run_id: undefined }), line()],
        },
      },
    })
    render(LogsViewer)

    await waitFor(() => expect(screen.getAllByTestId('log-row')).toHaveLength(2))
    const rows = screen.getAllByTestId('log-row')
    expect(rows[0].className).toContain('level-error')
    expect(rows[1].className).toContain('level-info')
  })

  it('drives the level filter from the dropdown', async () => {
    const fetchMock = stubLogs({
      [firstPage]: { json: { count: 1, entries: [line()] } },
      '/api/logs?level=error&limit=50&offset=0': { json: { count: 1, entries: [line({ level: 'error' })] } },
    })
    render(LogsViewer)
    await waitFor(() => expect(screen.getByTestId('logs-table')).toBeTruthy())

    await fireEvent.change(screen.getByTestId('logs-level'), { target: { value: 'error' } })
    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/logs?level=error&limit=50&offset=0'))
  })

  it('drives the run and source filters from the form, and clears them', async () => {
    const filtered = '/api/logs?run_id=run-1&source=scraper&limit=50&offset=0'
    const fetchMock = stubLogs({
      [firstPage]: { json: { count: 1, entries: [line()] } },
      [filtered]: { json: { count: 0, entries: null } },
    })
    render(LogsViewer)
    await waitFor(() => expect(screen.getByTestId('logs-table')).toBeTruthy())

    await fireEvent.input(screen.getByTestId('logs-run'), { target: { value: 'run-1' } })
    await fireEvent.input(screen.getByTestId('logs-source'), { target: { value: 'scraper' } })
    await fireEvent.click(screen.getByTestId('logs-submit'))

    await waitFor(() => expect(requestedPaths(fetchMock)).toContain(filtered))
    expect(screen.getByTestId('logs-empty').textContent).toContain('No log lines match')

    await fireEvent.click(screen.getByTestId('logs-clear'))
    await waitFor(() => expect(screen.getByTestId('logs-table')).toBeTruthy())
  })

  it('pages through the tail', async () => {
    const fullPage = Array.from({ length: 50 }, (_, i) => line({ id: `l${i}` }))
    const fetchMock = stubLogs({
      [firstPage]: { json: { count: 50, entries: fullPage } },
      '/api/logs?limit=50&offset=50': { json: { count: 1, entries: [line({ id: 'last' })] } },
    })
    render(LogsViewer)
    await waitFor(() => expect(screen.getByTestId('logs-table')).toBeTruthy())

    expect(screen.getByTestId('logs-prev')).toHaveProperty('disabled', true)
    await fireEvent.click(screen.getByTestId('logs-next'))

    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/logs?limit=50&offset=50'))
    expect(screen.getByTestId('logs-page').textContent).toContain('Page 2')
  })

  it('renders an empty log database cleanly', async () => {
    stubLogs({ [firstPage]: { json: { count: 0, entries: null } } })
    render(LogsViewer)

    await waitFor(() => expect(screen.getByTestId('logs-empty').textContent).toContain('Nothing has been logged'))
  })

  it('surfaces a failure', async () => {
    stubLogs({ [firstPage]: { status: 500, json: { error: 'boom' } } })
    render(LogsViewer)

    await waitFor(() => expect(screen.getByTestId('logs-error').textContent).toContain('boom'))
  })

  it('offers the polling controls', async () => {
    stubLogs({ [firstPage]: { json: { count: 1, entries: [line()] } } })
    render(LogsViewer)

    await waitFor(() => expect(screen.getByTestId('poll-toggle')).toHaveProperty('disabled', false))
  })
})
