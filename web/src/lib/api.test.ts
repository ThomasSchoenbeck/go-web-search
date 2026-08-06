import { describe, it, expect, vi, afterEach } from 'vitest'
import { get } from 'svelte/store'
import { createResource, statsResource, type Stats } from './api'

afterEach(() => vi.unstubAllGlobals())

const stats: Stats = {
  runs: 1,
  searches: 1,
  urls: 2,
  scrapes: 1,
  memory_facts: 1,
  search_cache: 1,
  vectors: 0,
  pending_jobs: 1,
  scrape_text_avg_chars: 11,
  scrape_text_max_chars: 11,
  scrape_raw_avg_chars: 46,
  scrape_raw_max_chars: 46,
}

describe('createResource', () => {
  it('starts idle with no data', () => {
    const resource = createResource(async () => 'value')
    expect(get(resource)).toEqual({ loading: false, error: null, data: null })
  })

  it('exposes a loading state before the data arrives', async () => {
    let release: (value: string) => void = () => {}
    const resource = createResource(() => new Promise<string>((r) => (release = r)))

    const pending = resource.reload()
    expect(get(resource).loading).toBe(true)
    expect(get(resource).data).toBeNull()

    release('value')
    await pending
    expect(get(resource)).toEqual({ loading: false, error: null, data: 'value' })
  })

  it('captures a failure as an Error', async () => {
    const resource = createResource(async () => {
      throw new Error('endpoint down')
    })

    await resource.reload()
    const state = get(resource)
    expect(state.loading).toBe(false)
    expect(state.error?.message).toBe('endpoint down')
  })

  it('keeps the last good data when a reload fails', async () => {
    let attempt = 0
    const resource = createResource(async () => {
      attempt += 1
      if (attempt === 1) return 'first'
      throw new Error('gone')
    })

    await resource.reload()
    await resource.reload()

    const state = get(resource)
    expect(state.data).toBe('first')
    expect(state.error?.message).toBe('gone')
  })

  it('clears a previous error on a successful reload', async () => {
    let attempt = 0
    const resource = createResource(async () => {
      attempt += 1
      if (attempt === 1) throw new Error('transient')
      return 'ok'
    })

    await resource.reload()
    expect(get(resource).error).not.toBeNull()

    await resource.reload()
    expect(get(resource).error).toBeNull()
    expect(get(resource).data).toBe('ok')
  })
})

describe('statsResource', () => {
  it('reads /api/stats through the shared client', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => stats })
    vi.stubGlobal('fetch', fetchMock)

    const resource = statsResource()
    await resource.reload()

    expect(fetchMock.mock.calls[0][0]).toBe(`${window.location.origin}/api/stats`)
    expect(get(resource).data).toEqual(stats)
  })
})
