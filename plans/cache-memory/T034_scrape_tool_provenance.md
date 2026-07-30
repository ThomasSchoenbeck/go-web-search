---
feature: MCP/REST Surface
task_number: 034
description: scrape tool/endpoint (use_cache) + provenance
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 034: `scrape` tool/endpoint + provenance

## Description

Extend the `scrape` surface (the existing `web_scrape` MCP tool and
`POST /api/scrape` in `server.go`) so it routes through the resolver's scrape
path (T031/T023) and exposes `use_cache` (default true) to disable the cache when
a caller explicitly wants a fresh fetch. The response surfaces provenance per URL
— whether the content came from the cache or was fetched live (and how) — using
the provenance field added to `ScrapeOutcome` in T023.

Update the shared `ScrapeRequest`/`ScrapeResponse` shapes and both transports so
REST and MCP stay in lockstep. Cache is on by default; `use_cache=false` forces a
live fetch. Keep the existing snippet behavior (capped text alongside ids) and
the run linking semantics intact. Follow the existing `server.go` patterns.

## Goal

`scrape` accepts `use_cache` (default true), routes through the scrape-path
resolver, and returns per-URL provenance (cache vs live) across both REST and
MCP.

## How to Verify

- `ScrapeRequest` gains `use_cache`; the response exposes provenance per result.
- Default calls use the cache; `use_cache=false` forces a live fetch (verified
  via a fetch counter or fake resolver).
- Both `POST /api/scrape` and the `web_scrape` MCP tool expose the same behavior.
- Snippets and run linking still work.
- Tests assert the default, the disable switch, and provenance values on both
  transports.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `server.go`
- `server_test.go`

## Dependencies

T023, T031.
