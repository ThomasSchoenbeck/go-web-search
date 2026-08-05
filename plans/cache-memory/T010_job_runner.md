---
feature: Unified Job System
task_number: 010
description: Job runner — poller goroutine + worker pool + typed handler registry
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 010: Job runner

## Description

Build the job runner in a new `jobs.go`: a poller goroutine that repeatedly
claims runnable jobs (via T009) and dispatches them to a bounded worker pool,
plus a one-place typed handler registry that maps a job type to its handler.
Handlers are registered in a single location (a registry struct or map), never
scattered across `init()` calls. A handler receives the job's decoded payload and
context; returning an error triggers fail-with-backoff (attempts++,
run_after=now+backoff via T009), success triggers complete.

Follow go-agent conventions: context-first, small consumption-site interfaces
(the runner depends on a narrow job-store interface so it can be tested against a
fake), a bounded worker pool sized from config, clean start/stop with context
cancellation, and structured logging. The runner exposes a way to register a
handler and a way to run until its context is cancelled. Recurring jobs (T012)
and lifecycle wiring (T013) build on this; the reaper (T011) is registered
alongside it.

## Goal

A runner polls for jobs, dispatches them to a worker pool, and routes each job to
a handler looked up in a single typed registry, with success completing and
failure applying backoff.

## How to Verify

- `jobs.go` defines the runner with handler registration in one place, a poller,
  and a bounded worker pool.
- `jobs_test.go` registers a fake handler, enqueues jobs, runs the runner against
  SQLite (or a fake store), and asserts: successful jobs are completed; failing
  jobs are retried with backoff; unknown job types are handled gracefully; the
  worker pool bound is respected.
- Cancelling the runner's context stops the poller and drains cleanly.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `jobs.go` [NEW]
- `jobs_test.go` [NEW]

## Dependencies

T009.
