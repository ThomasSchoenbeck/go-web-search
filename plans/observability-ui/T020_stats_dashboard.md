---
feature: Jobs, Caches, Logs & Stats
task_number: 020
description: Extend /api/stats (model+dim+migration, cache hit rates+tiers, job throughput); Stats dashboard view
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 020: Extend /api/stats + Stats dashboard view

## Description

Two small, coupled pieces: a minor read-only backend extension and the dashboard
view that consumes it.

Confirmed scope: the dashboard is the full picture, so `/api/stats` grows in three
read-only directions on top of the existing counts.

**Backend:** `StatsView`/`Store.Stats` (stats.go) already counts runs, searches,
urls, scrapes, memory_facts, search_cache, vectors, and pending_jobs, plus scrape
size aggregates. Add:

1. **Active embedding model + dim and re-embed migration state**, read from
   `system_meta` via `MetaGet` (model name + dim from `LLMClient`
   `EmbedModelName`/`EmbedDim` or meta; `metaMigration` empty means not migrating).
   Minor, best-effort — a missing key or in-flight migration leaves the field
   blank/false rather than erroring.
2. **Cache hit rates + tier distributions** for the search and scrape caches:
   counts of rows by tier (`short`/`long`/`permanent`) and an aggregate of
   `hit_count`. **RISK — true hit *rate* needs total lookups (hits + misses),
   which the schema does not track today (only per-row `hit_count`).** Resolve this
   in the task before building: either (a) present hit *counts* and tier
   distributions only (fully read-only, no schema change), or (b) if a real rate is
   wanted, add lightweight lookup/hit counters — but that is a write to app state
   and must be weighed against the read-only v1 rule. Default to (a) unless the
   rate is explicitly required.
3. **Job throughput/timings** derived read-only from the `jobs` table: counts by
   status and type, failed/retried counts (`attempts`), and simple timing derived
   from `created_at`/`updated_at`/`locked_at`.

All read-only additions to the existing `/api/stats` handler — no new route.

**Frontend:** a Stats dashboard view rendering all the counts and size aggregates
as readable cards/tiles, plus the active-model/dim and "migration in progress"
indicator, the cache tier/hit breakdowns, and the job throughput/timing summary.
Uses the shared API layer (T005).

## Goal

`/api/stats` returns the existing counts plus active embedding model+dim and
migration state, cache hit/tier breakdowns, and job throughput/timings, and a
Stats dashboard view renders them all readably.

## How to Verify

- `go fmt`, `go vet`, `go test` pass; a stats test confirms the new model/dim +
  migration fields, the cache tier/hit breakdowns, and the job throughput fields
  populate (and stay blank/zero gracefully when meta is unset or a migration is
  flagged). Backend tests run against an isolated temporary database and remove all
  test data on completion.
- The hit-rate-vs-count decision (see RISK) is resolved and reflected: the response
  exposes tier distributions + hit counts, and a true rate only if lookup counters
  were added.
- `/api/stats` includes all new fields alongside the existing counts.
- The dashboard view renders every count, the size aggregates, the active
  model+dim, a clear "migration in progress" state when flagged, the cache
  tier/hit breakdowns, and the job throughput/timing summary.
- The frontend builds and lints clean
  (`pnpm build`).
- Unit tests (Vitest) cover the dashboard's major functions; `pnpm test` passes.
- Playwright tests exercise the stats page and every interactive element; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down.

## Files to Touch

- `stats.go`
- `server.go`
- `stats_test.go`
- `web/src/views/StatsDashboard.svelte` [NEW]
- `web/src/App.svelte`

## Dependencies

T005.
