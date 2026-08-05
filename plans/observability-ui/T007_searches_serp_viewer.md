---
feature: Core Views — Runs, Searches & SERPs
task_number: 007
description: Searches view + raw SERP HTML viewer (reuses /api/runs/{id}/searches, /api/searches/{id}/raw)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 007: Searches view + raw SERP HTML viewer

## Description

Build the searches view over a run, reusing existing endpoints unchanged:
`GET /api/runs/{id}/searches` (per-run searches: term, engine, mode, status,
blocked flag, anchor count, duration, error) and `GET /api/searches/{id}/raw`
(the stored raw SERP HTML, served as `text/html`). No backend changes.

- **Searches list:** for a run, show each `(term, engine)` search with its
  metadata (engine, search_mode, http_status, blocked, anchor_count, duration_ms,
  error). Reached from the run detail view (T006).
- **Raw SERP viewer:** for a selected search, let the user view its raw SERP
  HTML from `/api/searches/{id}/raw`. Because the raw HTML is a full untrusted
  document, render it safely — e.g. in a sandboxed `iframe` or as escaped source
  — rather than injecting it into the app DOM. Handle the 404 when no raw HTML is
  stored for that search.

Uses the shared API layer (T005). Note `/api/searches/{id}/raw` returns HTML, not
JSON, so the viewer fetches it as text/opens it in a sandboxed frame rather than
through the JSON client path.

## Goal

A searches view lists a run's searches with their metadata and lets the user open
each search's raw SERP HTML safely (sandboxed), handling the no-raw-HTML case.

## How to Verify

- Selecting a run shows its searches with engine, mode, status, blocked,
  anchor_count, duration, and error fields from `/api/runs/{id}/searches`.
- Opening a search renders its raw SERP HTML from `/api/searches/{id}/raw` in a
  sandboxed frame / escaped view; a search with no stored raw HTML shows a clear
  "no raw HTML" state (the endpoint's 404) rather than an error crash.
- Loading/error states come
  from the shared layer.
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/SearchesList.svelte` [NEW]
- `web/src/views/SerpViewer.svelte` [NEW]
- `web/src/App.svelte`

## Dependencies

T006.
