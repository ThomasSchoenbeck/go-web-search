---
feature: Jobs, Caches, Logs & Stats
task_number: 014
description: NEW endpoint — jobs list with status/type filters + pagination over the jobs table
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 014: NEW jobs list endpoint

## Description

Add a new read-only endpoint that lists rows from the `jobs` table so the UI can
monitor the background queue. There is no read endpoint for jobs today (only the
`pending_jobs` count in stats). Add a store read query plus the handler and route.

The `jobs` table (schema.sql) has: id, type, payload, status, attempts,
run_after, locked_at, created_at, updated_at. The endpoint returns a page of jobs
with:

- **Filters:** by `status` (e.g. pending/running/failed/done, matching the values
  the job system uses) and by `type` (embed, distill, cleanup, reembed).
- **Pagination:** `limit`/`offset`, ordered newest-first (or by run_after) so the
  monitor can page through history.

Add the read query alongside the existing job store code (jobstore.go) or in
store.go, register the route in `newAPIServer`, and use `writeJSON`/`writeErr`.
Keep `payload` display-safe (it is arbitrary JSON text). Read-only — no claiming,
retrying, or deleting jobs from this endpoint.

## Goal

A new GET endpoint returns a filtered (status, type), paginated list of `jobs`
rows with their status/attempts/backoff fields, backed by a new store read query.

## How to Verify

- `go fmt`, `go vet`, `go test` pass; a store test (SQLite) covers status/type
- Backend tests run against an isolated temporary database and remove all test data on completion — no shared or production database is touched.
  filtering and pagination.
- Calling the endpoint returns jobs with id, type, status, attempts, run_after,
  locked_at, timestamps; `status` and `type` filters and `limit`/`offset` work.
- An empty queue returns an empty list, not an error.
- The route is registered in `newAPIServer` and behind the shared `/api` middleware (`withAuth`, a no-op when `api_key` is unset — edge auth).

## Note added during implementation

- The endpoint is `GET /api/jobs` and returns `{jobs, counts}` (`JobsPage`), not a
  bare list. `counts` is the whole queue by status — every known status present,
  at zero when nothing holds it — because T015's pending/running/failed
  breakdown must describe the queue, not the filtered page it is showing.
- Payload is returned as the stored string. Job payloads are small JSON objects
  written by the enqueuers; the view renders them as text, never as markup.
- The listing bound (`limit` in [1,500], default 50) is `clampPage` in `store.go`,
  shared with T016 and T018 rather than repeated three times.
- `seedTestData` gained a failed embed (3 attempts), a done embed and a running
  cleanup with a lock, so the filters and the breakdown have every case.

## Files to Touch

- `jobstore.go`
- `server.go`
- `store.go` — the shared `clampPage` listing bound
- `testsupport.go` — jobs covering every status and three types
- `jobstore_test.go` [NEW]

## Dependencies

T005.
