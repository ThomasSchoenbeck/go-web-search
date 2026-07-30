---
feature: Search Cache
task_number: 019
description: Query normalization + exact store/lookup + enqueue embed job
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 019: Query normalization + exact store/lookup + enqueue embed job

## Description

Implement the exact path for the search cache in a new `searchcache.go`.
Normalize a query into the canonical form used as the cache key (for example
trim, lowercase, collapse whitespace — define the exact rules and apply them
consistently on both store and lookup). On store: write the normalized query and
its result URLs to `search_cache` (T018) with a null vector, apply the initial
tier/expiry via the tier helper (T007), and enqueue an `embed` job (T015) so the
query's vector is filled in later. On lookup: return the cached URLs for an exact
normalized-query match if the row has not expired.

Follow store conventions: context-first `Store` methods, UUIDv7 ids, RFC3339
timestamps, explicit errors. Exact-match works immediately because it does not
depend on the vector; the semantic lookup that uses the vector is T020.

## Goal

The search cache can store a normalized query with its URLs (enqueuing an embed
job) and answer an exact normalized-query lookup, honoring expiry.

## How to Verify

- `searchcache.go` exposes context-first store and exact-lookup operations using
  consistent normalization on both sides.
- Storing a query enqueues an embed job and sets initial tier/expiry via T007.
- An exact lookup of an equivalent-but-differently-cased/spaced query hits the
  same row; an expired row does not hit.
- `searchcache_test.go` against SQLite covers normalization equivalence, store +
  exact hit, expiry miss, and that an embed job row is enqueued.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `searchcache.go` [NEW]
- `searchcache_test.go` [NEW]

## Dependencies

T018, T015, T007.
