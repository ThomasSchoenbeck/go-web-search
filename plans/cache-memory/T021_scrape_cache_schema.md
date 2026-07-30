---
feature: Scrape Cache & Content Hashing
task_number: 021
description: Scrape cache schema (content_hash, etag, last_modified, tier, expiry) + cache-check-before-insert
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 021: Scrape cache schema + cache-check-before-insert

## Description

Add a `scrape_cache` store keyed exactly on URL (no vectoring — scraped content
is not embedded). A row holds the fetched content and metadata needed for
conditional refresh and durability: `content_hash` (of the cleaned content),
`etag`, `last_modified`, tier and expiry fields for sliding expiry (T029), a
hit_count for promotion, and the audit columns. The URL key is unique so a lookup
is a single exact match.

Today `scraper.one` (scraper.go) always calls `SaveScrape`, inserting a new
`scrapes` row on every fetch with no cache check (documented in README "Known
limits"). This task introduces the cache-check-before-insert primitive: a
context-first lookup that returns a fresh cached row for a URL if present and not
expired, so `scraper.one` (rewired in T023) can return it instead of refetching.
Add the table to `schema.sql` (following its conventions) and the store helpers
in a new `scrapecache.go`. `scrape_cache` REPLACES the existing `scrapes` table:
this is a green-field deployment, so the old `scrapes` table (and its dependent
`scrape_images` FK) is dropped and no longer used. The existing by-id read paths
(`GetScrape` in store.go, `get_scrape` MCP tool, `/api/scrapes/{id}`) are
repointed to `scrape_cache` as part of this feature (schema + helper here; endpoint
rewiring in T023). No dual-write, no compatibility shim.

## Goal

A `scrape_cache` table keyed uniquely on URL with content_hash/etag/last_modified/
tier/expiry/hit_count and audit columns exists, plus a store helper that returns a
fresh cached entry for a URL or reports a miss.

## How to Verify

- `schema.sql` defines `scrape_cache` with the fields above, a unique URL index,
  `IF NOT EXISTS`, and no embedding/vector reference.
- `scrapecache.go` exposes context-first store and lookup helpers; lookup returns
  a hit only when the row is present and unexpired.
- The old `scrapes` and `scrape_images` tables are removed from `schema.sql`;
  `scrape_cache` is the sole scrape store and the by-id read paths (repointed in
  T023) still function against it.
- `scrapecache_test.go` against SQLite covers store, exact-URL hit, expiry miss,
  and content_hash storage.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `schema.sql`
- `scrapecache.go` [NEW]
- `scrapecache_test.go` [NEW]

## Dependencies

T007.
