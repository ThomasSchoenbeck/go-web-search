import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/svelte'
import StatsDashboard from './StatsDashboard.svelte'
import { stubApi } from '../lib/apiStub'
import type { Stats } from '../lib/api'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function stats(overrides: Partial<Stats> = {}): Stats {
  return {
    runs: 1,
    searches: 2,
    urls: 2,
    scrapes: 3,
    memory_facts: 1,
    search_cache: 2,
    vectors: 2,
    pending_jobs: 1,
    scrape_text_avg_chars: 1100,
    scrape_text_max_chars: 2000,
    scrape_raw_avg_chars: 46_000,
    scrape_raw_max_chars: 90_000,
    embed_model: 'stub-embedder',
    embed_dim: 8,
    vector_table: 'vectors_test',
    vector_migration_in_progress: false,
    search_cache_stats: {
      rows: 2,
      total_hits: 7,
      rows_with_hits: 1,
      expired: 0,
      tiers: { short: 1, long: 1, permanent: 0 },
    },
    scrape_cache_stats: {
      rows: 3,
      total_hits: 8,
      rows_with_hits: 2,
      expired: 1,
      tiers: { short: 2, long: 1, permanent: 0 },
    },
    jobs: {
      by_status: { pending: 1, running: 1, done: 1, failed: 1 },
      by_type: { embed: 2, distill: 1, cleanup: 1 },
      retried: 1,
      max_attempts: 3,
      oldest_pending_at: '2026-08-06T10:00:00Z',
      completed_sampled: 1,
      avg_completion_ms: 90_000,
    },
    ...overrides,
  }
}

describe('StatsDashboard', () => {
  it('renders every count and the size aggregates', async () => {
    stubApi({ '/api/stats': { json: stats() } })
    render(StatsDashboard)

    await waitFor(() => expect(screen.getByTestId('stats-counts')).toBeTruthy())
    expect(screen.getAllByTestId('stats-count')).toHaveLength(8)
    const counts = screen.getByTestId('stats-counts').textContent ?? ''
    expect(counts).toContain('Searches: 2')
    expect(counts).toContain('Vectors: 2')

    const sizes = screen.getByTestId('stats-sizes').textContent ?? ''
    expect(sizes).toContain('1.1k')
    expect(sizes).toContain('90.0k')
  })

  it('shows the active model and dimension', async () => {
    stubApi({ '/api/stats': { json: stats() } })
    render(StatsDashboard)

    await waitFor(() => expect(screen.getByTestId('stats-embedding')).toBeTruthy())
    const embedding = screen.getByTestId('stats-embedding').textContent ?? ''
    expect(embedding).toContain('stub-embedder')
    expect(embedding).toContain('8')
    expect(embedding).toContain('vectors_test')
    expect(screen.queryByTestId('stats-migrating')).toBeNull()
  })

  // Unset meta is "unknown", never a claim the backend did not make.
  it('reports missing embedding meta as unknown', async () => {
    stubApi({
      '/api/stats': { json: stats({ embed_model: undefined, embed_dim: undefined, vector_table: undefined }) },
    })
    render(StatsDashboard)

    await waitFor(() => expect(screen.getByTestId('stats-embedding').textContent).toContain('unknown'))
  })

  it('calls out a migration in progress', async () => {
    stubApi({ '/api/stats': { json: stats({ vector_migration_in_progress: true }) } })
    render(StatsDashboard)

    await waitFor(() => expect(screen.getByTestId('stats-migrating')).toBeTruthy())
    expect(screen.getByTestId('stats-migrating').textContent).toContain('re-embed migration')
  })

  it('breaks both caches down by tier and hit count', async () => {
    stubApi({ '/api/stats': { json: stats() } })
    render(StatsDashboard)

    await waitFor(() => expect(screen.getByTestId('stats-caches')).toBeTruthy())
    expect(screen.getAllByTestId('stats-cache-row')).toHaveLength(2)
    const table = screen.getByTestId('stats-caches').textContent ?? ''
    expect(table).toContain('Search')
    expect(table).toContain('Scrape')
    // Counts, not a rate — the note says why.
    expect(screen.getByTestId('stats-hit-note').textContent).toContain('not a hit rate')
  })

  it('summarises job throughput and timing', async () => {
    stubApi({ '/api/stats': { json: stats() } })
    render(StatsDashboard)

    await waitFor(() => expect(screen.getByTestId('stats-jobs')).toBeTruthy())
    expect(screen.getAllByTestId('stats-job-status')).toHaveLength(4)
    expect(screen.getAllByTestId('stats-job-type')).toHaveLength(3)
    const timing = screen.getByTestId('stats-job-timing').textContent ?? ''
    expect(timing).toContain('3') // most attempts
    expect(timing).toContain('1m 30s')
  })

  it('renders an empty install without inventing numbers', async () => {
    stubApi({
      '/api/stats': {
        json: stats({
          jobs: {
            by_status: { pending: 0, running: 0, done: 0, failed: 0 },
            by_type: {},
            retried: 0,
            max_attempts: 0,
            completed_sampled: 0,
            avg_completion_ms: 0,
          },
        }),
      },
    })
    render(StatsDashboard)

    await waitFor(() => expect(screen.getByTestId('stats-job-types-empty')).toBeTruthy())
    expect(screen.getByTestId('stats-job-timing').textContent).toContain('nothing finished yet')
    expect(screen.getByTestId('stats-job-timing').textContent).toContain('none waiting')
  })

  it('surfaces a failure', async () => {
    stubApi({ '/api/stats': { status: 500, json: { error: 'boom' } } })
    render(StatsDashboard)

    await waitFor(() => expect(screen.getByTestId('stats-error').textContent).toContain('boom'))
  })
})
