---
feature: Provenance / Causality
task_number: 025
description: NEW endpoint — whole-run causality graph (searches→urls→scrapes→facts for a run)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 025: NEW whole-run causality endpoint

## Description

Add a new read-only endpoint that, given a run id, assembles the run's entire
causality chain as a graph/tree the UI can render — the run-level counterpart to
the single-URL pivot in T009. Confirmed as part of the full provenance scope.

Given a `run_id`, return the connected structure: the run's **searches** (per
term/engine, with status/rank context) → the **urls** each search found and at
what **rank** (`search_urls`) → the **scrape** of each url (from `scrape_cache`,
via `ScrapeSizesByURL` or a batch equivalent) → the **memory_facts** distilled
from each url (`source_url`), with per-fact vector presence in the active vectors
table noted. Shape the payload as nodes + edges (or a nested tree) suitable for a
graph/tree view, deduplicating shared urls across searches.

Grounding to bake in:

- Reuse the T009 store queries where possible (searches-that-found-a-url,
  facts-by-source-url) but batch them by run so the whole graph is one bounded
  response rather than N round trips. Add a run-scoped read query alongside the
  T009 provenance queries.
- Bound the response: a large run can have many urls/facts, so cap/paginate or
  summarize where a node's children are large, and expose any cap in config rather
  than hardcoding.
- Degrade gracefully: when the vector store is mid-migration or absent, include
  facts without vector info rather than erroring, mirroring T009/Stats.

Register the route in `newAPIServer` behind the shared `/api` middleware
(`withAuth`, a no-op when `api_key` is unset — edge auth); use `writeJSON`/
`writeErr`. Read-only: no writes, no scrape/search/distill triggering.

## Goal

A new GET endpoint takes a run id and returns the run's full causality graph
(searches → urls+rank → scrapes → facts, with per-fact vector presence) as a
bounded nodes/edges (or nested-tree) payload, reusing the T009 provenance queries
batched per run and degrading gracefully when vectors are unavailable.

## How to Verify

- `go fmt`, `go vet`, `go test` pass; a store-level test against SQLite covers the
  run-scoped assembly (searches → urls+rank → scrapes → facts) and dedupe of a
  url shared by two searches. Backend tests run against an isolated temporary
  database and remove all test data on completion — no shared or production
  database is touched.
- Calling the endpoint with a known run id returns its connected graph; an unknown
  run id returns an empty/normal result, not an error.
- The response is bounded (cap/pagination/summarization for large runs), with the
  cap read from config, not hardcoded.
- With no active vector table / migration in flight, facts are returned without
  vector info rather than failing.

## Files to Touch

- `provenance.go`
- `provenance_test.go`
- `server.go`
- `config.go`
- `config.toml`

## Dependencies

T009.
