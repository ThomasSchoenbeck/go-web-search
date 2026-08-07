import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import SemanticExplorer from './SemanticExplorer.svelte'
import { stubApi } from '../lib/apiStub'

beforeEach(() => window.history.replaceState({}, '', '/explore'))

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  window.history.replaceState({}, '', '/')
})

const query = 'fixture term'
// Built the same way api.ts builds it: URLSearchParams encodes a space as "+",
// not "%20". Go's query parser accepts both.
const path = `/api/explore?${new URLSearchParams({ q: query, k: '10' })}`

const result = {
  query,
  k: 10,
  available: true,
  memory_hits: 1,
  search_hits: 1,
  neighbors: [
    {
      owner_kind: 'memory' as const,
      id: 'f1',
      distance: 0.0123,
      similarity: 0.9877,
      text: 'The fixture page says Fixture One.',
      source_url: 'https://example.com/fixture-one',
      tier: 'short',
    },
    {
      owner_kind: 'search' as const,
      id: 'sc1',
      distance: 0.4211,
      similarity: 0.5789,
      text: 'fixture term',
      tier: 'short',
      result_count: 3,
    },
  ],
}

describe('SemanticExplorer', () => {
  it('prompts before a query is entered', () => {
    stubApi({})
    render(SemanticExplorer, { props: { query: '', k: 10 } })

    expect(screen.getByTestId('explore-prompt')).toBeTruthy()
  })

  it('renders neighbours with distance and similarity, nearest first', async () => {
    stubApi({ [path]: { json: result } })
    render(SemanticExplorer, { props: { query, k: 10 } })

    await waitFor(() => expect(screen.getAllByTestId('explore-row')).toHaveLength(2))
    const distances = screen.getAllByTestId('explore-distance').map((el) => Number(el.textContent))
    expect(distances[0]).toBeLessThan(distances[1])
    expect(screen.getAllByTestId('explore-distance')[0].textContent).toBe('0.0123')
  })

  it('tags each neighbour by owner kind', async () => {
    stubApi({ [path]: { json: result } })
    render(SemanticExplorer, { props: { query, k: 10 } })

    await waitFor(() => expect(screen.getAllByTestId('explore-kind')).toHaveLength(2))
    const kinds = screen.getAllByTestId('explore-kind').map((el) => el.textContent)
    expect(kinds).toEqual(['fact', 'cached search'])
  })

  it('links memory neighbours to their fact and source', async () => {
    stubApi({ [path]: { json: result } })
    render(SemanticExplorer, { props: { query, k: 10 } })

    await waitFor(() => expect(screen.getByTestId('explore-fact-link').getAttribute('href')).toBe('/facts/f1'))
    expect(screen.getByTestId('explore-source-link').getAttribute('href')).toBe(
      `/provenance?url=${encodeURIComponent('https://example.com/fixture-one')}`,
    )
  })

  it('shows cached-search context instead of a fact link', async () => {
    stubApi({ [path]: { json: result } })
    render(SemanticExplorer, { props: { query, k: 10 } })

    await waitFor(() => expect(screen.getByTestId('explore-search-context').textContent).toContain('3 results'))
  })

  it('puts the query and k in the address bar so a probe can be linked to', async () => {
    stubApi({})
    render(SemanticExplorer, { props: { query: '', k: 10 } })

    await fireEvent.input(screen.getByTestId('explore-input'), { target: { value: 'something else' } })
    await fireEvent.change(screen.getByTestId('explore-k'), { target: { value: '25' } })
    await fireEvent.click(screen.getByTestId('explore-submit'))

    expect(window.location.search).toContain(encodeURIComponent('something else'))
    expect(window.location.search).toContain('k=25')
  })

  it('reports an unavailable vector store as a message, not an error', async () => {
    stubApi({
      [path]: {
        json: { ...result, available: false, neighbors: [], note: 'a re-embed migration is in progress' },
      },
    })
    render(SemanticExplorer, { props: { query, k: 10 } })

    await waitFor(() =>
      expect(screen.getByTestId('explore-unavailable').textContent).toContain('re-embed migration'),
    )
    expect(screen.queryByTestId('explore-error')).toBeNull()
    expect(screen.queryByTestId('explore-results')).toBeNull()
  })

  it('distinguishes "nothing near" from "store unavailable"', async () => {
    stubApi({ [path]: { json: { ...result, neighbors: [], memory_hits: 0, search_hits: 0 } } })
    render(SemanticExplorer, { props: { query, k: 10 } })

    await waitFor(() => expect(screen.getByTestId('explore-empty')).toBeTruthy())
    expect(screen.queryByTestId('explore-unavailable')).toBeNull()
  })

  it('tolerates a null neighbours array', async () => {
    stubApi({ [path]: { json: { ...result, neighbors: null } } })
    render(SemanticExplorer, { props: { query, k: 10 } })

    await waitFor(() => expect(screen.getByTestId('explore-empty')).toBeTruthy())
  })

  it('surfaces a failure', async () => {
    stubApi({ [path]: { status: 500, json: { error: 'boom' } } })
    render(SemanticExplorer, { props: { query, k: 10 } })

    await waitFor(() => expect(screen.getByTestId('explore-error').textContent).toContain('boom'))
  })
})
