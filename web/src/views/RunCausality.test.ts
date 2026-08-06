import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor } from '@testing-library/svelte'
import RunCausality from './RunCausality.svelte'
import { stubApi } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const path = '/api/runs/run-1/causality'

const graph = {
  run_id: 'run-1',
  nodes: [
    { id: 'search:s1', kind: 'search', ref_id: 's1', label: 'google · fixture term', detail: 'typed' },
    { id: 'url:u1', kind: 'url', ref_id: 'u1', label: 'https://a.example', url: 'https://a.example' },
    { id: 'url:u2', kind: 'url', ref_id: 'u2', label: 'https://b.example', url: 'https://b.example' },
    { id: 'scrape:c1', kind: 'scrape', ref_id: 'c1', label: 'A page', detail: 'HTTP 200', url: 'https://a.example' },
    { id: 'fact:f1', kind: 'fact', ref_id: 'f1', label: 'A distilled fact', url: 'https://a.example', has_vector: true },
  ],
  edges: [
    { from: 'search:s1', to: 'url:u1', rank: 1 },
    { from: 'search:s1', to: 'url:u2', rank: 2 },
    { from: 'url:u1', to: 'scrape:c1' },
    { from: 'url:u1', to: 'fact:f1' },
  ],
  truncated: false,
  limit: 200,
  vectors_available: true,
}

describe('RunCausality', () => {
  it('summarises the graph', async () => {
    stubApi({ [path]: { json: graph } })
    render(RunCausality, { props: { runId: 'run-1' } })

    await waitFor(() =>
      expect(screen.getByTestId('causality-counts').textContent).toContain('1 searches · 2 URLs · 1 scrapes · 1 facts'),
    )
  })

  it('renders the chain with ranks and cross-links', async () => {
    stubApi({ [path]: { json: graph } })
    render(RunCausality, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('causality-search')).toBeTruthy())

    expect(screen.getByTestId('causality-search-link').getAttribute('href')).toBe('/searches/s1')
    expect(screen.getAllByTestId('causality-rank')[0].textContent).toBe('#1')
    expect(screen.getAllByTestId('causality-url-link')[0].getAttribute('href')).toBe(
      `/provenance?url=${encodeURIComponent('https://a.example')}`,
    )
    expect(screen.getByTestId('causality-scrape-link').getAttribute('href')).toBe('/scrapes/c1')
    expect(screen.getByTestId('causality-fact-link').getAttribute('href')).toBe('/facts/f1')
    expect(screen.getByTestId('causality-fact-vector').textContent).toContain('embedded')
  })

  it('marks a url that was never scraped', async () => {
    stubApi({ [path]: { json: graph } })
    render(RunCausality, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('causality-no-scrape')).toBeTruthy())
  })

  it('explains a truncated graph and names the config key', async () => {
    stubApi({ [path]: { json: { ...graph, truncated: true, limit: 1 } } })
    render(RunCausality, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('causality-truncated')).toBeTruthy())
    expect(screen.getByTestId('causality-truncated').textContent).toContain('causality_max_urls')
  })

  it('hides vector markers when the store is unavailable', async () => {
    stubApi({ [path]: { json: { ...graph, vectors_available: false, note: 're-embed in progress' } } })
    render(RunCausality, { props: { runId: 'run-1' } })

    await waitFor(() =>
      expect(screen.getByTestId('causality-vectors-unavailable').textContent).toContain('re-embed in progress'),
    )
    expect(screen.queryByTestId('causality-fact-vector')).toBeNull()
  })

  it('renders an empty run clearly', async () => {
    stubApi({ [path]: { json: { ...graph, nodes: [], edges: [] } } })
    render(RunCausality, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('causality-empty')).toBeTruthy())
  })

  it('tolerates null node and edge arrays', async () => {
    // Go marshals empty slices as null.
    stubApi({ [path]: { json: { ...graph, nodes: null, edges: null } } })
    render(RunCausality, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('causality-empty')).toBeTruthy())
  })

  it('surfaces a failure', async () => {
    stubApi({ [path]: { status: 500, json: { error: 'boom' } } })
    render(RunCausality, { props: { runId: 'run-1' } })

    await waitFor(() => expect(screen.getByTestId('causality-error').textContent).toContain('boom'))
  })
})
