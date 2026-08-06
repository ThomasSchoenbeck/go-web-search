/**
 * Same-origin JSON reads.
 *
 * The SPA is served by the same Go listener as `/api/*`, so every request is a
 * relative path against the current origin — there is no base URL to configure
 * and nothing to change between the Vite dev server (which proxies `/api`) and
 * the embedded build.
 *
 * No Authorization header is ever set: the access model is edge auth, so the
 * app holds no token. If a deployment sets `server.api_key`, the edge (or the
 * operator) is responsible for the credential, not this code.
 */

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly path: string,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

/** Resolve an API path against the page's own origin. */
export function apiUrl(path: string): string {
  return new URL(path, window.location.origin).toString()
}

/** GET a path and decode it as JSON, throwing ApiError on a non-2xx reply. */
export async function getJson<T>(path: string, signal?: AbortSignal): Promise<T> {
  const response = await fetch(apiUrl(path), {
    method: 'GET',
    headers: { Accept: 'application/json' },
    signal,
  })

  if (!response.ok) {
    // Error replies use {"error": "..."} (writeErr in server.go); fall back to
    // the status text when a proxy or the SPA fallback returns something else.
    let detail = response.statusText
    try {
      const body = (await response.json()) as { error?: string }
      if (body?.error) detail = body.error
    } catch {
      // Not JSON — keep the status text.
    }
    throw new ApiError(`GET ${path} failed: ${detail}`, response.status, path)
  }

  return (await response.json()) as T
}
