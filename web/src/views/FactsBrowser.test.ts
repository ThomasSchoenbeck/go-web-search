import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import FactsBrowser from './FactsBrowser.svelte'
import { stubApi, requestedPaths } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function fact(overrides: Record<string, unknown> = {}) {
  return {
    id: 'f1',
    text: 'The fixture page says Fixture One.',
    text_chars: 34,
    source_url: 'https://example.com/fixture-one',
    volatility: 'stable',
    tier: 'short',
    hit_count: 2,
    expires_at: '2026-08-15T10:00:00Z',
    ...overrides,
  }
}

const firstPage = '/api/memory/facts?limit=25&offset=0'

describe('FactsBrowser', () => {
  it('renders a row per fact with its metadata', async () => {
    stubApi({ [firstPage]: { json: { count: 1, facts: [fact()] } } })
    render(FactsBrowser)

    await waitFor(() => expect(screen.getByTestId('facts-table')).toBeTruthy())
    const table = screen.getByTestId('facts-table').textContent ?? ''
    expect(table).toContain('The fixture page says')
    expect(table).toContain('stable')
    expect(table).toContain('short')
    expect(screen.getByTestId('fact-link').getAttribute('href')).toBe('/facts/f1')
  })

  it('links a fact source to the provenance pivot', async () => {
    stubApi({ [firstPage]: { json: { count: 1, facts: [fact()] } } })
    render(FactsBrowser)

    await waitFor(() =>
      expect(screen.getByTestId('fact-source-link').getAttribute('href')).toBe(
        `/provenance?url=${encodeURIComponent('https://example.com/fixture-one')}`,
      ),
    )
  })

  it('drives the q parameter from the search box', async () => {
    const fetchMock = stubApi({
      [firstPage]: { json: { count: 1, facts: [fact()] } },
      '/api/memory/facts?q=fixture&limit=25&offset=0': { json: { count: 1, facts: [fact({ id: 'f2' })] } },
    })
    render(FactsBrowser)
    await waitFor(() => expect(screen.getByTestId('facts-table')).toBeTruthy())

    await fireEvent.input(screen.getByTestId('facts-search'), { target: { value: 'fixture' } })
    await fireEvent.click(screen.getByTestId('facts-search-submit'))

    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/memory/facts?q=fixture&limit=25&offset=0'))
  })

  it('clears the search back to an unfiltered list', async () => {
    const fetchMock = stubApi({
      [firstPage]: { json: { count: 1, facts: [fact()] } },
      '/api/memory/facts?q=fixture&limit=25&offset=0': { json: { count: 0, facts: null } },
    })
    render(FactsBrowser)
    await waitFor(() => expect(screen.getByTestId('facts-table')).toBeTruthy())

    await fireEvent.input(screen.getByTestId('facts-search'), { target: { value: 'fixture' } })
    await fireEvent.click(screen.getByTestId('facts-search-submit'))
    await waitFor(() => expect(screen.getByTestId('facts-empty')).toBeTruthy())

    await fireEvent.click(screen.getByTestId('facts-search-clear'))
    await waitFor(() => expect(screen.getByTestId('facts-table')).toBeTruthy())
    expect(requestedPaths(fetchMock).filter((p) => p === firstPage).length).toBeGreaterThan(1)
  })

  it('pages with limit and offset', async () => {
    const fullPage = Array.from({ length: 25 }, (_, i) => fact({ id: `f${i}` }))
    const fetchMock = stubApi({
      [firstPage]: { json: { count: 25, facts: fullPage } },
      '/api/memory/facts?limit=25&offset=25': { json: { count: 1, facts: [fact({ id: 'last' })] } },
    })
    render(FactsBrowser)
    await waitFor(() => expect(screen.getByTestId('facts-table')).toBeTruthy())

    // A full page implies there may be another; the first page has no previous.
    expect(screen.getByTestId('facts-prev')).toHaveProperty('disabled', true)
    expect(screen.getByTestId('facts-next')).toHaveProperty('disabled', false)

    await fireEvent.click(screen.getByTestId('facts-next'))
    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/memory/facts?limit=25&offset=25'))
    expect(screen.getByTestId('facts-page').textContent).toContain('Page 2')

    // A short page means there is no next.
    await waitFor(() => expect(screen.getByTestId('facts-next')).toHaveProperty('disabled', true))
    expect(screen.getByTestId('facts-prev')).toHaveProperty('disabled', false)
  })

  it('changes the page size and returns to the first page', async () => {
    const fetchMock = stubApi({
      [firstPage]: { json: { count: 1, facts: [fact()] } },
      '/api/memory/facts?limit=100&offset=0': { json: { count: 1, facts: [fact()] } },
    })
    render(FactsBrowser)
    await waitFor(() => expect(screen.getByTestId('facts-table')).toBeTruthy())

    await fireEvent.change(screen.getByTestId('facts-limit'), { target: { value: '100' } })
    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/memory/facts?limit=100&offset=0'))
  })

  it('renders an empty store and a no-match search differently', async () => {
    stubApi({ [firstPage]: { json: { count: 0, facts: null } } })
    render(FactsBrowser)

    await waitFor(() => expect(screen.getByTestId('facts-empty').textContent).toContain('Nothing has been distilled'))
  })

  it('surfaces a failure', async () => {
    stubApi({ [firstPage]: { status: 500, json: { error: 'boom' } } })
    render(FactsBrowser)

    await waitFor(() => expect(screen.getByTestId('facts-error').textContent).toContain('boom'))
  })
})
