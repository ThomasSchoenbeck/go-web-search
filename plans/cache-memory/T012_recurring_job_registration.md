---
feature: Unified Job System
task_number: 012
description: Recurring-job registration as managed goroutines
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 012: Recurring-job registration as managed goroutines

## Description

Add support for recurring jobs to the job system. Per the planner's job-system
rule, recurring interval work runs as persistent goroutines that live for the
process lifetime but are registered with and managed by the job runner — not
started ad-hoc in `init()` or random startup functions. Each recurring job is
defined in one place (a struct/config entry: name, interval, and the work to
run, typically enqueueing a job or performing a periodic pass).

Extend `jobs.go` so a caller registers recurring jobs alongside handlers, and the
runner starts/stops them with its own lifecycle. This is what the cleanup jobs
(T030) and the re-embed migration's boot check (T017) build on. The reaper's
periodic pass (T011) may be expressed through this same mechanism. Keep
registration centralized so there is a single list of everything that runs on a
schedule.

## Goal

Recurring jobs can be registered with the runner from one place and are started
and stopped as managed goroutines under the runner's lifecycle.

## How to Verify

- `jobs.go` exposes recurring-job registration where each entry declares its
  interval and work in one place.
- Starting the runner starts the recurring goroutines; cancelling the runner
  stops them.
- `jobs_test.go` registers a fast-interval recurring job, runs the runner
  briefly, and asserts the work executed at least once and stopped on cancel.
- No recurring work is started from `init()`.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `jobs.go`
- `jobs_test.go`

## Dependencies

T010.
