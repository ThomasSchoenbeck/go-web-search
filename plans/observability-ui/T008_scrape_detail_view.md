---
feature: Core Views — Scrapes
task_number: 008
description: Scrape detail view — raw/clean/text toggle + images + fetch metadata (reuses /api/scrapes/{id}?raw=1)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 008: Scrape detail view

## Description

Build the scrape detail view over the existing `GET /api/scrapes/{id}` endpoint,
which reads the `scrape_cache` row and returns a `ScrapeDetail`. Passing `?raw=1`
includes the unprocessed `raw_html`; without it the raw field is omitted and the
cleaned/text content is returned.

**Correction (found during implementation): this task does require a backend
change.** It was written as "no backend changes", but `ScrapeDetail` and
`Store.GetScrape` did not select seven of the fetch-metadata fields listed below
— `content_hash`, `etag`, `last_modified`, `tier`, `hit_count`, `expires_at`,
`fetched_at`. The columns already exist on `scrape_cache`; only the struct and
the SELECT needed extending. The change is additive and read-only.

The view shows, for one scrape id:

- A **raw / clean / text toggle**: switch between the raw HTML, the cleaned HTML,
  and the plain `text_content`. Fetch `?raw=1` only when the raw tab is selected
  (raw can be large). Render raw and clean HTML safely (sandboxed frame / escaped
  source), not injected into the app DOM.
- **Images**: the scrape's stored images (the `images` JSON on the cache row).
- **Fetch metadata**: url, http_status, content_type, fetched_with, title,
  robots_allowed, content_hash, etag, last_modified, tier, hit_count, expires_at,
  fetched_at, duration_ms, and any error.

Reached from the run detail (T006), the searches flow, and the fact/provenance
views. Uses the shared API layer (T005).

## Goal

A scrape detail view renders a scrape's raw/clean/text content behind a toggle,
its images, and its fetch metadata from `/api/scrapes/{id}`, fetching the raw
HTML only on demand.

## How to Verify

- Opening a scrape id shows its metadata and, by default, the text/clean content;
  toggling to raw fetches `/api/scrapes/{id}?raw=1` and renders the raw HTML
  safely.
- Images from the scrape render; a scrape with no images renders gracefully.
- All listed fetch-metadata fields display; a missing scrape id surfaces the
  endpoint's 404 as a clear not-found state.
- Loading/error states come from the shared
  layer.
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/ScrapeDetail.svelte` [NEW]
- `web/src/App.svelte`
- `store.go` — seven cache-metadata fields added to `ScrapeDetail`
- `scrapecache.go` — the same seven columns added to the `GetScrape` SELECT

## Dependencies

T005.
