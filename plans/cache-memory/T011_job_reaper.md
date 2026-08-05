---
feature: Unified Job System
task_number: 011
description: Reaper — startup + periodic reset of stale running rows
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 011: Reaper

## Description

Add the reaper, the crash-recovery path for the job system. If the process dies
while jobs are in `running`, those rows would otherwise stay locked forever. The
reaper resets stale `running` rows back to `pending` (using the reset-stale
operation from T009): once on startup, and then periodically on an interval. A
row is stale when its `locked_at` is older than a configurable threshold.

Implement it as part of the job system (extend `jobs.go`) so it starts and stops
with the runner. The periodic pass is a managed recurring behavior, not an
ad-hoc goroutine started elsewhere — align it with the recurring-job model
(T012) or run it as an internal ticker owned by the runner. Log how many rows it
reset so a crash is visible in the logs.

## Goal

Stale `running` jobs are returned to `pending` on startup and on a periodic
interval, so a crash mid-job never permanently strands work.

## How to Verify

- `jobs.go` runs a reaper pass at startup and on an interval, using the T009
  reset-stale operation with a configurable staleness threshold.
- `jobs_test.go` (or a reaper test) inserts a running row with an old
  `locked_at`, runs the reaper, and asserts the row is back to pending; a
  freshly-locked row is left alone.
- The number of reset rows is logged.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `jobs.go`
- `jobs_test.go`

## Dependencies

T009.
