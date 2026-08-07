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

## Note added during implementation

The polling controls are a shared component, `web/src/components/PollControls.svelte`,
not view-local code: T019 needs exactly the same thing (seed from
`/api/ui-config`, interval dropdown, on/off toggle, stop on unmount), and two
copies of a lifecycle that leaks timers when it is got wrong is worse than one.
It owns the poller and takes the view's `reload` as its `task` prop. Its testids
(`poll-toggle`, `poll-interval`, `poll-status`) are unprefixed — only one
polling view is ever mounted at a time.

The breakdown renders from the endpoint's whole-queue `counts` (T014), so
filtering the table does not move it.

## Files to Touch

- `web/src/views/JobsMonitor.svelte` [NEW]
- `web/src/components/PollControls.svelte` [NEW] — shared with T019
- `web/src/App.svelte`
- `web/src/lib/api.ts`, `web/src/lib/routes.ts` — the resource and the route/nav entry

## Dependencies

T014.
