---
feature: Provenance / Causality
task_number: 010
description: Provenance view — pivot on a URL, render the backward/forward chain
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 010: Provenance view

## Description

Build the provenance view over the new endpoint from T009. The user enters or
navigates to a URL (from a run's URLs, a scrape, or a fact) and the view renders
the full causality chain around it:

- **Backward:** the searches that found this URL, each with its term, engine, run,
  and rank — linking back to the run detail (T006) and searches view (T007).
- **Forward:** the scrape of this URL (link to the scrape detail, T008), the
  memory facts distilled from it (link to the facts browser / fact detail, T011),
  and whether each fact has a vector.

Present it as a readable chain/graph (run → search → url → scrape → fact →
vector), not a raw JSON dump, so a user can trace "what caused what" at a glance.
This view is also the **reverse fact→sources** destination (a fact in T011 links
here via its `source_url`) and offers a link onward to the **whole-run causality
graph** (T026) for the URL's run. Handle the degraded cases the endpoint reports
(no scrape cached, no facts, vector store mid-migration). Uses the shared API
layer (T005).

## Goal

A provenance view pivots on a URL and renders its backward (searches+rank+run) and
forward (scrape→facts→vector) chain as linked, navigable elements into the run,
search, scrape, and fact views.

## How to Verify

- Entering/navigating to a known URL renders its finding searches with rank and
  its forward scrape→facts→vector chain from the T009 endpoint.
- Links resolve to the run (T006), search (T007), scrape (T008), and fact (T011)
  views.
- Degraded cases (no cached scrape, no facts, migration in flight) render clear
  states, not errors.
- Loading/error from the shared layer (no bearer — edge auth).
- The frontend builds and lints clean (`pnpm build`).
- Unit tests (Vitest) cover this view's major functions; `pnpm test` passes.
- Playwright tests exercise the page and every interactive element — every button, link, and input; `pnpm test:e2e` passes against an isolated throwaway test database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/views/Provenance.svelte` [NEW]
- `web/src/App.svelte`

## Dependencies

T009.
