---
feature: Embeddings 2-D Projection (later phase)
task_number: 022
description: 2-D projection scatter view (client-side PCA)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 022: 2-D projection scatter view

## Description

Build the embeddings scatter view over the T021 vector dump. Per the confirmed
decision, **the 2-D layout is computed in the browser using PCA** — the view
fetches the bounded raw vectors from T021 and runs a PCA reduction to 2-D in TypeScript
(PCA is simple linear algebra: a small local helper or a minimal, pinned + audited
client-side library — NOT a Go dependency, and not UMAP/t-SNE), then renders the
result as a scatter plot. PCA is fast and deterministic, so the same vectors
always map to the same layout.

- Points are colored/marked by owner kind (memory fact vs. cached search query).
- Hovering/selecting a point shows its label and links out: memory points to the
  fact detail (T011) and the semantic explorer (T013); search points to their
  cached query context.
- Respect the endpoint's bound/sample cap; show a clear message when the vector
  store reports unavailable / migration in progress.

Because layout runs client-side over potentially many high-dimensional vectors,
keep the computation off the main render path where practical (e.g. a worker or a
progress state) so the UI stays responsive. Uses the shared API layer (T005).

## Goal

A scatter view fetches the bounded vector dump, computes a 2-D projection in the
browser, and renders an interactive plot with points tagged by owner kind and
links to the fact / explorer views, degrading gracefully when vectors are
unavailable.

## How to Verify

- The view fetches the T021 dump and renders a 2-D scatter computed client-side
  via PCA (any projection helper/library is a pinned `web/` dependency; `go.mod`
  is unchanged).
- Points are distinguishable by owner kind; hovering/selecting shows the label and
  links to the fact detail (T011) / explorer (T013).
- The sample cap is respected; a migration-in-progress / no-vectors response shows
  a clear message, not an error.
- The UI stays responsive during layout (worker/progress state).
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/ProjectionScatter.svelte` [NEW]
- `web/src/lib/projection.ts` [NEW]
- `web/package.json`
- `web/src/App.svelte`

## Dependencies

T021, T013.
