# Plan: Observability & Data-Inspection UI

> Created: 2026-08-05 | Last Updated: 2026-08-06

A web UI, served by the existing `-mode serve` listener, for inspecting
everything the harvester stores and does: runs, per-engine searches and their raw
SERPs, scrapes (raw / clean / text) and their images, the search and scrape
caches, distilled memory facts, the vector store, the background job queue, and
the logs — plus the provenance chains that connect them ("what caused what":
run → search → url → scrape → memory fact → vector).

This is a read/observe layer over the data model already defined in `schema.sql`
and `schema_logs.sql`, reusing the JSON endpoints already wired in `server.go`
wherever they exist and adding read-only endpoints only where none does. It adds
no new pipeline behavior; it makes existing state visible. The frontend is a
Svelte SPA built to static assets with Vite and embedded into the Go binary via
`go:embed`, served on the same listener as the current REST + MCP routes — single
binary, single service, no separate deployable.

## Decisions (finalized — interview complete)

- **Frontend:** Svelte 5 + Vite (client-only SPA, not SvelteKit), **written in
  TypeScript** → Vite `dist/` → `go:embed` into the binary, served on the existing
  serve listener alongside REST + MCP. The SPA consumes JSON.
- **Language — TypeScript throughout.** All frontend code is TypeScript: `.ts`
  modules and Svelte components with `<script lang="ts">`, a `tsconfig`, and
  `svelte-check`/`tsc --noEmit` type-checking wired into the build and CI. Test
  files are `.test.ts` / `.spec.ts`. No plain-JS source.
- **Tooling & supply chain:** package manager is **pnpm** (not npm). Pinned latest
  at planning time — `create-vite` 9.1.2, `vite` 8.2.0, `svelte` 5.56.8, plus
  pinned `typescript` and `svelte-check` — with **every dependency pinned to an
  exact version**, a committed `pnpm-lock.yaml`, and pinned pnpm + Node. `pnpm
  audit` must pass and versions must be checked for CVEs / supply-chain issues
  before pinning (re-confirm at implementation time).
- **Read-only v1.** Inspection only; no action-triggering, no destructive ops.
  Existing write endpoints (`/api/distill/preview`, `/api/vacuum`, etc.) are out
  of scope for the UI.
- **All views are in v1:** runs/searches/SERPs, scrapes, provenance, memory facts,
  semantic explorer (neighbor lists), jobs, caches, logs, stats. The embeddings
  2-D projection scatter is the one deliberately later phase within this plan.
- **Access model:** **edge deployment with no app-level auth.** The app serves the
  SPA shell and `/api/*` openly; authentication is delegated to the edge (reverse
  proxy / gateway / trusted network) in front of it. No login UI, no token entry.
  The existing `server.api_key` bearer stays supported for anyone exposing the
  binary directly, but the UI's assumed deployment leaves it unset and relies on
  the edge. This is not a hardened public-auth model.
- **Live updates:** polling for jobs and logs, with settings in `config.toml`
  (default interval per session; polling can default to off). The UI overrides for
  the moment: a dropdown changes the interval and a button toggles polling on/off.
  An SSE endpoint remains an optional later task, not v1.

## Endpoint reuse vs. new (grounding)

Views over data that already has a GET endpoint reuse it unchanged: `/api/runs`,
`/api/runs/{id}`, `/api/runs/{id}/urls|searches|scrapes`, `/api/searches/{id}/raw`,
`/api/memory/facts` + `/{id}`, `/api/stats`. **Exception found in T008:**
`/api/scrapes/{id}` was *not* reusable unchanged — `ScrapeDetail` omitted seven
cache-metadata fields the scrape view has to show (`content_hash`, `etag`,
`last_modified`, `tier`, `hit_count`, `expires_at`, `fetched_at`), so T008 added
them to the struct and the SELECT. Read-only and additive. New
read-only endpoints are needed for: provenance-for-a-URL, the jobs list, the
`search_cache` and `scrape_cache` browsers, the logs query (over the separate log
DB), the semantic-explorer embed+search, and the vector-projection dump. `/api/stats`
gains active model+dim and re-embed migration state. Each is flagged in its task.

## Testing standards (applies to every task)

Tests are part of the definition of done for each task, not a separate phase.
The shared harness is set up once in **T024** (Vitest + Playwright + isolated-DB
fixtures); every view and endpoint task then ships its own tests.

- **Frontend unit (Vitest):** every non-trivial function or component behavior
  gets a unit test. Files colocated as `*.test.ts`. `pnpm test`.
- **End-to-end (Playwright):** every page is loaded and **every interactive
  element — every button, link, and input — is exercised.** Specs live in
  `web/tests/e2e/*.spec.ts`. `pnpm test:e2e`.
- **Backend (Go):** each new endpoint keeps its `go test` coverage.
- **Isolated databases + full teardown (hard rule):** all tests — frontend e2e
  and backend Go — run against a **throwaway database created per run** (temp
  main DB + temp log DB), seeded with fixtures, and **every trace of test data is
  removed on completion**, pass or fail. No shared or production DB is ever
  touched. Shared fixtures for this live in T024.

## Features

### Feature: Foundation — Build, Embed & Serving

- [x] T001: Document the Observability UI (what it is, build step, embedded serving) in README — [T001_readme_observability_ui.md](T001_readme_observability_ui.md)
- [x] T002: Scaffold the Svelte + Vite SPA under `web/` — pnpm, pinned versions, `pnpm audit` gate, dev proxy — [T002_svelte_vite_scaffold.md](T002_svelte_vite_scaffold.md)
- [x] T003: `go:embed` the Vite `dist/` + static serving + SPA fallback routing (dev vs embedded) — [T003_goembed_dist_spa_serving.md](T003_goembed_dist_spa_serving.md)
- [x] T004: Access model — edge auth (no app auth) + expose non-secret UI config (poll defaults, projection cap) from `config.toml` — [T004_spa_auth_bearer.md](T004_spa_auth_bearer.md)
- [x] T005: Shared frontend API-read layer (typed same-origin fetch, loading/error, config-driven polling helper) — [T005_frontend_api_read_layer.md](T005_frontend_api_read_layer.md)
- [x] T024: Test harness — Vitest unit + Playwright e2e, isolated test DB + teardown fixtures — [T024_test_harness.md](T024_test_harness.md)

### Feature: Core Views — Runs, Searches & SERPs

- [x] T006: Runs list + run detail view (reuses `/api/runs`, `/api/runs/{id}`, `/urls`, `/searches`, `/scrapes`) — [T006_runs_list_detail_view.md](T006_runs_list_detail_view.md)
- [x] T007: Searches view + raw SERP HTML viewer (reuses `/api/runs/{id}/searches`, `/api/searches/{id}/raw`) — [T007_searches_serp_viewer.md](T007_searches_serp_viewer.md)

### Feature: Core Views — Scrapes

- [x] T008: Scrape detail view — raw/clean/text toggle + images + fetch metadata (reuses `/api/scrapes/{id}?raw=1`) — [T008_scrape_detail_view.md](T008_scrape_detail_view.md)

### Feature: Navigation Shell

- [x] T027: Navigation shell — persistent nav over every built view, active-route marking, one source of truth for routes — [T027_navigation_shell.md](T027_navigation_shell.md)

### Feature: Provenance / Causality

- [x] T009: NEW endpoint — provenance pivot for a URL (backward searches+rank, forward scrape→facts→vectors) — [T009_provenance_url_endpoint.md](T009_provenance_url_endpoint.md)
- [x] T010: Provenance view — pivot on a URL, render the backward/forward chain; link from facts (reverse) — [T010_provenance_view.md](T010_provenance_view.md)
- [x] T025: NEW endpoint — whole-run causality graph (searches→urls→scrapes→facts for a run) — [T025_run_causality_endpoint.md](T025_run_causality_endpoint.md)
- [x] T026: Run causality graph view (render the run-level chain) — [T026_run_causality_view.md](T026_run_causality_view.md)

### Feature: Memory & Semantic Explorer

- [x] T011: Memory facts browser view (reuses `/api/memory/facts` + `/{id}`) — [T011_memory_facts_browser.md](T011_memory_facts_browser.md)
- [x] T012: NEW endpoint — semantic explorer: embed query text, VectorSearch memory+search, return neighbors+distance — [T012_semantic_explorer_endpoint.md](T012_semantic_explorer_endpoint.md)
- [x] T013: Semantic explorer view — query box, top-k neighbor list with cosine distance, links to facts/searches — [T013_semantic_explorer_view.md](T013_semantic_explorer_view.md)

### Feature: Jobs, Caches, Logs & Stats

- [x] T014: NEW endpoint — jobs list with status/type filters + pagination over the `jobs` table — [T014_jobs_list_endpoint.md](T014_jobs_list_endpoint.md)
- [x] T015: Jobs queue monitor view (pending/running/failed, attempts, backoff; polling) — [T015_jobs_monitor_view.md](T015_jobs_monitor_view.md)
- [x] T016: NEW endpoints — `search_cache` + `scrape_cache` browsers (tier/expiry/hit_count, filters, pagination) — [T016_cache_browser_endpoints.md](T016_cache_browser_endpoints.md)
- [x] T017: Cache browser views (search_cache + scrape_cache) — [T017_cache_browser_views.md](T017_cache_browser_views.md)
- [x] T018: NEW endpoint — logs query over the separate log DB (run_id/level/source filters, pagination) + wire log-read into the server — [T018_logs_query_endpoint.md](T018_logs_query_endpoint.md)
- [x] T019: Logs viewer view (filters + polling tail) — [T019_logs_viewer_view.md](T019_logs_viewer_view.md)
- [x] T020: Extend `/api/stats` with model+dim + migration state; Stats dashboard view — [T020_stats_dashboard.md](T020_stats_dashboard.md)

### Feature: Embeddings 2-D Projection (later phase)

- [x] T021: NEW endpoint — vector projection data dump (memory + search owners) for scatter rendering — [T021_vector_projection_endpoint.md](T021_vector_projection_endpoint.md)
- [x] T022: 2-D projection scatter view (client-side PCA) — [T022_projection_scatter_view.md](T022_projection_scatter_view.md)

### Feature: Documentation & Verification

- [ ] T023: README refresh (final UI, build, embedded serving) + end-to-end verification — [T023_readme_refresh_verification.md](T023_readme_refresh_verification.md)

## Implementation Order & Phases

Dependencies are noted in parentheses. Within a phase, tasks may run in the listed
order; a later phase should not start until the tasks it depends on are `[x]`.

**Phase 1: Foundation — Build, Embed & Serving**

1. T001 — README: document the planned Observability UI, build step, embedded serving (no deps)
2. T002 — Scaffold the Svelte 5 + Vite SPA under `web/` with pnpm, exact-pinned + audited deps, dev proxy (no deps)
3. T003 — `go:embed` the Vite `dist/`, static serving + SPA fallback route; dev-vs-embedded serving (T002)
4. T004 — Access model: edge auth, no app auth; expose non-secret UI config (poll interval/enabled defaults, projection cap) via `/api/ui-config` (T003)
5. T005 — Shared frontend API-read layer: typed same-origin fetch, loading/error, config-driven polling helper (interval/enable override) (T004)
6. T024 — Test harness: Vitest + Playwright + isolated-DB fixtures with full teardown (T002, T003, T005)

**Phase 2: Core Views — Runs, Searches, SERPs, Scrapes**

6. T006 — Runs list + run detail view over the existing run endpoints (T005)
7. T007 — Searches view + raw SERP HTML viewer over the existing search endpoints (T006)
8. T008 — Scrape detail view: raw/clean/text toggle, images, fetch metadata over `/api/scrapes/{id}` (T005)

**Phase 3: Navigation & Provenance / Causality**

8a. T027 — Navigation shell: persistent nav over the built views, one shared route
definition. Placed here, before the remaining views, so each later view task
registers its entry as it lands rather than leaving views URL-only (T006)
9. T009 — NEW provenance-for-a-URL endpoint: backward searches+rank, forward scrape→facts→vectors, plus store queries (T005)
10. T010 — Provenance view: pivot on a URL, render the backward/forward chain; fact→source reverse links here (T009)
10a. T025 — NEW whole-run causality endpoint: assemble searches→urls→scrapes→facts for a run (T009)
10b. T026 — Run causality graph view: render the run-level chain (T025, T010)

**Phase 4: Memory & Semantic Explorer**

11. T011 — Memory facts browser view over `/api/memory/facts` + `/{id}` (T005)
12. T012 — NEW semantic-explorer endpoint: embed query text, VectorSearch memory+search owners, return neighbors with distance (T005)
13. T013 — Semantic explorer view: query box + top-k neighbor list, links to facts/searches (T012, T011)

**Phase 5: Jobs, Caches, Logs & Stats**

14. T014 — NEW jobs list endpoint: filters + pagination over `jobs` (T005)
15. T015 — Jobs queue monitor view with polling (T014)
16. T016 — NEW search_cache + scrape_cache browser endpoints: tier/expiry/hit_count, filters, pagination (T005)
17. T017 — Cache browser views for both caches (T016)
18. T018 — NEW logs query endpoint over the separate log DB + wire log-read access into the server (T005)
19. T019 — Logs viewer view: filters + polling tail (T018)
20. T020 — Extend `/api/stats` with model+dim + migration state, then build the Stats dashboard view (T005)

**Phase 6: Embeddings 2-D Projection (later)**

21. T021 — NEW vector projection data endpoint for memory + search owners (T012)
22. T022 — 2-D projection scatter view (client-side PCA) (T021, T013)

**Phase 7: Documentation & Verification**

23. T023 — README refresh + end-to-end verification of the embedded binary, all views, auth, and polling (all prior)

## Notes, Risks & Assumptions

- **Read-only, no new pipeline behavior.** Every task only reads existing tables
  or adds read-only JSON endpoints plus the embedded SPA. No task writes to the
  data model, triggers a scrape/search/distill, or touches the resolver. Existing
  write endpoints (`/api/distill/preview`, `/api/vacuum`, `/api/memory/store`) are
  explicitly out of scope for the UI in v1.
- **Access model — edge auth, no app-level auth (T004).** The app implements no UI
  authentication: the SPA shell and every `/api/*` route are served openly, and
  auth is delegated to an edge layer (reverse proxy / gateway / trusted network) in
  front of the binary. There is no login screen and no token handling in the SPA.
  `withAuth` in `server.go` and the `server.api_key` bearer remain available for
  anyone exposing the binary directly, but the assumed observability-UI deployment
  runs with `api_key` unset and the edge enforcing access. T004 is therefore a
  deployment/access task (document the model, ensure shell + `/api` are reachable
  in the no-key config, note how to re-enable `api_key` for direct exposure), not a
  login-UI task. This is explicitly not a hardened public-auth model.
- **Log DB read path is new (T018).** `LogStore` in `logstore.go` is write-only
  (a batching goroutine); there is no method to read logs back, and `apiServer`
  holds only the main `Store`, not the log DB handle. T018 adds a read query over
  the separate log database AND wires that read access into the server — flag this
  as touching serve-mode wiring (`main.go` / `newAPIServer`), not just `server.go`.
  **Landed as** a `logs *LogStore` parameter on `newAPIServer`/`serveMode`/
  `serveWithBrowser` plus both test entry points, and `GET /api/logs`.
- **The log database carries the test server's own lines (found in T018/T019).**
  `dbLogWriter` tees the artifacts logger into the log DB, so a `-mode testserve`
  process writes its startup lines there under *its own* run id, alongside the
  fixtures. This is the `runs`-count trap again: **never assert a total log-line
  count in e2e** — pivot on the seeded run id or on fixture message text.
- **Provenance — full scope (T009/T010 + T025/T026 + a link from T011).** Confirmed
  to cover all three shapes. (1) **URL pivot** (T009 endpoint, T010 view): backward
  via `search_urls` (search_id, url_id, rank) joined to `searches`; forward via
  `scrape_cache` (keyed on URL) → `memory_facts` by `source_url` → active vectors
  table. `ScrapeSizesByURL` already resolves a URL to its scrape; the rest are new
  read queries. (2) **Whole-run causality graph** (NEW T025 endpoint, T026 view):
  for a run, assemble searches → urls (with rank) → scrapes → distilled facts as a
  graph/tree. (3) **Reverse fact→sources**: from a memory fact, resolve its
  `source_url` and reuse the URL-pivot provenance — surfaced as a link from the
  memory facts browser (T011) and the explorer into the provenance view, so no
  separate endpoint is needed beyond T009. All read-only.
- **Semantic explorer is distinct from `/api/memory/query` (T012).** The existing
  `memory/query` runs confidence gating and synthesizes an answer over the
  `memory` owner only. The explorer is a raw nearest-neighbor tool: embed arbitrary
  text once, run `VectorSearch` over BOTH `memory` and `search` owner kinds, and
  return neighbors with cosine distance resolved to fact text / cached query — no
  gating, no synthesis. It reuses `LLMClient` embeddings and `Store.VectorSearch`.
- **VectorSearch is an exact linear scan (context for T012/T021).** `vectors.go`
  does `vector_distance_cos` over every row of a kind (the Rust Turso engine has no
  ANN index). Fine at local-research scale; the projection dump (T021) reads whole
  vectors, so keep result sets bounded/paginated.
- **2-D projection runs client-side with PCA (confirmed, T021/T022).** The T021
  endpoint returns bounded/sampled raw embedding vectors for the memory + search
  owner kinds and adds NO new Go dependency; the T022 view computes a **PCA**
  projection to 2-D in the browser (fast, deterministic, minimal/no JS dep — not
  UMAP/t-SNE). The sample cap is exposed in `config.toml` rather than hardcoded.
  **Landed with no dependency change at all** — neither `go.mod` nor
  `package.json` moved. PCA is a local helper (`web/src/lib/projection.ts`) doing
  power iteration without ever forming the d×d covariance matrix, which at real
  embedding dimensions would be larger than the data it summarises.
  Responsiveness is a progress state rather than a worker; the config cap already
  bounds the work, and a worker would not run under jsdom.
- **`vector_extract()` IS available on the Rust Turso engine (found in T021).**
  vectors.go lists what the engine lacks (`libsql_vector_idx`, `vector_top_k`)
  but was silent on extraction. Probed against the real engine:
  `vector_extract(embedding)` returns the same `'[a,b,c]'` text `vector32()`
  parses, which is how T021 reads embeddings back out of an `F32_BLOB` column.
  `parseVectorLiteral` in `vectors.go` is its inverse. Decoding the blob bytes
  directly also works (little-endian float32) but assumes a layout the engine
  never documented — don't.
- **Vectors live in a generation table (T012/T021).** The active table name comes
  from `system_meta` (`metaVectorTable`); during a re-embed migration semantic
  reads are unavailable. The explorer and projection must degrade gracefully
  (empty/"migration in progress") rather than error, mirroring `Stats`.
- **Stats — full dashboard (T020).** Confirmed to include everything: (a) active
  embedding model+dim and re-embed migration state (`system_meta`); (b) cache hit
  rates and tier (short/long/permanent) distributions for search + scrape caches;
  (c) job throughput/timings from the `jobs` table. `StatsView` already counts
  runs/searches/urls/scrapes/facts/search_cache/vectors/pending-jobs, so (a) is a
  minor add. **RISK — hit *rate* instrumentation:** the caches store `hit_count`
  per row but not total lookups (hits + misses), so a true hit-rate may not be
  derivable from current data. T020 must either add lightweight counters or, if
  that crosses into non-read-only territory, present hit *counts* and tier
  distributions instead of a computed rate — resolve this in the task before
  building. Job timings derive from `created_at`/`updated_at`/`locked_at`/`attempts`.
  **RESOLVED in T020: option (a), counts not rates.** A miss leaves nothing
  behind to count, and adding lookup counters would be a write on every cache
  read — outside read-only v1. `/api/stats` carries rows, tier distribution,
  expired, rows-reused and total-hits per cache, and the dashboard states why
  there is no rate rather than leaving the absence unexplained. Job timing is
  averaged over the most recent `observability.job_timing_sample` finished jobs
  (default 200; 0 disables), because the whole history is an unbounded scan.
- **Dev vs. embedded serving (T002/T003).** In development the SPA runs under the
  Vite dev server proxied to the serve listener; in production the built `dist/` is
  embedded via `go:embed` and served directly, with an SPA fallback so client-side
  routes resolve to `index.html`. The `web/` directory and the Node/Vite toolchain
  are new build-time dependencies — document them in the README (T001) and the
  build steps (T023). **Frontend flavor (confirmed): Svelte 5 + Vite, client-only
  SPA — NOT SvelteKit** (T002 pins this).
- **Client routing — History API + server fallback (T003/T010 etc.).** Svelte 5
  ships no router, so this is a choice, not a default: the SPA uses History-API
  clean URLs (e.g. `/runs/123`) with a minimal client router, and the Go embed
  handler serves `index.html` for unknown non-`/api`/`/mcp`/`/healthz` paths (the
  SPA fallback already in T003). Hash routing was the simpler alternative but is
  not used.
- **Build orchestration via Taskfile (confirmed).** The repo's `Taskfile.yaml`
  gains targets that run `pnpm build` (in `web/`) BEFORE `go build`, so one command
  produces the embedded binary in the right order. T002/T003 reference it and T023
  verifies building the documented way works; the ordering constraint (dist must
  exist before `go build`) is unchanged.
- **Build ordering: `dist/` is gitignored and generated (confirmed).** `web/dist/`
  is NOT committed; it is produced by the Node/Vite build step (`pnpm build`) in
  CI/local builds BEFORE `go build`. Because `go:embed` needs `dist/` present at
  compile time, a plain `go build` with no prior `pnpm build` embeds nothing (or
  fails on a missing dir). T003 keeps the package compiling in a clean checkout
  (placeholder/empty-tolerant embed) while embedding the real assets when built.
  This ordering is documented in T001 (README), enforced in T002/T003 (scaffold +
  embed), and verified in T023.
- **Polling, config-driven with UI overrides (T005/T015/T019).** Jobs (T015) and
  logs (T019) refresh by polling from the shared API layer (T005). `config.toml`
  holds the per-session defaults: the poll interval and whether polling is enabled
  by default (it may default to off). The UI overrides for the moment — a dropdown
  changes the interval and a button toggles polling on/off — without persisting
  back to config. An SSE/stream endpoint is explicitly deferred, not a v1 task.
- **Externalized config — all app settings in `config.toml`, UI overrides live.**
  Every new setting (poll interval + enabled default, projection sample cap, any
  other UI tunable) lives in `config.toml`, exposed to the SPA (served/injected at
  runtime). No settings are hardcoded and none are baked at build time. The UI
  changes values only for the current moment via direct controls (a dropdown for
  the interval, a toggle button for polling); it does not write back to config.
  With edge auth there is no bearer or token entry to store.
- **Shared frontend machinery landed in Phase 2 (no task owned it).** Three
  modules the later view tasks all depend on were added while building T006–T008,
  because the plan assigned them to no task: `web/src/lib/router.ts` (the
  History-API router the routing decision implies — Svelte ships none),
  `web/src/lib/format.ts` (duration/timestamp/truncate display helpers), and
  `web/src/lib/apiStub.ts` (test-only fetch stub keyed by path *and* query).
  **Every later view task must register its route** — in the match chain in
  `web/src/App.svelte` until T027 lands, in the shared route definition after —
  add its navigation entry (T027) and its `data-testid` hooks. No view task
  hand-rolls fetching, formatting or routing. **Phase 5 added a fourth shared
  piece:** `web/src/components/PollControls.svelte` (T015), which owns the
  poller for both live views — seeded from `/api/ui-config`, with the interval
  dropdown, the on/off toggle and stop-on-unmount in one place.
- **The e2e seed is a shared, growing fixture.** `seedTestData` in
  `testsupport.go` (T024) writes the fixture dataset every Playwright run reads,
  and `web/tests/e2e/fixtures-data.ts` exports the fixed ids. Tasks that add a
  view over data the seed does not yet contain must extend it there rather than
  seeding per-spec. The log database was a known gap here until T018 added
  `seedTestLogs` — a **second** seeding function, because the log DB is a
  separate file with its own handle. Phase 5 also grew the main seed: jobs
  covering every status, and a long-tier row in each cache.
- **Tone/convention parity.** Task files follow the `plans/cache-memory/`
  conventions exactly: the frontmatter block, the Description/Goal/How to
  Verify/Files to Touch/Dependencies sections, `[NEW]` markers on new files,
  `go fmt`/`go vet`/`go test` in verification for backend tasks, and a frontend
  build/lint check for SPA tasks. No code snippets.

## Resolved decisions (confirmed before task files were written)

1. **Build/dist — CONFIRMED.** `web/dist/` is gitignored and generated by a
   Node/Vite build step (`pnpm build`) in CI/local builds BEFORE `go build`;
   `dist/` is NOT committed. `go:embed` therefore needs the built `dist/` present at
   compile time, and a bare `go build` without a prior `pnpm build` embeds an
   empty dir / fails. Baked into T002 (scaffold + `.gitignore`), T003 (`go:embed` +
   compile-in-clean-checkout placeholder), and T023 (documented build ordering +
   verification).
2. **Svelte flavor — CONFIRMED.** Svelte 5 + Vite, client-only SPA (NOT SvelteKit).
   Pinned in T002.
3. **Projection — CONFIRMED client-side PCA.** The T021 endpoint returns bounded
   raw vectors (memory + search owners) with NO new Go dependency; the T022 view
   computes a **PCA** projection to 2-D in the browser (not UMAP/t-SNE). The sample
   cap is exposed in `config.toml`.
4. **Tooling & supply chain — CONFIRMED.** pnpm (not npm) throughout; pinned
   `create-vite` 9.1.2, `vite` 8.2.0, `svelte` 5.56.8; every dependency pinned to
   an exact version with a committed `pnpm-lock.yaml`; pinned pnpm + Node;
   `pnpm audit` and a CVE / supply-chain check are a gate. Baked into T002 and the
   Testing standards section. Versions are the latest at planning time and must be
   re-confirmed at implementation time.
5. **Testing — CONFIRMED.** Vitest unit tests for major frontend functions and
   Playwright tests covering every page and every button/link/input, plus Go tests
   for each endpoint. All tests run against an isolated throwaway database (main +
   log) and remove all test data on teardown. Harness in T024; per-task tests are
   in each task's How-to-Verify.
6. **Access model — CONFIRMED edge auth, no app auth.** No login UI, no token
   handling; the SPA + `/api` are served openly and auth is delegated to an edge/
   trusted network. `api_key`/`withAuth` stay available for direct exposure but the
   UI deployment assumes they are unset. Reframes T004 as a deployment/access task.
7. **Polling — CONFIRMED config default + live UI override.** `config.toml` sets the
   default interval and whether polling is on by default (may be off); the UI has a
   dropdown to change the interval and a button to toggle polling, for the moment
   only (no write-back). Applies to T005/T015/T019.
8. **Provenance — CONFIRMED full scope.** URL pivot (T009/T010), whole-run causality
   graph (NEW T025/T026), and fact→source reverse (link from T011/T010 reusing T009).
9. **Stats — CONFIRMED everything.** Model+dim+migration, cache hit rates + tier
   distributions, and job throughput/timings (T020). Open risk: hit-*rate* may need
   new counters — T020 resolves rate-vs-count while staying read-only.
10. **Routing & build — CONFIRMED.** History-API clean URLs + Go SPA fallback (no
    hash routing); `Taskfile.yaml` gets targets chaining `pnpm build` → `go build`.
11. **TypeScript — CONFIRMED.** The whole frontend is TypeScript: `.ts` modules,
    Svelte components with `<script lang="ts">`, `.test.ts`/`.spec.ts` tests, a
    `tsconfig`, pinned `typescript` + `svelte-check`, and type-checking in the
    build/CI. T002 scaffolds with the Svelte-TS template; all view/lib/test tasks
    are TypeScript.
