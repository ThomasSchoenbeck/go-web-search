---
feature: Jobs, Caches, Logs & Stats
task_number: 018
description: NEW endpoint — logs query over the separate log DB (run_id/level/source filters, pagination) + wire log-read into the server
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 018: NEW logs query endpoint + wire log-read into the server

## Description

Add a read path over the log database and expose it as a new read-only endpoint.
This task is larger than a plain handler because of two grounding facts:

1. **`LogStore` is write-only today.** `logstore.go` is a batching writer goroutine
   with no method to read logs back. A read query over the `logs` table
   (schema_logs.sql: id, run_id, level, source, message, created_at) must be added
   — with filters by `run_id`, `level`, and `source`, plus `limit`/`offset`
   pagination ordered by `created_at` (newest-first for a tail).
2. **`apiServer` does not hold the log DB handle.** It holds only the main `Store`.
   `main.go` opens the `LogStore` (`openLogStore`) and wraps it in a `dbLogWriter`,
   but that handle is not passed into `serveMode` → `newAPIServer`. So this task
   must **wire a log-read path through serve mode**: thread the log DB handle (or a
   small read-only accessor over it) from `main.go`/`serveWithBrowser` →
   `serveMode` → `newAPIServer` so the handler can query it. Flag this as touching
   serve-mode wiring, not just `server.go`.

Add the read method (on `LogStore` or a small companion reader), thread the handle
into the server, register the route in `newAPIServer`, and use
`writeJSON`/`writeErr`. Read-only — no log writing or deletion via the endpoint.

## Goal

A new GET endpoint queries the separate log DB with run_id/level/source filters
and pagination, backed by a new log-read query, with the log DB handle threaded
from `main.go` through `serveMode` into `newAPIServer` so the handler can reach it.

## How to Verify

- `go fmt`, `go vet`, `go test` pass; a test (SQLite log DB) covers the read query
- Backend tests run against an isolated temporary database and remove all test data on completion — no shared or production database is touched.
  with run_id/level/source filters and pagination.
- The log DB handle reaches `newAPIServer` via the serve-mode wiring
  (`main.go`/`serveWithBrowser` → `serveMode` → `newAPIServer`); the server no
  longer depends only on the main `Store` for this endpoint.
- Calling the endpoint returns log rows (id, run_id, level, source, message,
  created_at) newest-first; filters and `limit`/`offset` work; an empty result is
  not an error.
- The route is registered in `newAPIServer` and behind the shared `/api` middleware (`withAuth`, a no-op when `api_key` is unset — edge auth).

## Note added after Phase 1 (test entry points)

Serve mode is no longer the only path into `newAPIServer`. The test harness added
in T024 reaches it two other ways, and both must be updated when this task
changes the constructor's wiring, or the frontend and Go tests stop compiling:

- `testEnv.Server()` in `testsupport.go` — used by the Go endpoint tests.
- `testServeMode` in `testsupport.go`, behind `-mode testserve` — the browserless
  server the Playwright harness runs.

Also: `seedTestData` currently writes only to the **main** database, so the log
DB is empty in every test run. This task (or T019) must seed log rows there, or
the logs viewer has nothing to render in e2e and the filters cannot be exercised.

## Files to Touch

- `logstore.go`
- `server.go`
- `main.go`
- `testsupport.go` — log-store wiring for both test entry points, plus log fixtures
- `logstore_test.go` [NEW]

## Dependencies

T005.
