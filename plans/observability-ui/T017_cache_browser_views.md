---
feature: Jobs, Caches, Logs & Stats
task_number: 017
description: Cache browser views (search_cache + scrape_cache)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 017: Cache browser views

## Description

Build the two cache browser views over the T016 endpoints.

- **Search cache view:** a filterable, paginated table of cached searches — query,
  tier, hit_count, expires_at, fetched_at, and the results summary. Filter by tier
  and query text.
- **Scrape cache view:** a filterable, paginated table of cached scrapes — url,
  http_status, content_type, title, tier, hit_count, expires_at, fetched_at,
  content sizes, robots_allowed, error. Filter by tier and URL/domain. Each row
  links to the scrape detail (T008) and its provenance (T010).

Both surface the tier/expiry/hit_count fields so promotion and staleness are
visible. Uses the shared API layer (T005).

## Goal

Search-cache and scrape-cache browser views render the T016 listings with tier
and text/URL filters and pagination, and the scrape-cache rows link to scrape
detail and provenance.

## How to Verify

- The search-cache view lists cached queries with tier/hit_count/expiry and a
  results summary; tier + text filters and paging work.
- The scrape-cache view lists cached URLs with tier/hit_count/expiry and content
  sizes; tier + URL filters and paging work; rows link to scrape detail (T008)
  and provenance (T010).
- Empty caches render cleanly; loading/error from
  the shared layer.
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/SearchCacheBrowser.svelte` [NEW]
- `web/src/views/ScrapeCacheBrowser.svelte` [NEW]
- `web/src/App.svelte`

## Dependencies

T016.
