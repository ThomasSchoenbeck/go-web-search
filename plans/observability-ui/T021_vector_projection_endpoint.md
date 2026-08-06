---
feature: Embeddings 2-D Projection (later phase)
task_number: 021
description: NEW endpoint — vector projection data dump (memory + search owners) for scatter rendering
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 021: NEW vector projection data endpoint

## Description

Add a new read-only endpoint that dumps bounded raw embedding vectors so the
browser can compute a 2-D layout. Per the confirmed decision, **the 2-D
projection (PCA) runs client-side (T022)** — this endpoint returns the raw
vectors, and adds **no new Go dependency**.

For both owner kinds (`ownerMemory` and `ownerSearch`, see vectors.go), read from
the active vectors table: each row's owner id, owner_kind, and its embedding
vector, plus a small label per point (fact text snippet for memory, query for
search) so the scatter can annotate points. Resolve labels via the memory /
search_cache read paths.

Grounding to bake in:

- **Bound the result set.** `VectorSearch`/the vectors table is an exact linear
  scan and each embedding is large (e.g. dim 4096). Reading whole vectors for
  every row is heavy, so cap the number of points returned and support
  pagination/sampling. Expose the sample cap as **config** rather than hardcoding
  it (the plan's externalized-config rule; e.g. a projection sample-size setting
  in the `server` or a UI block of `config.toml`).
- **Degrade gracefully.** The active table name comes from `activeVectorTable`;
  while a re-embed migration is in flight or no table exists, return an
  empty/"unavailable" result with a note, mirroring `Stats` — never a 500.

Register the route in `newAPIServer`, use `writeJSON`/`writeErr`. Read-only.

## Goal

A new GET endpoint returns a bounded, sampled set of raw embedding vectors for the
memory and search owner kinds, each with an owner id and a short label, with the
sample cap in config and graceful degradation when the vector store is
unavailable — and with no new Go dependency.

## How to Verify

- `go fmt`, `go vet`, `go test` pass; a test (SQLite, stubbed vectors) confirms
- Backend tests run against an isolated temporary database and remove all test data on completion — no shared or production database is touched.
  the dump returns bounded rows for both owner kinds with labels and respects the
  cap.
- The endpoint returns raw vectors + owner id + owner_kind + label, capped by the
  configured sample size and paginable; no projection compute or new dependency is
  added on the Go side (`go.mod` unchanged).
- With no active vector table / migration in flight, it returns an empty result
  with a note, not a 500.
- The route is registered in `newAPIServer`, behind the shared `/api` middleware (`withAuth`, a no-op when `api_key` is unset — edge auth); the sample cap
  is read from config, not hardcoded.

## Note added during implementation

- **`config.go` and `config.toml` needed no change.** The task expected the cap
  to be added here, but `observability.projection_sample_cap` already exists —
  T004 added it, and `/api/ui-config` already serves it. The handler passes it to
  the store; the view asks for it. Nothing new was introduced.
- **Reading a vector back out of an `F32_BLOB` column: `vector_extract()` works.**
  vectors.go documents what the Rust Turso engine does *not* implement
  (`libsql_vector_idx`, `vector_top_k`) but said nothing about extraction. It was
  probed against the real engine: `vector_extract(embedding)` returns the same
  `'[a,b,c]'` text `vector32()` accepts. The inverse parser, `parseVectorLiteral`,
  therefore sits in `vectors.go` beside `vectorLiteral` rather than in
  `projection.go` — it is that function's mirror image, not a projection concern.
  Decoding the raw blob bytes also works (little-endian float32, 4 bytes each)
  but hard-codes a storage layout the engine never promised, so it was not used.
- The route is `GET /api/projection?limit=&offset=`. `limit` is clamped to the
  configured cap, so a caller cannot ask for more than deployment policy allows.
- A vector whose owning row was deleted is skipped rather than plotted as an
  anonymous dot, matching the explorer. `total` still counts it, because it is
  genuinely in the store.
- `go.mod` is unchanged, as required.

## Files to Touch

- `projection.go` [NEW]
- `projection_test.go` [NEW]
- `vectors.go` — `parseVectorLiteral`, the inverse of `vectorLiteral`
- `server.go`

## Dependencies

T012.
