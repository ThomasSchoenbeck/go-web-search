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

## Note added during implementation

- **`web/package.json` needed no change.** The task allowed either a local helper
  or a pinned library; PCA is short enough that the helper won, which also means
  no new package to audit. `web/pnpm-lock.yaml` is untouched.
- The helper does power iteration **without forming the covariance matrix**: for
  embeddings the dimension dwarfs the point count, so a d×d covariance (4096² at
  the real model size) would be far larger than the data it summarises. The
  second component is found by orthogonalising against the first each iteration
  rather than deflating the data, which would cost another copy of it.
- **Responsiveness is a progress state, not a worker** — the task offered either.
  A worker would add Vite worker plumbing and would not run in jsdom, for a
  computation the config cap already bounds. The view paints "Projecting N
  vectors…" and yields before computing.
- The sample cap comes from `/api/ui-config` and the view requests exactly it, so
  `projection_sample_cap` is the one place that decides how much is pulled. If
  the config cannot be read, the view says so rather than guessing a cap.
- **The seeded fixtures are the degenerate case:** two vectors are rank 1 after
  centring, so there is no second axis to spread along. Both the helper's unit
  tests and the e2e specs assert the layout stays finite there — a NaN
  coordinate would silently drop points off the canvas rather than fail loudly.
- The view states that distances are indicative: two dimensions cannot carry
  what the full space knows, and a scatter invites over-reading.

## Files to Touch

- `web/src/views/ProjectionScatter.svelte` [NEW]
- `web/src/lib/projection.ts` [NEW]
- `web/src/App.svelte`
- `web/src/lib/api.ts`, `web/src/lib/routes.ts` — the resource and the route/nav entry

## Dependencies

T021, T013.
