/**
 * Test-only fetch stub for component tests. Not imported by application code.
 *
 * Routes are keyed by `pathname` or `pathname + search`, so a test can give
 * `/api/scrapes/x` and `/api/scrapes/x?raw=1` different bodies — which is how
 * the scrape view's "fetch raw only on demand" rule is verified.
 */

import { vi } from 'vitest'

export interface StubRoute {
  /** Defaults to 200. */
  status?: number
  /** JSON body. Go marshals empty slices as null, so `null` is meaningful. */
  json?: unknown
  /** Text body, for endpoints that do not serve JSON. */
  text?: string
}

export type FetchMock = ReturnType<typeof vi.fn>

export function stubApi(routes: Record<string, StubRoute>): FetchMock {
  const fetchMock = vi.fn(async (input: string) => {
    const url = new URL(input)
    const route = routes[url.pathname + url.search] ?? routes[url.pathname]

    if (!route) {
      return response({ status: 404, json: { error: `no stub for ${url.pathname}${url.search}` } })
    }
    return response(route)
  })

  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

function response(route: StubRoute): Response {
  const status = route.status ?? 200
  // `'json' in route` rather than `route.json ?? {}`: a null body is meaningful
  // here (Go marshals empty slices as null) and must survive the stub intact.
  const body = 'json' in route ? route.json : {}
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText: statusText(status),
    json: async () => body,
    text: async () => route.text ?? JSON.stringify(body),
  } as Response
}

function statusText(status: number): string {
  if (status === 404) return 'Not Found'
  if (status === 500) return 'Internal Server Error'
  return status === 200 ? 'OK' : String(status)
}

/** Every path a fetch mock was called with, ignoring the origin. */
export function requestedPaths(fetchMock: FetchMock): string[] {
  return fetchMock.mock.calls.map((call) => {
    const url = new URL(call[0] as string)
    return url.pathname + url.search
  })
}
