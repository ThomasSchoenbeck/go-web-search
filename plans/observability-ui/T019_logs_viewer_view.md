---
feature: Jobs, Caches, Logs & Stats
task_number: 019
description: Logs viewer view (filters + polling tail)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 019: Logs viewer view

## Description

Build the logs viewer over the T018 endpoint. It refreshes by **polling** (the v1
live-update mechanism; no SSE), using the shared polling helper (T005), to give a
tail-like experience.

- A log table/stream showing created_at, level, source, run_id, and message,
  newest-first.
- **Filters** wired to the endpoint: by run_id, level, and source; plus paging.
- **Polling tail with UI controls:** re-fetch on an interval to append/refresh
  recent lines. Interval and enabled state seed from `/api/ui-config` via the
  shared helper (T005); the view exposes a **dropdown to change the interval** and
  a **button to toggle polling on/off** for the moment (not written back to config),
  starting off if config defaults it off. The poll stops when the view unmounts.
  Level-based styling (error/warn/notice/info) helps scanning.

Uses the shared API layer (T005).

## Goal

A logs viewer lists log rows with run_id/level/source filters and pagination and
auto-refreshes as a polling tail.

## How to Verify

- The viewer lists logs from the T018 endpoint newest-first with created_at,
  level, source, run_id, message; run_id/level/source filters and paging work.
- Polling refreshes the tail on the interval and stops when the view unmounts (no
  leaked timers).
- Levels are visually distinguishable; an empty result renders cleanly.
- The interval dropdown changes the poll cadence live and the toggle button
  stops/starts the tail; both seed from `/api/ui-config` (start off if configured).
- Loading/error come from the shared layer (no bearer — edge auth).
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/LogsViewer.svelte` [NEW]
- `web/src/App.svelte`

## Dependencies

T018.
