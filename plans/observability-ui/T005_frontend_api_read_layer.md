---
feature: Foundation — Build, Embed & Serving
task_number: 005
description: Shared frontend API-read layer (typed fetch client, loading/error, config-driven polling helper)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 005: Shared frontend API-read layer

## Description

Build the single shared read layer every view uses to talk to the backend, so no
view hand-rolls fetch or polling. It lives under `web/src/lib/` and provides:

- A typed fetch client that resolves the API base from the SPA's own origin (no
  hardcoded host/port — the SPA is served from the same listener) and issues GET
  requests to `/api/*`. **No bearer/token handling:** auth is delegated to the
  edge (T004), so the client makes plain same-origin requests.
- Uniform loading/error handling: a small wrapper (Svelte store or helper) that
  exposes `loading`, `error`, and `data` states so views render spinners/errors
  consistently.
- **UI settings from config:** on startup, read `GET /api/ui-config` (T004) once to
  get the default poll interval, the poll-enabled default, and the projection
  sample cap, and expose them to the app as the session defaults.
- A **config-driven polling helper** that re-fetches an endpoint on an interval and
  can be started/stopped. It initializes from the `/api/ui-config` defaults (and
  may start disabled if config says so). The current interval and enabled state are
  overridable at runtime by the UI — the jobs (T015) and logs (T019) views wire a
  dropdown (interval) and a toggle button (on/off) to it — but overrides are for
  the moment only and are not written back to config. Polling — not SSE — is the
  v1 live-update mechanism.

This task only builds the layer and a trivial smoke usage (e.g. calling
`/api/stats` and rendering the raw JSON) to prove it works end to end against the
running serve binary. Individual views consume it in later tasks.

## Goal

A reusable `web/src/lib` API module exposes a typed same-origin GET client with
consistent loading/error state, reads UI defaults from `/api/ui-config`, and
provides a config-seeded start/stop polling helper whose interval and enabled
state can be overridden live by the UI — proven by a smoke call to an existing
endpoint.

## How to Verify

- A view or test calling the client for `/api/stats` renders the returned JSON and
  shows a loading state first, then data.
- The layer reads `/api/ui-config` and seeds the poll interval, poll-enabled
  default, and projection sample cap from it (no hardcoded values); when config
  disables polling, the helper starts stopped.
- The polling helper re-fetches on the current interval, stops cleanly when torn
  down, and honors a runtime interval/enabled override (verified against
  `/api/stats` or `/api/runs`).
- The API base is derived from the current origin (no hardcoded host/port); the
  client sends no Authorization header.
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover the API-read layer's major functions — loading/error
  handling, `/api/ui-config` seeding, and the polling helper's start/stop +
  interval/enable override; `pnpm test` passes.

## Files to Touch

- `web/src/lib/api.js` [NEW]
- `web/src/lib/poll.js` [NEW]
- `web/src/lib/request.js` [NEW]
- `web/src/lib/uiconfig.js` [NEW]

## Dependencies

T004.
