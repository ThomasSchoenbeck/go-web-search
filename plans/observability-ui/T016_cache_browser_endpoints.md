---
feature: Jobs, Caches, Logs & Stats
task_number: 016
description: NEW endpoints — search_cache + scrape_cache browsers (tier/expiry/hit_count, filters, pagination)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 016: NEW search_cache + scrape_cache browser endpoints

## Description

Add two new read-only endpoints so the UI can browse the caches. Neither table
has a list endpoint today. Add store read queries plus handlers and routes.

- **search_cache browser:** list rows from `search_cache` (schema.sql): query,
  query_norm, tier, hit_count, expires_at, fetched_at, timestamps, and a
  size/summary of `results` (results is JSON — return a count or truncated summary
  rather than the full blob by default). Filter by tier and optional text match on
  the query; paginate with `limit`/`offset`.
- **scrape_cache browser:** list rows from `scrape_cache`: url, http_status,
  content_type, fetched_with, title, tier, hit_count, expires_at, fetched_at,
  content_hash, robots_allowed, error, and sizes (lengths of raw_html/clean_html/
  text_content) — NOT the full content bodies (those stay behind
  `/api/scrapes/{id}`). Filter by tier and optional URL/domain match; paginate.

Both should surface the tier/expiry/hit_count fields the sliding-expiry system
uses, so a user can see promotion and staleness. Add the read queries alongside
searchcache.go / scrapecache.go (or store.go), register the routes in
`newAPIServer`, and use `writeJSON`/`writeErr`. Read-only — no eviction or edits.

## Goal

Two new GET endpoints list `search_cache` and `scrape_cache` rows with their
tier/expiry/hit_count metadata (and content sizes, not full bodies), with tier and
text/URL filters and pagination, backed by new store read queries.

## How to Verify

- `go fmt`, `go vet`, `go test` pass; store tests (SQLite) cover both listings
- Backend tests run against an isolated temporary database and remove all test data on completion — no shared or production database is touched.
  with tier filter, text/URL filter, and pagination.
- The search_cache endpoint returns query/tier/hit_count/expiry with a results
  summary (not the full blob); the scrape_cache endpoint returns url/tier/
  hit_count/expiry and content sizes (not full bodies).
- Empty caches return empty lists; filters and paging work.
- Both routes are registered in `newAPIServer` and behind the shared `/api` middleware (`withAuth`, a no-op when `api_key` is unset — edge auth).

## Note added during implementation

- The routes are `GET /api/cache/searches` and `GET /api/cache/scrapes`, both
  replying `{count, entries}`.
- The search-cache summary reports `result_count` (URLs in the stored blob) and
  `results_chars` (its size) instead of the blob. Its text filter runs against
  `query_norm` through `normalizeQuery`, so it matches the way the cache key
  does — case- and whitespace-insensitive.
- The scrape-cache summary reports `text_chars`/`clean_html_chars`/
  `raw_html_chars` computed in SQL, so no body is ever read into memory.
- Both use the shared `clampPage` bound added in `store.go` (T014).
- `seedTestData` gained a long-tier cached query and a long-tier cached page
  attached to no run, so the tier filters have something to separate. The new
  page is not attached to the seeded run, so the run views' counts are unmoved.

## Files to Touch

- `searchcache.go`
- `scrapecache.go`
- `server.go`
- `testsupport.go` — long-tier fixtures for both caches
- `searchcache_test.go` [NEW]
- `scrapecache_test.go` [NEW]

## Dependencies

T005.
