import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import JobsMonitor from './JobsMonitor.svelte'
import { stubApi, requestedPaths, type StubRoute } from '../lib/apiStub'
import { resetUIConfig } from '../lib/uiconfig'

beforeEach(() => resetUIConfig())

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function job(overrides: Record<string, unknown> = {}) {
  return {
    id: 'j1',
    type: 'distill',
    payload: '{"scrape_id":"s1"}',
    status: 'pending',
    attempts: 0,
    run_after: '2026-08-06T10:00:00Z',
    created_at: '2026-08-06T09:59:00Z',
    updated_at: '2026-08-06T09:59:00Z',
    ...overrides,
  }
}

const counts = { pending: 1, running: 2, done: 3, failed: 4 }
const firstPage = '/api/jobs?limit=25&offset=0'

// Every view that embeds the polling controls also reads the session defaults.
function stubJobs(routes: Record<string, StubRoute>) {
  return stubApi({
    '/api/ui-config': { json: { poll_interval_ms: 5000, poll_enabled: false, projection_sample_cap: 10 } },
    ...routes,
  })
}

describe('JobsMonitor', () => {
  it('renders a row per job with its queue metadata', async () => {
    stubJobs({ [firstPage]: { json: { jobs: [job({ attempts: 2, locked_at: '2026-08-06T10:01:00Z' })], counts } } })
    render(JobsMonitor)

    await waitFor(() => expect(screen.getByTestId('jobs-table')).toBeTruthy())
    const table = screen.getByTestId('jobs-table').textContent ?? ''
    expect(table).toContain('distill')
    expect(table).toContain('pending')
    expect(table).toContain('2') // attempts
    expect(screen.getByTestId('job-payload').textContent).toContain('scrape_id')
  })

  it('shows the whole-queue breakdown, not just the page', async () => {
    stubJobs({ [firstPage]: { json: { jobs: [job()], counts } } })
    render(JobsMonitor)

    await waitFor(() => expect(screen.getByTestId('jobs-breakdown')).toBeTruthy())
    expect(screen.getByTestId('jobs-count-running').textContent).toContain('2')
    expect(screen.getByTestId('jobs-count-failed').textContent).toContain('4')
  })

  it('drives the status and type filters from the dropdowns', async () => {
    const fetchMock = stubJobs({
      [firstPage]: { json: { jobs: [job()], counts } },
      '/api/jobs?status=failed&limit=25&offset=0': { json: { jobs: [job({ status: 'failed' })], counts } },
      '/api/jobs?status=failed&type=embed&limit=25&offset=0': { json: { jobs: null, counts } },
    })
    render(JobsMonitor)
    await waitFor(() => expect(screen.getByTestId('jobs-table')).toBeTruthy())

    await fireEvent.change(screen.getByTestId('jobs-status'), { target: { value: 'failed' } })
    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/jobs?status=failed&limit=25&offset=0'))

    await fireEvent.change(screen.getByTestId('jobs-type'), { target: { value: 'embed' } })
    await waitFor(() =>
      expect(requestedPaths(fetchMock)).toContain('/api/jobs?status=failed&type=embed&limit=25&offset=0'),
    )
    // A filter that matches nothing reads as "no match", not as an empty queue.
    await waitFor(() => expect(screen.getByTestId('jobs-empty').textContent).toContain('No jobs match'))
  })

  it('pages with limit and offset', async () => {
    const fullPage = Array.from({ length: 25 }, (_, i) => job({ id: `j${i}` }))
    const fetchMock = stubJobs({
      [firstPage]: { json: { jobs: fullPage, counts } },
      '/api/jobs?limit=25&offset=25': { json: { jobs: [job({ id: 'last' })], counts } },
    })
    render(JobsMonitor)
    await waitFor(() => expect(screen.getByTestId('jobs-table')).toBeTruthy())

    expect(screen.getByTestId('jobs-prev')).toHaveProperty('disabled', true)
    await fireEvent.click(screen.getByTestId('jobs-next'))

    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/jobs?limit=25&offset=25'))
    expect(screen.getByTestId('jobs-page').textContent).toContain('Page 2')
    await waitFor(() => expect(screen.getByTestId('jobs-next')).toHaveProperty('disabled', true))

    await fireEvent.click(screen.getByTestId('jobs-prev'))
    await waitFor(() => expect(screen.getByTestId('jobs-page').textContent).toContain('Page 1'))
  })

  it('renders an empty queue cleanly', async () => {
    stubJobs({ [firstPage]: { json: { jobs: null, counts: {} } } })
    render(JobsMonitor)

    await waitFor(() => expect(screen.getByTestId('jobs-empty').textContent).toContain('queue is empty'))
    // A status nothing holds still gets a tile, at zero.
    expect(screen.getByTestId('jobs-count-pending').textContent).toContain('0')
  })

  it('surfaces a failure', async () => {
    stubJobs({ [firstPage]: { status: 500, json: { error: 'boom' } } })
    render(JobsMonitor)

    await waitFor(() => expect(screen.getByTestId('jobs-error').textContent).toContain('boom'))
  })

  it('offers the polling controls', async () => {
    stubJobs({ [firstPage]: { json: { jobs: [job()], counts } } })
    render(JobsMonitor)

    await waitFor(() => expect(screen.getByTestId('poll-toggle')).toHaveProperty('disabled', false))
    expect(screen.getByTestId('poll-status').textContent).toContain('paused')
  })
})
