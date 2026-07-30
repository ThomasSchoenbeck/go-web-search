---
feature: Tiering & Cleanup
task_number: 007
description: Tier/expiry calculation helper (pure sliding-expiry + promotion)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 007: Tier/expiry calculation helper

## Description

Write a pure, dependency-free helper (in a new `tier.go`) that computes expiry
and tier transitions from the tunables in T006. Tier is the durability axis:
`short`, `long`, `permanent`. The helper implements sliding expiry (each hit
extends the expiry from now by the tier's window, capped at the tier's ceiling)
and hit-count promotion (enough hits promotes a row to the next tier up). It does
not touch the database or the clock directly beyond an injected "now" so it can
be unit tested deterministically.

Keep this axis strictly separate from volatility: this helper knows nothing about
staleness rate or freshness `max_age`, which the distiller emits and the gate
consumes (T025/T028). Explain in the task that the split exists because
durability (how long we keep a row) and staleness (how fast its content goes out
of date) are independent decisions.

Being pure and injectable makes it the natural unit-test target that the three
stores (T029) and cleanup (T030) reuse.

## Goal

A pure helper computes a new expiry timestamp and, when warranted, a promoted
tier, given the current tier, hit count, last expiry, and an injected now, using
the T006 tunables.

## How to Verify

- `tier.go` exposes functions that take current tier, hit count, and an injected
  now, returning the new expiry and (possibly promoted) tier — no DB, no
  `time.Now()` call inside the pure path.
- `tier_test.go` is table-driven: short/long/permanent sliding-expiry cases,
  ceiling clamping, promotion at threshold, and no promotion below it.
- Nothing in the helper references volatility or freshness.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `tier.go` [NEW]
- `tier_test.go` [NEW]

## Dependencies

T006.
