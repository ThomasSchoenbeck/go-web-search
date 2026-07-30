---
feature: Memory, Distillation & Confidence Gating
task_number: 026
description: Semantic upsert on store (update near-identical fact vs insert)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 026: Semantic upsert on store

## Description

Implement semantic upsert for `memory_facts` in a new `memory.go`. When storing a
new fact, first check whether a near-identical fact already exists: embed/compare
against existing fact vectors (T016) and, if the nearest existing fact is above
the configured similarity threshold (T006), UPDATE that fact in place (refresh
its content, volatility, validity horizon, provenance, and slide its expiry)
rather than inserting a duplicate. Otherwise INSERT a new fact with a null vector
and enqueue an embed job (T015).

This keeps memory from filling with restatements of the same fact. Because a
brand-new fact has no vector yet, define the behavior when semantic comparison is
unavailable (during a re-embed migration, T016 reports unavailable): fall back to
inserting, accepting occasional duplicates that a later pass can reconcile — note
this trade-off in the task. Follow store conventions: context-first, UUIDv7,
RFC3339, explicit errors.

## Goal

Storing a fact updates an existing near-identical fact when similarity exceeds
the threshold, otherwise inserts a new fact and enqueues its embed job.

## How to Verify

- `memory.go` exposes a context-first upsert that compares against existing fact
  vectors via T016 using the T006 threshold.
- A fact near-identical to an existing one updates that row (no new row); a
  distinct fact inserts a new row and enqueues an embed job.
- When semantic comparison is unavailable, the upsert inserts (documented
  fallback).
- `memory_test.go` covers update-on-match, insert-on-distinct, and the
  unavailable-fallback path.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `memory.go` [NEW]
- `memory_test.go` [NEW]

## Dependencies

T024, T016, T025.
