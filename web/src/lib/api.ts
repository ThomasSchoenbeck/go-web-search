/**
 * The read layer every view uses. Views import from here rather than calling
 * fetch themselves, so loading and error handling stay uniform and endpoint
 * shapes are declared in one place.
 *
 * Read-only by design: v1 of the observability UI inspects, it never triggers
 * searches, scrapes or distillation.
 */

import { writable, type Readable } from 'svelte/store'
import { getJson, getText } from './request'

export { ApiError } from './request'

/** GET /api/stats (StatsView in stats.go). */
export interface Stats {
  runs: number
  searches: number
  urls: number
  scrapes: number
  memory_facts: number
  search_cache: number
  vectors: number
  pending_jobs: number
  scrape_text_avg_chars: number
  scrape_text_max_chars: number
  scrape_raw_avg_chars: number
  scrape_raw_max_chars: number
}

export interface ResourceState<T> {
  loading: boolean
  error: Error | null
  data: T | null
}

export interface Resource<T> extends Readable<ResourceState<T>> {
  /** Fetch again, keeping the previous data visible while in flight. */
  reload(): Promise<void>
}

/**
 * Wraps a loader in the loading/error/data states views render from.
 *
 * Data is kept across a reload so a polling view does not blink back to a
 * spinner every interval, and a failed reload leaves the last good data on
 * screen next to the error rather than blanking the page.
 */
export function createResource<T>(load: () => Promise<T>): Resource<T> {
  const { subscribe, update } = writable<ResourceState<T>>({
    loading: false,
    error: null,
    data: null,
  })

  async function reload(): Promise<void> {
    update((state) => ({ ...state, loading: true, error: null }))
    try {
      const data = await load()
      update((state) => ({ ...state, loading: false, error: null, data }))
    } catch (error) {
      update((state) => ({
        ...state,
        loading: false,
        error: error instanceof Error ? error : new Error(String(error)),
      }))
    }
  }

  return { subscribe, reload }
}

export function statsResource(): Resource<Stats> {
  return createResource(() => getJson<Stats>('/api/stats'))
}

/**
 * Go marshals an empty slice as `null`, not `[]`, so every list endpoint can
 * return null for "nothing found". Normalising here means no view has to guard
 * for it.
 */
function getList<T>(path: string): Promise<T[]> {
  return getJson<T[] | null>(path).then((value) => value ?? [])
}

/** GET /api/runs and /api/runs/{id} (RunSummary in store.go). */
export interface RunSummary {
  id: string
  mode: string
  artifact_dir?: string
  started_at: string
  finished_at?: string
  searches: number
  urls: number
  scrapes: number
}

/** GET /api/runs/{id}/urls (URLRow in store.go). */
export interface UrlRow {
  id: string
  url: string
  domain: string
  rank?: number
  engine?: string
}

/** GET /api/runs/{id}/searches (SearchSummary in store.go). */
export interface SearchSummary {
  id: string
  run_id: string
  term: string
  engine: string
  search_mode: string
  landed_url?: string
  http_status?: number
  blocked: boolean
  anchor_count: number
  error?: string
  duration_ms: number
  created_at: string
}

/** GET /api/scrapes/{id} (ScrapeDetail in store.go). */
export interface ScrapeImage {
  id: string
  url: string
  alt?: string
  width?: number
  height?: number
}

export interface ScrapeDetail {
  id: string
  url: string
  run_id?: string
  http_status?: number
  content_type?: string
  fetched_with?: string
  robots_allowed: boolean
  title?: string
  clean_html?: string
  text?: string
  raw_html?: string
  error?: string
  duration_ms: number
  created_at: string
  images?: ScrapeImage[]
  content_hash?: string
  etag?: string
  last_modified?: string
  tier?: string
  hit_count: number
  expires_at?: string
  fetched_at?: string
}

export function runsResource(limit?: number): Resource<RunSummary[]> {
  const query = limit && limit > 0 ? `?limit=${limit}` : ''
  return createResource(() => getList<RunSummary>(`/api/runs${query}`))
}

export function runResource(id: string): Resource<RunSummary> {
  return createResource(() => getJson<RunSummary>(`/api/runs/${encodeURIComponent(id)}`))
}

export function runUrlsResource(id: string): Resource<UrlRow[]> {
  return createResource(() => getList<UrlRow>(`/api/runs/${encodeURIComponent(id)}/urls`))
}

export function runSearchesResource(id: string): Resource<SearchSummary[]> {
  return createResource(() => getList<SearchSummary>(`/api/runs/${encodeURIComponent(id)}/searches`))
}

export function runScrapeIdsResource(id: string): Resource<string[]> {
  return createResource(() =>
    getJson<{ scrape_ids: string[] | null }>(`/api/runs/${encodeURIComponent(id)}/scrapes`).then(
      (body) => body.scrape_ids ?? [],
    ),
  )
}

/** Raw SERP HTML. Untrusted — render sandboxed or escaped, never injected. */
export function searchRawResource(id: string): Resource<string> {
  return createResource(() => getText(`/api/searches/${encodeURIComponent(id)}/raw`))
}

/** `includeRaw` pulls raw_html, which can be large — request it only on demand. */
export function scrapeResource(id: string, includeRaw = false): Resource<ScrapeDetail> {
  const query = includeRaw ? '?raw=1' : ''
  return createResource(() => getJson<ScrapeDetail>(`/api/scrapes/${encodeURIComponent(id)}${query}`))
}
