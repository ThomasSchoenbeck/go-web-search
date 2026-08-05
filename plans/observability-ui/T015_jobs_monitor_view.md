---
feature: Jobs, Caches, Logs & Stats
task_number: 015
description: Jobs queue monitor view (pending/running/failed, attempts, backoff; polling)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 015: Jobs queue monitor view

## Description

Build the jobs queue monitor over the T014 endpoint. It shows the background work
queue and refreshes by **polling** (the v1 live-update mechanism; no SSE), using
the shared polling helper (T005).

- A table of jobs with type, status, attempts, run_after (next-eligible /
  backoff), locked_at, and timestamps.
- Status and type **filters** wired to the endpoint's query params.
- A visible pending/running/failed breakdown so the queue's health is legible at
  a glance.
- **Polling with UI controls:** re-fetch on an interval so newly claimed/failed/
  retried jobs appear without a manual reload. The interval and enabled state seed
  from `/api/ui-config` defaults via the shared helper (T005); the view exposes a
  **dropdown to change the interval** and a **button to toggle polling on/off** for
  the moment (not written back to config). If config defaults polling to off, it
  starts off. The poll stops when the view unmounts.

Read-only: the view displays state; it does not requeue, cancel, or retry jobs.
Uses the shared API layer (T005).

## Goal

A jobs monitor view lists queue rows with status/attempts/backoff, supports
status/type filters, shows a pending/running/failed breakdown, and auto-refreshes
by polling.

## How to Verify

- The view lists jobs from the T014 endpoint with type, status, attempts,
  run_after/backoff, locked_at, timestamps; status/type filters work.
- Polling refreshes the list on the interval and stops when the view unmounts (no
  leaked timers).
- A pending/running/failed summary is visible; an empty queue renders cleanly.
- The interval dropdown changes the poll cadence live and the toggle button
  stops/starts polling; both seed from `/api/ui-config` (start off if configured).
- Loading/error come from the shared layer (no bearer — edge auth).
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/JobsMonitor.svelte` [NEW]
- `web/src/App.svelte`

## Dependencies

T014.
