---
feature: Memory, Distillation & Confidence Gating
task_number: 027
description: Memory retrieval (top-k relevant facts)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 027: Memory retrieval

## Description

Add memory retrieval to `memory.go`: given a query, embed it as a query
(asymmetric prefix) and use the vector search helper (T016) to return the top-k
most relevant facts from `memory_facts`, with their similarity scores and the
metadata the gate needs (volatility, validity horizon, provenance, expiry). This
is the read side of memory that the resolver chain (T032) and the `memory.query`
tool (T035) consume, and the input to confidence gating (T028).

Respect the migration degradation contract: when T016 reports semantic
unavailable, retrieval returns no facts (memory misses to the web) rather than
inventing a non-vector path. Retrieval itself does not decide whether a fact is
good enough to skip a web search — it just returns candidates and scores; the
gating decision is T028. Follow store conventions: context-first, explicit
errors.

## Goal

A retrieval operation returns the top-k relevant facts for a query with
similarity scores and gating metadata, and returns nothing when semantic search
is unavailable.

## How to Verify

- `memory.go` exposes a context-first top-k retrieval using T016.
- Returned facts include score, volatility, validity horizon, provenance, and
  expiry.
- A relevant query returns matching facts ordered by similarity; an unrelated
  query returns none above threshold.
- When T016 reports unavailable, retrieval returns empty without error.
- `memory_test.go` covers a relevant hit, ordering, threshold miss, and the
  unavailable path.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `memory.go`
- `memory_test.go`

## Dependencies

T024, T016.
