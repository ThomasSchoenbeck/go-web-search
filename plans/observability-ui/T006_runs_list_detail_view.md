---
feature: Core Views — Runs, Searches & SERPs
task_number: 006
description: Runs list + run detail view (reuses /api/runs, /api/runs/{id}, /urls, /searches, /scrapes)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 006: Runs list + run detail view

## Description

Build the first real inspection views over runs, reusing existing endpoints
unchanged: `GET /api/runs` (list, `limit` query param), `GET /api/runs/{id}`
(summary), and the per-run children `GET /api/runs/{id}/urls`,
`/api/runs/{id}/searches`, and `/api/runs/{id}/scrapes` (returns scrape ids).
No backend changes.

- **Runs list:** a table/list of recent runs (mode, started/finished, id) from
  `/api/runs`, each linking to its detail. Support the `limit` param.
- **Run detail:** for a selected run id, show the run summary plus its children —
  the URLs it found (`/urls`), the searches it ran (`/searches`), and the scrape
  ids it produced (`/scrapes`). Link searches to the searches/SERP view (T007) and
  scrape ids to the scrape detail view (T008) so this becomes the navigation hub
  into the rest of the UI.

Uses the shared API layer (T005) for fetching and loading/error state (no bearer — edge auth). Client-side routing selects a run by id.

## Goal

A runs list view and a run detail view render live data from the existing run
endpoints, with the detail view listing the run's URLs, searches, and scrape ids
and linking onward to the search and scrape views.

## How to Verify

- The runs list renders rows from `/api/runs` and respects `limit`; clicking a
  run opens its detail.
- Run detail shows the summary from `/api/runs/{id}` and the three child lists
  from `/urls`, `/searches`, `/scrapes`; empty children render gracefully.
- Search and scrape entries link to the T007 and T008 views (routes resolve even
  before those views are complete).
- Loading and error states come from the shared layer (no bearer — edge auth).
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/RunsList.svelte` [NEW]
- `web/src/views/RunDetail.svelte` [NEW]
- `web/src/App.svelte`

## Dependencies

T005.
