---
feature: Memory & Semantic Explorer
task_number: 012
description: NEW endpoint — semantic explorer: embed query text, VectorSearch memory+search, return neighbors+distance
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 012: NEW semantic-explorer endpoint

## Description

Add a new read-only endpoint that is a raw nearest-neighbor tool over the vector
store — distinct from the existing `/api/memory/query`. `memory/query` runs
confidence gating and synthesizes an answer over the `memory` owner only; the
explorer does none of that. It embeds arbitrary query text once (reusing
`LLMClient.Embed` as a query) and runs `Store.VectorSearch` over **both** owner
kinds — `ownerMemory` and `ownerSearch` (see vectors.go) — returning the top-k
neighbors with cosine **distance** (similarity = 1 − distance), no gating, no
synthesis.

Resolve each neighbor id to something displayable: `memory` ids to their fact
text (via the memory read path), `search` ids to their cached query/results (via
the search_cache read path). The request carries the query text and an optional
`k`; the response lists neighbors grouped or tagged by owner kind, each with
distance and the resolved text.

Grounding to bake in: `VectorSearch` is an exact linear scan (no ANN index), so
keep `k` bounded. The active table comes from `activeVectorTable`; while a re-embed
migration is in flight (`ready == false`) or no table exists, semantic reads are
unavailable — the endpoint must degrade gracefully (return an empty result with a
"migration in progress"/"no vectors yet" note, mirroring `Stats`), not error.
Register the route in `newAPIServer` and use `writeJSON`/`writeErr`. Read-only.

## Goal

A new GET/POST endpoint embeds query text once, runs `VectorSearch` over the
`memory` and `search` owner kinds, and returns top-k neighbors with cosine
distance resolved to fact text / cached query — with graceful degradation when the
vector store is unavailable.

## How to Verify

- `go fmt`, `go vet`, `go test` pass; a test (SQLite, stubbed embeddings) confirms
- Backend tests run against an isolated temporary database and remove all test data on completion — no shared or production database is touched.
  the endpoint queries both owner kinds and returns neighbors with distance
  resolved to text.
- A query against a populated store returns neighbors tagged by owner kind
  (memory/search) with cosine distance and resolved text, honoring a bounded `k`.
- With no active vector table or a migration in flight, the endpoint returns an
  empty result with an explanatory note instead of a 500.
- The route is registered in `newAPIServer` and behind the shared `/api` middleware (`withAuth`, a no-op when `api_key` is unset — edge auth).

## Files to Touch

- `explorer.go` [NEW]
- `explorer_test.go` [NEW]
- `server.go`

## Dependencies

T005.
