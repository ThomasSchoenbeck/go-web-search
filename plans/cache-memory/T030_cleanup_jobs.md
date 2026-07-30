---
feature: Tiering & Cleanup
task_number: 030
description: Per-store cleanup jobs (delete expired) + cascade vector deletion
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 030: Per-store cleanup jobs + cascade vector deletion

## Description

Add cleanup jobs that delete expired rows from each store and remove their
vectors. There is one cleanup job per store (search cache, scrape cache, memory),
each deleting rows whose `expires_at` is in the past, registered as recurring
managed jobs (T012) on the intervals from T006. When a data row is deleted, its
vector rows in the `vectors` table (T014) must be removed too — via the cascade
delete defined in T014, or an explicit cascade in the cleanup helper if native
cascade is unavailable.

Note (from the index): `clean.go` does HTML content cleaning, not TTL cleanup —
there is no existing cleanup to build on, so this is created from scratch in a new
`cleanup.go`. Follow go-agent conventions: context-first, explicit errors, and
log how many rows/vectors each pass removed. Keep the three cleanups as distinct
registered jobs so each can be tuned and observed independently.

## Goal

Three recurring cleanup jobs delete expired rows from the search cache, scrape
cache, and memory respectively, and every deleted row's vectors are removed too.

## How to Verify

- `cleanup.go` defines one cleanup per store, each deleting `expires_at < now`
  rows, registered as recurring jobs (T012) on the T006 intervals.
- Deleting a data row removes its associated vector rows (verified in a test).
- Non-expired rows and their vectors are untouched.
- `cleanup_test.go` against SQLite seeds expired and fresh rows per store, runs
  each cleanup, and asserts only expired rows and their vectors are gone.
- Each pass logs counts.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `cleanup.go` [NEW]
- `cleanup_test.go` [NEW]

## Dependencies

T012, T018, T021, T024, T014.
