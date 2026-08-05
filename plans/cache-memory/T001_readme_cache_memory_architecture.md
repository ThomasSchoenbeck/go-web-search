---
feature: Foundation & Documentation
task_number: 001
description: Document planned cache/memory architecture in README
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 001: Document planned cache/memory architecture in README

## Description

Add an early, forward-looking section to `README.md` describing the cache and
memory system this plan introduces, before any of it is built. The current
README already carries a "Known limits" note that re-scraping inserts a new row
with no cache check; this task frames the coming work so the README is honest
about the direction from the start (the planner's "documentation is mandatory,
placed early" rule).

The section should describe, in plain terms: the three stores (search cache,
scrape cache, semantic memory of atomic facts), the single DB-backed crash-safe
job system that does background work, the fact that all state stays in Turso,
and the two named behavior changes coming (re-scrape becomes a cache hit path,
and `max_open_conns=1` will be raised). It should also note the two orthogonal
axes kept separate: tier (durability) and volatility (staleness rate).

This is a documentation-only task. It does not describe code that exists yet;
it sets expectations. T037 refreshes the README to match final behavior.

## Goal

`README.md` contains a new section (for example "Caching & Memory (planned)")
that a new reader can use to understand what the project is growing into,
including the three stores, the job system, the Turso-only state, and the two
upcoming behavior changes.

## How to Verify

- `README.md` renders with a clearly labeled new section covering: the three
  stores and what each keys on; the unified job system; Turso as the only state
  store; the tier-vs-volatility distinction.
- The section explicitly mentions the two behavior changes (re-scrape cache
  hit; raising `max_open_conns`).
- No source files change; `go build ./...` still succeeds unchanged.

## Files to Touch

- `README.md`

## Dependencies

None.
