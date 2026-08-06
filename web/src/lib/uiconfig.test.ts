import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { loadUIConfig, resetUIConfig, toUIConfig } from './uiconfig'

beforeEach(() => resetUIConfig())
afterEach(() => vi.unstubAllGlobals())

const wire = { poll_interval_ms: 2500, poll_enabled: true, projection_sample_cap: 1234 }

describe('toUIConfig', () => {
  it('maps the wire shape to camelCase', () => {
    expect(toUIConfig(wire)).toEqual({
      pollIntervalMs: 2500,
      pollEnabled: true,
      projectionSampleCap: 1234,
    })
  })
})

describe('loadUIConfig', () => {
  it('seeds the session defaults from the endpoint rather than constants', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => wire }))

    const config = await loadUIConfig()
    expect(config.pollIntervalMs).toBe(2500)
    expect(config.pollEnabled).toBe(true)
    expect(config.projectionSampleCap).toBe(1234)
  })

  it('propagates polling disabled by config', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => ({ ...wire, poll_enabled: false }) }),
    )
    await expect(loadUIConfig()).resolves.toMatchObject({ pollEnabled: false })
  })

  it('fetches once per session and shares the result', async () => {
    const fetchMock = vi.fn().mockResolvedValue({ ok: true, status: 200, json: async () => wire })
    vi.stubGlobal('fetch', fetchMock)

    const [a, b] = await Promise.all([loadUIConfig(), loadUIConfig()])
    await loadUIConfig()

    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(a).toBe(b)
  })

  it('does not cache a failure, so a later caller retries', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce({ ok: false, status: 500, statusText: 'boom', json: async () => ({}) })
      .mockResolvedValueOnce({ ok: true, status: 200, json: async () => wire })
    vi.stubGlobal('fetch', fetchMock)

    await expect(loadUIConfig()).rejects.toThrow()
    await expect(loadUIConfig()).resolves.toMatchObject({ pollIntervalMs: 2500 })
    expect(fetchMock).toHaveBeenCalledTimes(2)
  })
})
