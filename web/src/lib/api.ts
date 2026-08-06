/**
 * The read layer every view uses. Views import from here rather than calling
 * fetch themselves, so loading and error handling stay uniform and endpoint
 * shapes are declared in one place.
 *
 * Read-only by design: v1 of the observability UI inspects, it never triggers
 * searches, scrapes or distillation.
 */

import { writable, type Readable } from 'svelte/store'
import { getJson } from './request'

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
