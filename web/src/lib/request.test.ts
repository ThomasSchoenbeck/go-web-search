import { describe, it, expect, vi, afterEach } from 'vitest'
import { apiUrl, getJson, ApiError } from './request'

afterEach(() => vi.unstubAllGlobals())

function stubFetch(response: Partial<Response>): ReturnType<typeof vi.fn> {
  const fetchMock = vi.fn().mockResolvedValue(response as Response)
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

describe('apiUrl', () => {
  it('resolves against the page origin, with no hardcoded host or port', () => {
    expect(apiUrl('/api/stats')).toBe(`${window.location.origin}/api/stats`)
  })
})

describe('getJson', () => {
  it('returns the decoded body', async () => {
    stubFetch({ ok: true, status: 200, json: async () => ({ runs: 3 }) })
    await expect(getJson<{ runs: number }>('/api/stats')).resolves.toEqual({ runs: 3 })
  })

  it('sends no Authorization header', async () => {
    const fetchMock = stubFetch({ ok: true, status: 200, json: async () => ({}) })
    await getJson('/api/stats')

    const init = fetchMock.mock.calls[0][1] as RequestInit
    const headers = init.headers as Record<string, string>
    expect(Object.keys(headers).map((k) => k.toLowerCase())).not.toContain('authorization')
    expect(init.method).toBe('GET')
  })

  it('raises ApiError carrying the status and the server message', async () => {
    stubFetch({
      ok: false,
      status: 404,
      statusText: 'Not Found',
      json: async () => ({ error: 'no such run' }),
    })

    const error = await getJson('/api/runs/nope').catch((e: unknown) => e)
    expect(error).toBeInstanceOf(ApiError)
    expect((error as ApiError).status).toBe(404)
    expect((error as ApiError).message).toContain('no such run')
  })

  it('falls back to the status text when the error body is not JSON', async () => {
    stubFetch({
      ok: false,
      status: 503,
      statusText: 'Service Unavailable',
      json: async () => {
        throw new Error('not json')
      },
    })

    const error = (await getJson('/api/stats').catch((e: unknown) => e)) as ApiError
    expect(error.status).toBe(503)
    expect(error.message).toContain('Service Unavailable')
  })
})
