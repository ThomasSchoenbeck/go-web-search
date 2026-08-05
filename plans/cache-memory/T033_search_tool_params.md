---
feature: MCP/REST Surface
task_number: 033
description: search tool/endpoint params (use_cache, use_memory, max_age) + source tag
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 033: `search` tool/endpoint params + source tag

## Description

Extend the `search` surface (the existing `web_search` MCP tool and
`POST /api/search` in `server.go`) so it routes through the resolver dispatch
(T031/T032) and exposes the new parameters: `use_cache` (default true),
`use_memory` (default true), and an optional `max_age` that overrides the
freshness gate. The response carries a source tag on each result indicating
`memory`, `cache`, or `live`. Cache and memory are on by default; the params are
disable switches, and memory-first is enforced in code (the resolver), not by the
tool description.

Update the shared request/response shapes (`SearchRequest`/`SearchResponse`) and
both transports (REST handler and MCP tool) so they stay in lockstep — the file
already shares these shapes between REST and MCP. Keep backward-compatible
behavior for existing callers (omitted params default to on). Follow the existing
patterns in `server.go` for jsonschema-tagged fields and handler wiring.

## Goal

`search` accepts `use_cache`, `use_memory`, and `max_age`, defaults cache and
memory on, routes through the resolver, and returns a `memory|cache|live` source
tag on results, across both REST and MCP.

## How to Verify

- `SearchRequest` gains `use_cache`, `use_memory`, `max_age`; `SearchResponse`
  (or per-result shape) gains a source tag.
- Omitting the new params keeps cache and memory enabled; setting them false
  disables the respective path.
- A response identifies whether each result came from memory, cache, or live.
- Both `POST /api/search` and the `web_search` MCP tool expose the same behavior.
- Tests (using the resolver with fakes) assert defaults, disable switches, and
  source tagging on both transports.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `server.go`
- `server_test.go` [NEW]

## Dependencies

T032, T031.
