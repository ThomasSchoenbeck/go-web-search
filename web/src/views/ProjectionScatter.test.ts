import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import ProjectionScatter from './ProjectionScatter.svelte'
import { stubApi, requestedPaths, type StubRoute } from '../lib/apiStub'
import { resetUIConfig } from '../lib/uiconfig'

beforeEach(() => resetUIConfig())

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

function point(overrides: Record<string, unknown> = {}) {
  return {
    id: 'f1',
    owner_kind: 'memory',
    label: 'The fixture page says Fixture One.',
    source_url: 'https://example.com/fixture-one',
    vector: [1, 0, 0, 0],
    ...overrides,
  }
}

const searchPoint = point({
  id: 'c1',
  owner_kind: 'search',
  label: 'fixture term',
  source_url: undefined,
  vector: [0, 1, 0, 0],
})

function dump(overrides: Record<string, unknown> = {}) {
  return {
    available: true,
    model: 'stub-embedder',
    dim: 4,
    limit: 2000,
    offset: 0,
    total: { memory: 1, search: 1 },
    truncated: false,
    points: [point(), searchPoint],
    ...overrides,
  }
}

// The cap the view asks for comes from /api/ui-config, so every test serves it.
function stubProjection(routes: Record<string, StubRoute>) {
  return stubApi({
    '/api/ui-config': { json: { poll_interval_ms: 5000, poll_enabled: false, projection_sample_cap: 2000 } },
    ...routes,
  })
}

const dumpPath = '/api/projection?limit=2000&offset=0'

describe('ProjectionScatter', () => {
  it('requests the sample cap the config supplies, not a constant', async () => {
    const fetchMock = stubProjection({
      '/api/ui-config': { json: { poll_interval_ms: 5000, poll_enabled: false, projection_sample_cap: 50 } },
      '/api/projection?limit=50&offset=0': { json: dump() },
    })
    render(ProjectionScatter)

    await waitFor(() => expect(requestedPaths(fetchMock)).toContain('/api/projection?limit=50&offset=0'))
  })

  it('plots one point per vector, tagged by owner kind', async () => {
    stubProjection({ [dumpPath]: { json: dump() } })
    render(ProjectionScatter)

    await waitFor(() => expect(screen.getByTestId('projection-plot')).toBeTruthy())
    const points = screen.getAllByTestId('projection-point')
    expect(points).toHaveLength(2)
    expect(points.map((p) => p.getAttribute('data-kind')).sort()).toEqual(['memory', 'search'])
    // The layout must place them, not stack them at NaN.
    for (const circle of points) {
      expect(Number.isFinite(Number(circle.getAttribute('cx')))).toBe(true)
      expect(Number.isFinite(Number(circle.getAttribute('cy')))).toBe(true)
    }
  })

  it('says how much of the space it is showing', async () => {
    stubProjection({ [dumpPath]: { json: dump() } })
    render(ProjectionScatter)

    await waitFor(() => expect(screen.getByTestId('projection-summary')).toBeTruthy())
    const summary = screen.getByTestId('projection-summary').textContent ?? ''
    expect(summary).toContain('Showing 2 of 2')
    expect(summary).toContain('PCA')
    // Two dimensions cannot hold a 4096-dimensional space; the view must not
    // let a viewer read exact distances into the picture.
    expect(summary).toContain('indicative')
  })

  it('reports a capped dump as capped', async () => {
    stubProjection({ [dumpPath]: { json: dump({ truncated: true, limit: 2, total: { memory: 40, search: 10 } }) } })
    render(ProjectionScatter)

    await waitFor(() => expect(screen.getByTestId('projection-summary').textContent).toContain('Showing 2 of 50'))
    expect(screen.getByTestId('projection-summary').textContent).toContain('capped at 2')
  })

  it('selecting a memory point shows its label and links out', async () => {
    stubProjection({ [dumpPath]: { json: dump() } })
    render(ProjectionScatter)
    await waitFor(() => expect(screen.getByTestId('projection-plot')).toBeTruthy())

    expect(screen.getByTestId('projection-prompt')).toBeTruthy()
    const memory = screen.getAllByTestId('projection-point').find((p) => p.getAttribute('data-kind') === 'memory')
    await fireEvent.click(memory!)

    await waitFor(() => expect(screen.getByTestId('projection-selection')).toBeTruthy())
    expect(screen.getByTestId('projection-selected-kind').textContent).toContain('Memory fact')
    expect(screen.getByTestId('projection-selected-label').textContent).toContain('Fixture One')
    expect(screen.getByTestId('projection-fact-link').getAttribute('href')).toBe('/facts/f1')
    expect(screen.getByTestId('projection-source-link').getAttribute('href')).toBe(
      `/provenance?url=${encodeURIComponent('https://example.com/fixture-one')}`,
    )
    expect(screen.getByTestId('projection-explore-link').getAttribute('href')).toContain('/explore?q=')
  })

  it('selecting a search point shows its cached query, with no fact link', async () => {
    stubProjection({ [dumpPath]: { json: dump() } })
    render(ProjectionScatter)
    await waitFor(() => expect(screen.getByTestId('projection-plot')).toBeTruthy())

    const search = screen.getAllByTestId('projection-point').find((p) => p.getAttribute('data-kind') === 'search')
    await fireEvent.click(search!)

    await waitFor(() => expect(screen.getByTestId('projection-selected-kind').textContent).toContain('Cached search'))
    expect(screen.getByTestId('projection-selected-label').textContent).toContain('fixture term')
    expect(screen.queryByTestId('projection-fact-link')).toBeNull()
    expect(screen.getByTestId('projection-explore-link').getAttribute('href')).toBe(
      `/explore?q=${encodeURIComponent('fixture term')}&k=10`,
    )
  })

  it('selects with the keyboard as well as the mouse', async () => {
    stubProjection({ [dumpPath]: { json: dump() } })
    render(ProjectionScatter)
    await waitFor(() => expect(screen.getByTestId('projection-plot')).toBeTruthy())

    await fireEvent.keyDown(screen.getAllByTestId('projection-point')[0], { key: 'Enter' })
    await waitFor(() => expect(screen.getByTestId('projection-selection')).toBeTruthy())
  })

  it('clears the selection', async () => {
    stubProjection({ [dumpPath]: { json: dump() } })
    render(ProjectionScatter)
    await waitFor(() => expect(screen.getByTestId('projection-plot')).toBeTruthy())

    await fireEvent.click(screen.getAllByTestId('projection-point')[0])
    await waitFor(() => expect(screen.getByTestId('projection-clear')).toBeTruthy())

    await fireEvent.click(screen.getByTestId('projection-clear'))
    await waitFor(() => expect(screen.getByTestId('projection-prompt')).toBeTruthy())
  })

  it('reports an unavailable vector store as a state, not an error', async () => {
    stubProjection({
      [dumpPath]: {
        json: dump({ available: false, note: 'a re-embed migration is in progress', points: [], total: {} }),
      },
    })
    render(ProjectionScatter)

    await waitFor(() =>
      expect(screen.getByTestId('projection-unavailable').textContent).toContain('re-embed migration'),
    )
    expect(screen.queryByTestId('projection-error')).toBeNull()
  })

  it('renders an available but empty store cleanly', async () => {
    stubProjection({ [dumpPath]: { json: dump({ points: [], total: {} }) } })
    render(ProjectionScatter)

    await waitFor(() => expect(screen.getByTestId('projection-empty')).toBeTruthy())
  })

  it('surfaces a failure', async () => {
    stubProjection({ [dumpPath]: { status: 500, json: { error: 'boom' } } })
    render(ProjectionScatter)

    await waitFor(() => expect(screen.getByTestId('projection-error').textContent).toContain('boom'))
  })

  it('surfaces a missing sample cap rather than guessing one', async () => {
    stubApi({ '/api/ui-config': { status: 500, json: { error: 'no config' } } })
    render(ProjectionScatter)

    await waitFor(() => expect(screen.getByTestId('projection-config-error')).toBeTruthy())
  })
})
