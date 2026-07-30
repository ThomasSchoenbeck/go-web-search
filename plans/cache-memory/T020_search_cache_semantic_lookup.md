---
feature: Search Cache
task_number: 020
description: Semantic search-cache lookup via vector similarity
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 020: Semantic search-cache lookup

## Description

Extend `searchcache.go` with a semantic lookup: embed the incoming query as a
query (asymmetric prefix, via the embed path), run the vector search helper
(T016) against the search-cache vectors, and if the nearest cached query is above
the configured similarity threshold and not expired, return its URLs as a
semantic cache hit. This catches queries that are worded differently but mean the
same thing, which the exact path (T019) cannot.

Respect the migration degradation contract: when T016 reports semantic
unavailable (a re-embed migration is in progress), the semantic lookup is skipped
and callers rely on the exact match only. Use the similarity threshold from T006.
Keep exact and semantic as distinct, composable lookups so the resolver chain
(T032) can try exact first, then semantic.

## Goal

A semantic search-cache lookup returns the URLs of the nearest cached query above
the similarity threshold, and cleanly skips when a migration makes semantic
search unavailable.

## How to Verify

- `searchcache.go` exposes a context-first semantic lookup using T016 and the
  T006 threshold.
- A differently-worded but semantically-equivalent query returns the cached URLs;
  an unrelated query does not.
- When T016 reports semantic unavailable, the semantic lookup returns no hit
  without error.
- `searchcache_test.go` covers a semantic hit, a below-threshold miss, and the
  migration-unavailable path (stubbed vector helper).
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `searchcache.go`
- `searchcache_test.go`

## Dependencies

T019, T016.
