---
feature: Foundation & Documentation
task_number: 037
description: README refresh + end-to-end integration tests
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 037: README refresh + end-to-end integration tests

## Description

Final documentation and cross-cutting verification pass. Once all prior tasks
land, the README must describe the system as it actually behaves — not the
planned architecture from T001. Refresh `README.md` to cover: the three stores
and their keys, the unified DB-backed job system (poller, worker pool, reaper,
recurring jobs), deferred embeddings and the blue/green re-embed migration, the
tier-vs-volatility axes, the resolver chain (search and scrape paths), the LLM
provider config in `config.toml`, and the MCP/REST surface including source tags
and provenance. Update the file map, the mode list (browse + serve only), the
Concurrency table (new `max_open_conns`), and the "Known limits" section (the
re-scrape-inserts-a-new-row limitation is now fixed).

Add an end-to-end integration test pass that exercises the full flow against
SQLite (as the existing suite does), stubbing the LLM endpoints so no live model
is required: store a search result and confirm exact then semantic cache hits;
scrape a URL and confirm a second scrape is a cache hit with provenance; distill
a page into facts and confirm memory retrieval and confidence gating; run a
cleanup job and confirm expired rows and their vectors are gone. Each earlier
task already carries its own unit verification; this task adds the cross-cutting
integration coverage.

## Goal

`README.md` matches final behavior, and an end-to-end integration test suite
covers cache, scrape cache, memory, and cleanup against SQLite with stubbed LLM
endpoints.

## How to Verify

- `README.md` reflects: browse+serve modes only, three stores, job system,
  re-embed migration, tier/volatility, resolver chain, LLM config, MCP/REST
  source tags and provenance; the fixed re-scrape limit is removed from "Known
  limits".
- A new integration test file runs green: `go test ./...` passes including the
  end-to-end scenarios.
- The integration tests use in-memory/temp SQLite and stubbed HTTP LLM
  endpoints; no live model or network is required.
- `go fmt` and `go vet` are clean.

## Files to Touch

- `README.md`
- `integration_test.go` [NEW]

## Dependencies

All prior tasks (T001–T036, T038).
