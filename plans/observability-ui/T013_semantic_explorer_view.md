---
feature: Memory & Semantic Explorer
task_number: 013
description: Semantic explorer view — query box, top-k neighbor list with cosine distance, links to facts/searches
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 013: Semantic explorer view

## Description

Build the semantic explorer view over the T012 endpoint. It is a raw
nearest-neighbor probe, not a Q&A box:

- A **query box** where the user types arbitrary text and an optional `k`.
- A **top-k neighbor list** returned by the endpoint, each neighbor tagged by
  owner kind (memory fact vs. cached search query), showing cosine distance (and
  optionally similarity = 1 − distance), sorted nearest first.
- **Links out:** memory neighbors link to the fact detail (T011); search
  neighbors link to their cached query context. This makes the explorer a jumping
  point into the rest of the UI.

Handle the degraded states the endpoint reports (no vectors yet / migration in
progress) with a clear message rather than an empty error. Uses the shared API
layer (T005).

## Goal

A semantic explorer view lets the user embed-and-search arbitrary text, renders
the top-k neighbors with cosine distance tagged by owner kind, and links memory
neighbors to their facts, degrading gracefully when the vector store is
unavailable.

## How to Verify

- Typing a query and submitting shows top-k neighbors from the T012 endpoint with
  distance and owner-kind tags, nearest first; `k` is honored.
- Memory neighbors link to the fact detail (T011); search neighbors show their
  cached query context.
- When the endpoint reports no vectors / migration in progress, the view shows a
  clear message, not an error.
- Loading/error from the shared layer (no bearer — edge auth).
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/SemanticExplorer.svelte` [NEW]
- `web/src/App.svelte`

## Dependencies

T012, T011.
