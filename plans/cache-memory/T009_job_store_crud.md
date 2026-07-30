---
feature: Unified Job System
task_number: 009
description: Job store CRUD (enqueue, claim, complete, fail+backoff, reset stale)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 009: Job store CRUD

## Description

Implement the data-access layer for the `jobs` table (T008) in a new
`jobstore.go`. Operations: enqueue a job (type + JSON payload, status pending,
attempts 0, run_after now); claim the next runnable job atomically (oldest
pending row whose `run_after` has passed, flipping it to running and stamping
`locked_at`, so two workers never take the same row); complete a job (status
done); fail a job (increment `attempts`, set status back to pending — or failed
past a max — and set `run_after = now + backoff`); and reset stale running rows
(the reaper's query: rows stuck in running past a threshold return to pending).

Follow store conventions in `store.go`: methods on the `Store`, context-first,
UUIDv7 ids, RFC3339 timestamps, explicit error handling, and audit columns kept
current on writes. The atomic claim is the delicate part — it must be a single
statement or a transaction that cannot hand the same row to two claimers, which
is why T002 raises the connection count so real concurrency is possible.

## Goal

`Store` gains enqueue, claim, complete, fail-with-backoff, and reset-stale
operations against `jobs`, with an atomic claim.

## How to Verify

- `jobstore.go` exposes the five operations, all context-first, as `Store`
  methods.
- `jobstore_test.go` against SQLite: enqueue then claim returns the row as
  running with `locked_at` set; a second claim returns nothing; complete marks it
  done; fail increments attempts and pushes `run_after` forward; reset-stale
  returns an overdue running row to pending.
- A concurrency test claims from multiple goroutines and asserts no job is
  claimed twice.
- Backoff grows with attempts.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `jobstore.go` [NEW]
- `jobstore_test.go` [NEW]

## Dependencies

T008.
