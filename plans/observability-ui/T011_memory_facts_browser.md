---
feature: Memory & Semantic Explorer
task_number: 011
description: Memory facts browser view (reuses /api/memory/facts + /{id})
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 011: Memory facts browser view

## Description

Build the memory facts browser over the existing endpoints unchanged:
`GET /api/memory/facts` (list; query params `limit`, `offset`, `q` for a
case-insensitive text substring) and `GET /api/memory/facts/{id}` (fact detail —
returns the fact plus, when the source page is still cached, the source scrape's
sizes and a `read_raw` link to `/api/scrapes/{id}?raw=1`). No backend changes.

- **Facts list:** a searchable, paginated list of distilled facts (text,
  source_url, volatility, tier, hit_count, expires_at). Wire the `q` search box
  and `limit`/`offset` paging to the endpoint.
- **Fact detail:** for a selected fact, show its full text and metadata plus the
  source-scrape sizes (the bloat signal) when present, with the `read_raw` link
  opening the source scrape (T008). When the source page is no longer cached, show
  the endpoint's note. Link `source_url` to the provenance view (T010) — this is
  the **reverse fact→sources** path: from a fact, jump to the URL pivot and trace
  back to the searches that surfaced it.

Uses the shared API layer (T005).

## Goal

A memory facts browser lists facts with text-search and pagination and opens a
fact detail showing the fact, its source-scrape sizes, and a link to the raw
source, using the existing memory endpoints.

## How to Verify

- The list renders facts from `/api/memory/facts` and the `q` box + paging drive
  `q`/`limit`/`offset`.
- Opening a fact shows detail from `/api/memory/facts/{id}`, including source
  sizes and the `read_raw` link when present, and the "no longer cached" note
  otherwise.
- `source_url` links to the provenance view (T010); `read_raw` opens the scrape
  detail (T008).
- Loading/error from the shared layer (no bearer — edge auth).
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/FactsBrowser.svelte` [NEW]
- `web/src/views/FactDetail.svelte` [NEW]
- `web/src/App.svelte`

## Dependencies

T005.
