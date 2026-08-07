/**
 * Session defaults from `GET /api/ui-config`, which serves the non-secret part
 * of config.toml's `[observability]` block.
 *
 * These are the *only* source of the polling defaults and the projection cap —
 * nothing here is hardcoded, so changing config.toml changes the app without a
 * rebuild. The UI may override the polling values for the current session
 * (T015/T019); it never writes back.
 */

import { getJson } from './request'

/** Wire shape of GET /api/ui-config (UIConfig in server.go). */
interface UIConfigWire {
  poll_interval_ms: number
  poll_enabled: boolean
  projection_sample_cap: number
}

export interface UIConfig {
  pollIntervalMs: number
  pollEnabled: boolean
  projectionSampleCap: number
}

export function toUIConfig(wire: UIConfigWire): UIConfig {
  return {
    pollIntervalMs: wire.poll_interval_ms,
    pollEnabled: wire.poll_enabled,
    projectionSampleCap: wire.projection_sample_cap,
  }
}

let pending: Promise<UIConfig> | null = null

/**
 * Fetch the settings once per session. Concurrent callers share one request;
 * a failed load is not cached, so a later caller retries.
 */
export function loadUIConfig(): Promise<UIConfig> {
  if (!pending) {
    pending = getJson<UIConfigWire>('/api/ui-config')
      .then(toUIConfig)
      .catch((err: unknown) => {
        pending = null
        throw err
      })
  }
  return pending
}

/** Drop the memoized settings. Exists for tests. */
export function resetUIConfig(): void {
  pending = null
}
