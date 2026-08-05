---
feature: Provenance / Causality
task_number: 009
description: NEW endpoint — provenance pivot for a URL (backward searches+rank, forward scrape→facts→vectors)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 009: NEW provenance-for-a-URL endpoint

## Description

Add a new read-only endpoint that pivots on a single URL and returns its
provenance chain in both directions — the "what caused what" the plan is built
around. No such endpoint exists today; this needs new store read queries plus the
handler and route.

Given a URL (query param or path segment), return:

- **Backward (what led here):** which searches found this URL and at what rank.
  Resolve the URL to its `urls.id`, join `search_urls` (search_id, url_id, rank)
  to `searches` to list each finding search with its term, engine, run_id, and
  rank.
- **Forward (what came from here):** the scrape of this URL and what was distilled
  from it. Use the existing `ScrapeSizesByURL` (scrapecache.go) to resolve the URL
  to its `scrape_cache` row/scrape id; list the `memory_facts` whose `source_url`
  equals this URL; and, for those facts, note whether they have a vector in the
  active vectors table (owner_kind `memory`).

New store queries live alongside the existing read helpers (e.g. a
`provenance.go` or additions to store.go): searches-that-found-a-url (backward)
and facts-by-source-url (forward). Reuse `ScrapeSizesByURL` for the scrape link
and `activeVectorTable` to name the vectors table. Degrade gracefully when the
vector store is mid-migration or absent (report facts without vector info rather
than error). Register the route on the mux in `newAPIServer` and use
`writeJSON`/`writeErr` like the other handlers. Read-only: no writes, no scrape
or distill triggering.

## Goal

A new GET endpoint takes a URL and returns its backward chain (searches + rank +
run) and forward chain (scrape id/sizes, distilled facts, and per-fact vector
presence), backed by new store read queries and reusing `ScrapeSizesByURL`.

## How to Verify

- `go fmt`, `go vet`, `go test` pass; a store-level test against SQLite covers
- Backend tests run against an isolated temporary database and remove all test data on completion — no shared or production database is touched.
  the backward (searches+rank) and forward (facts-by-source-url) queries.
- Calling the endpoint with a URL known to the DB returns its finding searches
  with rank/run and its scrape + facts; an unknown URL returns an empty/normal
  result, not an error.
- With no active vector table (or a migration in flight) the endpoint still
  returns the chain, omitting or flagging vector info rather than failing.
- The route is registered in `newAPIServer` and behind the shared `/api` middleware (`withAuth`, a no-op when `api_key` is unset — edge auth) like other
  `/api/*` routes.

## Files to Touch

- `provenance.go` [NEW]
- `provenance_test.go` [NEW]
- `server.go`

## Dependencies

T005.
