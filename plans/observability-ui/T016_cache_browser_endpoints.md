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

## Files to Touch

- `searchcache.go`
- `scrapecache.go`
- `server.go`
- `searchcache_test.go`
- `scrapecache_test.go`

## Dependencies

T005.
