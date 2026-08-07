import { describe, it, expect, vi, afterEach } from 'vitest'
import { render, screen, cleanup, waitFor, fireEvent } from '@testing-library/svelte'
import SerpViewer from './SerpViewer.svelte'
import { stubApi } from '../lib/apiStub'

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
})

const serpHtml = '<html><body><h1>fixture SERP</h1><script>alert(1)</script></body></html>'

describe('SerpViewer', () => {
  it('renders the SERP in a fully sandboxed frame', async () => {
    stubApi({ '/api/searches/search-1/raw': { text: serpHtml } })
    render(SerpViewer, { props: { id: 'search-1' } })

    const frame = await waitFor(() => screen.getByTestId('serp-frame'))
    // sandbox="" is the whole point: an opaque origin with scripts, forms and
    // navigation disabled, so untrusted markup cannot reach the app or the API.
    expect(frame.getAttribute('sandbox')).toBe('')
    expect(frame.getAttribute('srcdoc')).toBe(serpHtml)
    expect(frame.getAttribute('referrerpolicy')).toBe('no-referrer')
  })

  it('never injects the SERP into the app DOM', async () => {
    stubApi({ '/api/searches/search-1/raw': { text: serpHtml } })
    const { container } = render(SerpViewer, { props: { id: 'search-1' } })

    await waitFor(() => expect(screen.getByTestId('serp-frame')).toBeTruthy())
    expect(container.querySelector('script')).toBeNull()
    expect(container.querySelector('h1')?.textContent).toBe('Raw SERP')
  })

  it('toggles to escaped source and back', async () => {
    stubApi({ '/api/searches/search-1/raw': { text: serpHtml } })
    render(SerpViewer, { props: { id: 'search-1' } })

    await waitFor(() => expect(screen.getByTestId('serp-frame')).toBeTruthy())

    await fireEvent.click(screen.getByTestId('serp-view-source'))
    const source = screen.getByTestId('serp-source')
    expect(source.textContent).toBe(serpHtml)
    expect(screen.queryByTestId('serp-frame')).toBeNull()

    await fireEvent.click(screen.getByTestId('serp-view-rendered'))
    await waitFor(() => expect(screen.getByTestId('serp-frame')).toBeTruthy())
  })

  it('reports the stored size', async () => {
    stubApi({ '/api/searches/search-1/raw': { text: serpHtml } })
    render(SerpViewer, { props: { id: 'search-1' } })

    await waitFor(() => expect(screen.getByTestId('serp-size').textContent).toContain('characters'))
  })

  it('treats a 404 as "no raw HTML", not an error', async () => {
    stubApi({ '/api/searches/search-2/raw': { status: 404 } })
    render(SerpViewer, { props: { id: 'search-2' } })

    await waitFor(() => expect(screen.getByTestId('serp-missing')).toBeTruthy())
    expect(screen.queryByTestId('serp-error')).toBeNull()
  })

  it('still reports a real failure', async () => {
    stubApi({ '/api/searches/search-3/raw': { status: 500 } })
    render(SerpViewer, { props: { id: 'search-3' } })

    await waitFor(() => expect(screen.getByTestId('serp-error')).toBeTruthy())
    expect(screen.queryByTestId('serp-missing')).toBeNull()
  })
})
