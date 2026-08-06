---
feature: Navigation Shell
task_number: 027
description: Persistent navigation over every built view, with the active route marked
created: 2026-08-06
last_updated: 2026-08-06
completed:
status: [ ]
blocker_note:
---

# [ ] Task 027: Navigation shell

## Description

After Phase 2 the app shell carries a single link back to the runs list. Every
view added from Phase 3 onwards — provenance, memory facts, the semantic
explorer, jobs, both cache browsers, logs, stats, the projection scatter — is
reachable only by typing its URL. This task adds the navigation that makes the
UI usable as one application rather than a set of deep links, and it lands early
in Phase 3 so each later view task plugs into it as that view is built.

Build a persistent navigation region in the app shell that lists every view
currently built, marks which one is active, and stays correct as the user
navigates. It is presentation only: no new endpoints, no data fetching, no
change to what any view shows.

Two constraints matter more than the layout:

- **One source of truth for routes.** The route patterns the shell matches on and
  the entries the navigation renders must come from the same place, so a route
  can never exist without a navigation entry or point somewhere different from
  the link that reaches it. Today the match chain in `App.svelte` is the only
  list; this task gives the routes a home both it and the navigation read from.
- **Navigation entries stay real links.** The router established in T006 works by
  intercepting same-origin anchor clicks, which is what keeps modified-click,
  open-in-new-tab and keyboard activation working. The navigation must not
  replace anchors with click handlers.

The active entry must be identifiable both visually and to assistive technology,
and must update on in-app navigation and on browser back/forward alike. Views
not yet built simply have no entry: each later view task adds its own when it
lands, which is why this task comes before them rather than after.

## Goal

A persistent navigation region in the app shell lists every built view, marks
the active route for both sighted and assistive users, keeps working under
browser history navigation, and is the single place a new view registers itself
alongside its route.

## How to Verify

- Every view built so far is reachable from the navigation on every page,
  without typing a URL.
- The active entry is marked visually and exposed to assistive technology, and
  the marking follows in-app navigation, browser back and browser forward.
- Navigation entries are plain links: a modified click still opens a new tab,
  and the router handles the unmodified click without a full page load.
- Routes and navigation entries resolve from one shared definition — adding a
  route without an entry, or an entry pointing at an unmatched route, is
  detectable rather than silent.
- Deep-linking directly to any view still resolves through the server-side SPA
  fallback with the navigation rendered and the correct entry active.
- The frontend builds and lints clean (`pnpm build`, `pnpm check`).
- Unit tests (Vitest) cover the active-route logic, including the trailing-slash
  and nested-route cases; `pnpm test` passes.
- Playwright exercises every navigation link from more than one starting page,
  plus back/forward; `pnpm test:e2e` passes against an isolated throwaway test
  database that is seeded and fully torn down, leaving no residual test data.

## Files to Touch

- `web/src/lib/routes.ts` [NEW]
- `web/src/components/Nav.svelte` [NEW]
- `web/src/App.svelte`
- `web/src/lib/router.ts`

## Dependencies

T006.
