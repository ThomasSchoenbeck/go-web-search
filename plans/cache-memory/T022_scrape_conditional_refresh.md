---
feature: Scrape Cache & Content Hashing
task_number: 022
description: Conditional refresh (ETag/Last-Modified/304) + cross-URL content dedupe
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 022: Conditional refresh + cross-URL content dedupe

## Description

When a cached scrape (T021) has expired but still has an `etag` or
`last_modified`, refresh it conditionally rather than refetching blindly: send
`If-None-Match` / `If-Modified-Since` on the HTTP fetch, and on a `304 Not
Modified` treat the cached content as still valid (extend its expiry) instead of
re-downloading and re-cleaning. When the server does return new content, update
the row, recompute the `content_hash`, and store the new `etag`/`last_modified`.

Also add cross-URL content dedupe using `content_hash`: if a freshly fetched
page's cleaned content hashes to the same value as an existing entry, recognize
it as duplicate content so the system does not treat identical pages served at
different URLs as distinct work downstream (e.g. distillation). Implement the
conditional-request and hashing logic in `scraper.go` (the fetch path) and
`scrapecache.go` (the store/compare side). This composes with T023, which wires
the resolver so the cache is consulted first.

## Goal

Expired-but-validatable cache entries are refreshed via ETag/Last-Modified with
304 handling that extends expiry without re-download, and identical cleaned
content across different URLs is detected via content_hash.

## How to Verify

- `scraper.go` sends `If-None-Match`/`If-Modified-Since` when the cached row has
  the corresponding validators.
- A 304 response extends the cached row's expiry and does not overwrite content;
  a 200 with changed content updates content, hash, and validators.
- Two URLs whose cleaned content is identical produce the same `content_hash` and
  are recognized as duplicates.
- Tests using `httptest.Server` cover the 304 path, the changed-content path, and
  the cross-URL dedupe.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `scraper.go`
- `scrapecache.go`
- `scrapecache_test.go`

## Dependencies

T021.
