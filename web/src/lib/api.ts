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

/**
 * GET /api/stats (CacheStats in stats.go).
 *
 * Hit *counts*, not a rate: the schema records hit_count on the rows that
 * exist and nothing about lookups that missed, so a rate is not derivable
 * without writing new counters — which the read-only v1 does not do.
 */
export interface CacheStats {
  rows: number
  total_hits: number
  rows_with_hits: number
  expired: number
  tiers: Record<string, number>
}

/** GET /api/stats (JobStats in stats.go). */
export interface JobStats {
  by_status: Record<string, number>
  by_type: Record<string, number>
  retried: number
  max_attempts: number
  oldest_pending_at?: string
  completed_sampled: number
  avg_completion_ms: number
}

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
  embed_model?: string
  embed_dim?: number
  vector_table?: string
  /** True while a re-embed runs, when the vector count is the old generation's. */
  vector_migration_in_progress: boolean
  search_cache_stats: CacheStats
  scrape_cache_stats: CacheStats
  jobs: JobStats
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

/** GET /api/provenance?url= (URLProvenance in provenance.go). */
export interface FoundBySearch {
  search_id: string
  run_id: string
  term: string
  engine: string
  search_mode: string
  rank: number
  created_at: string
}

export interface FactProvenance {
  id: string
  text: string
  source_url?: string
  volatility?: string
  tier?: string
  has_vector: boolean
  created_at: string
}

export interface ScrapeSizes {
  scrape_id: string
  url: string
  title?: string
  http_status?: number
  fetched_with?: string
  text_chars: number
  clean_html_chars: number
  raw_html_chars: number
  created_at: string
}

export interface UrlProvenance {
  url: string
  url_id?: string
  known: boolean
  found_by: FoundBySearch[]
  scrape?: ScrapeSizes
  facts: FactProvenance[]
  /** False while a re-embed migration runs or no vector table is active. */
  vectors_available: boolean
  note?: string
}

/** GET /api/runs/{id}/causality (CausalityGraph in provenance.go). */
export type CausalityKind = 'search' | 'url' | 'scrape' | 'fact'

export interface CausalityNode {
  id: string
  kind: CausalityKind
  ref_id: string
  label: string
  detail?: string
  url?: string
  has_vector?: boolean
}

export interface CausalityEdge {
  from: string
  to: string
  rank?: number
}

export interface CausalityGraph {
  run_id: string
  nodes: CausalityNode[]
  edges: CausalityEdge[]
  truncated: boolean
  limit: number
  vectors_available: boolean
  note?: string
}

export function provenanceResource(url: string): Resource<UrlProvenance> {
  return createResource(() => getJson<UrlProvenance>(`/api/provenance?url=${encodeURIComponent(url)}`))
}

/** GET /api/memory/facts (FactSummary in memory.go). */
export interface FactSummary {
  id: string
  text: string
  text_chars: number
  source_url?: string
  volatility?: string
  tier?: string
  hit_count: number
  created_at?: string
  expires_at?: string
}

/** GET /api/memory/facts/{id} (FactDetail in server.go). */
export interface FactDetail {
  fact: FactSummary
  source?: ScrapeSizes
  /** API path to the raw source page; absent when it is no longer cached. */
  read_raw?: string
  note?: string
}

/** GET /api/explore (ExploreResult in explorer.go). */
export interface Neighbor {
  owner_kind: 'memory' | 'search'
  id: string
  distance: number
  similarity: number
  text: string
  source_url?: string
  tier?: string
  result_count?: number
}

export interface ExploreResult {
  query: string
  k: number
  /** False when no vector table is active or a re-embed is in flight. */
  available: boolean
  note?: string
  neighbors: Neighbor[]
  memory_hits: number
  search_hits: number
}

export interface FactsQuery {
  q?: string
  limit: number
  offset: number
}

export function factsResource(query: FactsQuery): Resource<FactSummary[]> {
  const params = new URLSearchParams()
  if (query.q) params.set('q', query.q)
  params.set('limit', String(query.limit))
  params.set('offset', String(query.offset))
  return createResource(() =>
    getJson<{ count: number; facts: FactSummary[] | null }>(`/api/memory/facts?${params}`).then(
      (body) => body.facts ?? [],
    ),
  )
}

export function factResource(id: string): Resource<FactDetail> {
  return createResource(() => getJson<FactDetail>(`/api/memory/facts/${encodeURIComponent(id)}`))
}

export function exploreResource(query: string, k: number): Resource<ExploreResult> {
  const params = new URLSearchParams({ q: query, k: String(k) })
  return createResource(() =>
    getJson<ExploreResult>(`/api/explore?${params}`).then((result) => ({
      ...result,
      neighbors: result.neighbors ?? [],
    })),
  )
}

/** GET /api/jobs (JobSummary in jobstore.go). */
export interface JobSummary {
  id: string
  type: string
  /** Arbitrary JSON written by whichever enqueuer produced the job. Display as text. */
  payload?: string
  status: string
  attempts: number
  run_after?: string
  locked_at?: string
  created_at: string
  updated_at: string
}

export interface JobsPage {
  jobs: JobSummary[]
  /** Whole-queue counts by status, so filtering the list does not distort them. */
  counts: Record<string, number>
}

export interface JobsQuery {
  status?: string
  type?: string
  limit: number
  offset: number
}

export function jobsResource(query: JobsQuery): Resource<JobsPage> {
  const params = new URLSearchParams()
  if (query.status) params.set('status', query.status)
  if (query.type) params.set('type', query.type)
  params.set('limit', String(query.limit))
  params.set('offset', String(query.offset))
  return createResource(() =>
    getJson<JobsPage>(`/api/jobs?${params}`).then((page) => ({
      jobs: page.jobs ?? [],
      counts: page.counts ?? {},
    })),
  )
}

/** GET /api/cache/searches (SearchCacheSummary in searchcache.go). */
export interface SearchCacheEntry {
  id: string
  query: string
  query_norm: string
  tier: string
  hit_count: number
  result_count: number
  results_chars: number
  expires_at?: string
  fetched_at: string
  created_at: string
  updated_at: string
}

/** GET /api/cache/scrapes (ScrapeCacheSummary in scrapecache.go). */
export interface ScrapeCacheEntry {
  id: string
  url: string
  http_status?: number
  content_type?: string
  fetched_with?: string
  title?: string
  robots_allowed: boolean
  error?: string
  content_hash?: string
  tier: string
  hit_count: number
  text_chars: number
  clean_html_chars: number
  raw_html_chars: number
  expires_at?: string
  fetched_at: string
  created_at: string
  updated_at: string
}

/** `q` matches the cached query text for searches and the URL for scrapes. */
export interface CacheQuery {
  tier?: string
  q?: string
  limit: number
  offset: number
}

function cacheParams(query: CacheQuery): URLSearchParams {
  const params = new URLSearchParams()
  if (query.tier) params.set('tier', query.tier)
  if (query.q) params.set('q', query.q)
  params.set('limit', String(query.limit))
  params.set('offset', String(query.offset))
  return params
}

function entriesOf<T>(path: string): Promise<T[]> {
  return getJson<{ count: number; entries: T[] | null }>(path).then((body) => body.entries ?? [])
}

export function searchCacheResource(query: CacheQuery): Resource<SearchCacheEntry[]> {
  return createResource(() => entriesOf<SearchCacheEntry>(`/api/cache/searches?${cacheParams(query)}`))
}

export function scrapeCacheResource(query: CacheQuery): Resource<ScrapeCacheEntry[]> {
  return createResource(() => entriesOf<ScrapeCacheEntry>(`/api/cache/scrapes?${cacheParams(query)}`))
}

/** GET /api/logs (LogEntry in logstore.go). Reads the separate log database. */
export interface LogEntry {
  id: string
  run_id?: string
  level: string
  source?: string
  message: string
  created_at: string
}

export interface LogsQuery {
  run_id?: string
  level?: string
  source?: string
  limit: number
  offset: number
}

export function logsResource(query: LogsQuery): Resource<LogEntry[]> {
  const params = new URLSearchParams()
  if (query.run_id) params.set('run_id', query.run_id)
  if (query.level) params.set('level', query.level)
  if (query.source) params.set('source', query.source)
  params.set('limit', String(query.limit))
  params.set('offset', String(query.offset))
  return createResource(() => entriesOf<LogEntry>(`/api/logs?${params}`))
}

export function runCausalityResource(runId: string): Resource<CausalityGraph> {
  return createResource(() =>
    getJson<CausalityGraph>(`/api/runs/${encodeURIComponent(runId)}/causality`).then((graph) => ({
      ...graph,
      nodes: graph.nodes ?? [],
      edges: graph.edges ?? [],
    })),
  )
}
