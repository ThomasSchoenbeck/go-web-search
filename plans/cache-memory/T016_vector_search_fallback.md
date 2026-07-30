---
feature: Vector Store, Embeddings & Migration
task_number: 016
description: Vector search helper (semantic; degrades to exact/miss only during re-embed migration)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 016: Vector search helper

## Description

Add a helper (extend `vectors.go`) that performs a semantic nearest-neighbor
search against the `vectors` table's ANN index (T014), returning the owner rows
whose embeddings are closest to a query embedding, with their similarity scores,
filtered by store discriminator and by a caller-supplied similarity threshold and
top-k. This is the shared retrieval primitive for the semantic search-cache
lookup (T020) and memory retrieval (T027).

There is NO fallback vector implementation — we are fully on Turso native
vectors. The only degradation is migration-time: while a blue/green re-embed
(T017) is running, semantic lookup is unavailable, so this helper (or its
callers) must detect the in-progress migration state (from `system_meta`) and
signal that semantic search is off. In that window the search cache falls back to
exact query match and memory simply misses to the web; this helper's contract is
to report "semantic unavailable" rather than to invent a second vector path.
Verify the ANN query syntax against current Turso docs, matching T014.

## Goal

A semantic top-k nearest-neighbor helper over the `vectors` table returns
owner rows with similarity scores above a threshold, and cleanly reports
"semantic unavailable" when a re-embed migration is in progress.

## How to Verify

- `vectors.go` exposes a context-first ANN search filtered by store, threshold,
  and top-k, returning owner ids and scores.
- When `system_meta` marks a migration in progress, the helper reports semantic
  unavailable rather than returning partial results.
- ANN query syntax is confirmed against current Turso docs.
- A test (against the Turso driver, or documented Turso-only coverage) inserts
  vectors and asserts nearest neighbors and threshold filtering; a second test
  asserts the migration-in-progress path reports unavailable.
- `go fmt`, `go vet` pass.

## Files to Touch

- `vectors.go`
- `vectors_test.go`

## Dependencies

T014, T015.
