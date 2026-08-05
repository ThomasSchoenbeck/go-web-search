---
feature: Provenance / Causality
task_number: 026
description: Run causality graph view (render the run-level searches→urls→scrapes→facts chain)
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 026: Run causality graph view

## Description

Build the whole-run causality view over the T025 endpoint: for a chosen run,
render the full chain — searches → urls (with rank) → scrapes → distilled facts —
as a graph or tree so the "what caused what" is legible at the run level. This is
the run-level counterpart to the single-URL provenance view (T010).

- Render the T025 nodes/edges (or nested tree) as an interactive graph/tree:
  searches at the top, the urls they found (annotated with rank), each url's scrape,
  and the facts distilled from it, with per-fact vector presence marked.
- Cross-link into the rest of the UI: a url node links to the URL provenance view
  (T010) and its scrape detail (T008); a fact node links to the fact detail (T011);
  a search node links to the searches/SERP view (T007). Entry points: from the run
  detail view (T006) and from the URL provenance view (T010).
- Respect the endpoint's bound/summary for large runs and show a clear message when
  a run is empty or the vector store reports unavailable.

Keep the graph rendering dependency minimal and, if any library is added, pinned +
audited per the tooling rules. Uses the shared API layer (T005).

## Goal

A run causality view fetches the T025 graph and renders the run's
searches→urls+rank→scrapes→facts chain interactively, with nodes cross-linking to
the URL-provenance, scrape, fact, and search views, degrading gracefully for empty
runs or unavailable vectors.

## How to Verify

- The view fetches T025 for a run and renders the connected searches→urls→scrapes→
  facts structure with rank annotations and per-fact vector markers.
- Nodes link correctly: url → provenance (T010) + scrape (T008); fact → fact detail
  (T011); search → SERP view (T007). It is reachable from run detail (T006) and the
  URL provenance view (T010).
- A large run stays usable (respects the endpoint bound/summary); an empty run or a
  migration-in-progress/no-vectors response shows a clear message, not an error.
- Loading/error come from the shared layer (no bearer — edge auth).
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/RunCausality.svelte` [NEW]
- `web/src/lib/graph.js` [NEW]
- `web/src/App.svelte`

## Dependencies

T025, T010.
