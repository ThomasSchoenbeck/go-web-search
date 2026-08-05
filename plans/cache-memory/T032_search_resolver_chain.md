---
feature: Resolver Chain
task_number: 032
description: Search resolver chain (memory -> search cache -> engines) + source tag
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 032: Search resolver chain

## Description

Implement the search path of the resolver (extend `resolver.go`): for a text
query, try memory first, then the search cache, then the live engines, and tag
the result with its source. The order enforces memory-first in code:

1. Memory — retrieve top-k facts (T027) and run the confidence gate (T028); if
   the gate passes, answer from memory (source tag `memory`) and skip everything
   downstream.
2. Search cache — try exact then semantic lookup (T019/T020); on a hit, answer
   from cache (source tag `cache`).
3. Engines — fall back to a live search via the harvester, store the result in
   the search cache (which enqueues an embed job), optionally enqueue distill on
   any scraped content, and tag the result `live`.

Each stage records which source answered so the tool/endpoint layer (T033) can
surface `memory|cache|live`. Follow go-agent conventions: context-first, small
interfaces, explicit errors.

## Goal

The search path resolves memory -> search cache -> engines in that order, answers
from the first that satisfies the request, and tags the result with its source.

## How to Verify

- `resolver.go` search path tries memory (with gating), then search cache (exact
  then semantic), then live engines, in order.
- A gate-passing memory hit short-circuits and is tagged `memory`; a cache hit is
  tagged `cache`; a fallthrough is tagged `live` and is stored back into the
  search cache.
- `resolver_test.go` covers memory-hit, cache-hit, and live-fallback with fakes,
  asserting the source tag and that later stages are not consulted after an
  earlier one satisfies.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `resolver.go`
- `resolver_test.go`

## Dependencies

T020, T028, T019.
