---
feature: Tiering & Cleanup
task_number: 029
description: Apply sliding expiry + tier promotion on hits across all three stores
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 029: Apply sliding expiry + tier promotion on hits

## Description

Wire the pure tier/expiry helper (T007) into the read paths of all three stores
so that every cache/memory hit slides the row's expiry forward and increments its
hit_count, promoting the tier when the promotion threshold is reached. Apply this
consistently in: the search cache on an exact or semantic hit (T019/T020), the
scrape cache on a hit (T023), and memory on a fact retrieval hit (T027).

This is the durability axis in action: frequently-used rows live longer and can
be promoted toward `permanent`. Keep it separate from the freshness/volatility
axis — a hit extends durability but does not change a fact's volatility or
validity horizon. Update each store's hit path to call the helper and persist the
new expiry/tier/hit_count. Follow store conventions: context-first, explicit
errors, audit columns kept current.

## Goal

A hit in any of the three stores slides expiry forward, increments hit_count, and
promotes the tier at the threshold, using the shared T007 helper.

## How to Verify

- Search-cache, scrape-cache, and memory hit paths all call the T007 helper and
  persist the updated expiry, hit_count, and (when promoted) tier.
- A row hit enough times is promoted to the next tier; its expiry moves forward
  on each hit up to the tier ceiling.
- Hits do not alter volatility/validity-horizon fields.
- Tests for each store assert expiry extension, hit_count increment, and
  promotion at the threshold.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `searchcache.go`
- `scrapecache.go`
- `memory.go`

## Dependencies

T007, T019, T023, T027.
