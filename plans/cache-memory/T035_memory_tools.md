---
feature: MCP/REST Surface
task_number: 035
description: memory.query + memory.store tools/endpoints
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 035: `memory.query` + `memory.store` tools/endpoints

## Description

Expose memory directly through the MCP/REST surface in `server.go`, mirroring the
existing tool/endpoint pattern: `memory.query` retrieves relevant facts for a
query (T027) and returns them with scores and provenance (optionally applying the
confidence gate, T028); `memory.store` stores a fact (or facts) via the semantic
upsert path (T026), which enqueues embedding and applies tiering. Add both as MCP
tools and as REST endpoints, sharing request/response shapes as the file already
does for search and scrape.

`memory.store` is also where an explicit "remember this" from a caller enters;
the tier flag it carries is handled in T036. Keep the tools thin — the real logic
lives in the memory package and resolver, per the memory-first-in-code principle.
Follow the existing `server.go` conventions for jsonschema-tagged fields, handler
registration, and MCP tool descriptions.

## Goal

`memory.query` and `memory.store` are available over both MCP and REST, backed by
retrieval (T027), gating (T028), and semantic upsert (T026).

## How to Verify

- New MCP tools `memory.query` and `memory.store` are registered; matching REST
  endpoints exist and share request/response shapes.
- `memory.query` returns relevant facts with scores/provenance; `memory.store`
  persists a fact via the T026 upsert and enqueues its embed job.
- Tests exercise both tools/endpoints against fakes or SQLite, asserting query
  results and that store performs an upsert.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `server.go`
- `server_test.go`

## Dependencies

T027, T028, T025.
