# Plan: Search/Scrape Caching + Semantic Memory

> Created: 2026-07-30 | Last Updated: 2026-07-30

Adds a multilayer cache (search cache, scrape cache) plus a vectored semantic
memory of atomic facts to the existing flat `package main` harvester, so the app
minimizes web searches and scrapes. All state lives in Turso. Background work
runs on a single DB-backed, crash-safe job system. Single instance, single
binary, single service.

This plan extends the real code: `schema.sql`, `config.go`/`config.toml`,
`store.go`, `server.go` (REST + MCP), `scraper.go`, `harvest.go`, and adds new
`package main` files. It changes today's behavior in three named places —
re-scrape currently inserts a new row with no cache check (`scraper.one` in
`scraper.go`); `max_open_conns=1` serialises DB writes as a beta-Turso safeguard
while the job system needs real concurrency; and the one-shot `-mode search`/
`-mode scrape` test modes are removed, leaving `-mode browse` and `-mode serve`.

## Features

### Feature: Foundation & Documentation

- [x] T001: Document planned cache/memory architecture in README — [T001_readme_cache_memory_architecture.md](T001_readme_cache_memory_architecture.md)
- [x] T002: Revisit database concurrency (raise `max_open_conns`) for the job system — [T002_revisit_db_concurrency.md](T002_revisit_db_concurrency.md)
- [x] T003: Add `system_meta` key/value table (active embed model+dim, migration state) — [T003_system_meta_table.md](T003_system_meta_table.md)
- [x] T038: Remove one-shot `-mode search`/`-mode scrape` (keep browse + serve) — [T038_remove_oneshot_modes.md](T038_remove_oneshot_modes.md)
- [x] T037: README refresh + end-to-end integration tests — [T037_readme_refresh_integration_tests.md](T037_readme_refresh_integration_tests.md)

### Feature: LLM Provider & Config

- [x] T004: Config schema for named model endpoints keyed by role (chat|embed) — [T004_llm_endpoints_config.md](T004_llm_endpoints_config.md)
- [x] T005: OpenAI-compatible HTTP client (chat completions + embeddings) — [T005_openai_compatible_client.md](T005_openai_compatible_client.md)
- [x] T006: Config for cache/memory/tiering tunables (TTLs, thresholds, gates, remember default) — [T006_cache_memory_tunables_config.md](T006_cache_memory_tunables_config.md)

### Feature: Unified Job System

- [x] T008: Add `jobs` table to schema — [T008_jobs_table.md](T008_jobs_table.md)
- [x] T009: Job store CRUD (enqueue, claim, complete, fail+backoff, reset stale) — [T009_job_store_crud.md](T009_job_store_crud.md)
- [x] T010: Job runner — poller goroutine + worker pool + typed handler registry — [T010_job_runner.md](T010_job_runner.md)
- [x] T011: Reaper — startup + periodic reset of stale `running` rows — [T011_job_reaper.md](T011_job_reaper.md)
- [x] T012: Recurring-job registration as managed goroutines — [T012_recurring_job_registration.md](T012_recurring_job_registration.md)
- [x] T013: Wire job system into serve-mode lifecycle (start/stop) — [T013_wire_job_system_serve.md](T013_wire_job_system_serve.md)

### Feature: Vector Store, Embeddings & Migration

- [x] T014: Dedicated vectors table + ANN index (Turso native vector) — [T014_vectors_table_ann_index.md](T014_vectors_table_ann_index.md)
- [x] T015: Embed job type + worker (asymmetric prefixes, stamp model+dim) — [T015_embed_job_worker.md](T015_embed_job_worker.md)
- [x] T016: Vector search helper (semantic; degrades to exact/miss only during re-embed migration) — [T016_vector_search_fallback.md](T016_vector_search_fallback.md)
- [x] T017: Re-embed migration (blue/green) + boot-time model/dim check — [T017_reembed_migration.md](T017_reembed_migration.md)

### Feature: Search Cache

- [x] T018: Add `search_cache` table — [T018_search_cache_table.md](T018_search_cache_table.md)
- [x] T019: Query normalization + exact store/lookup + enqueue embed job — [T019_search_cache_store_lookup.md](T019_search_cache_store_lookup.md)
- [x] T020: Semantic search-cache lookup via vector similarity — [T020_search_cache_semantic_lookup.md](T020_search_cache_semantic_lookup.md)

### Feature: Scrape Cache & Content Hashing

- [x] T021: Scrape cache schema (content_hash, etag, last_modified, tier, expiry) + cache-check-before-insert — [T021_scrape_cache_schema.md](T021_scrape_cache_schema.md)
- [x] T022: Conditional refresh (ETag/Last-Modified/304) + cross-URL content dedupe — [T022_scrape_conditional_refresh.md](T022_scrape_conditional_refresh.md)
- [x] T023: Scrape resolver wiring in `scraper.one` (cache -> chromedp) + provenance — [T023_scrape_resolver_wiring.md](T023_scrape_resolver_wiring.md)

### Feature: Memory, Distillation & Confidence Gating

- [x] T024: Add `memory_facts` table — [T024_memory_facts_table.md](T024_memory_facts_table.md)
- [x] T025: Distill job type + worker (multiple atomic facts, volatility + validity horizon) — [T025_distill_job_worker.md](T025_distill_job_worker.md)
- [x] T026: Semantic upsert on store (update near-identical fact vs insert) — [T026_memory_semantic_upsert.md](T026_memory_semantic_upsert.md)
- [x] T027: Memory retrieval (top-k relevant facts) — [T027_memory_retrieval.md](T027_memory_retrieval.md)
- [x] T028: Confidence gating (similarity, freshness, LLM adjudication) — [T028_memory_confidence_gating.md](T028_memory_confidence_gating.md)

### Feature: Tiering & Cleanup

- [x] T007: Tier/expiry calculation helper (pure sliding-expiry + promotion) — [T007_tier_expiry_helper.md](T007_tier_expiry_helper.md)
- [x] T029: Apply sliding expiry + tier promotion on hits across all three stores — [T029_tier_sliding_expiry_hits.md](T029_tier_sliding_expiry_hits.md)
- [x] T030: Per-store cleanup jobs (delete expired) + cascade vector deletion — [T030_cleanup_jobs.md](T030_cleanup_jobs.md)

### Feature: Resolver Chain

- [x] T031: Dispatch — URL present -> scrape path; text only -> search path — [T031_resolver_dispatch.md](T031_resolver_dispatch.md)
- [x] T032: Search resolver chain (memory -> search cache -> engines) + source tag — [T032_search_resolver_chain.md](T032_search_resolver_chain.md)

### Feature: MCP/REST Surface

- [x] T033: `search` tool/endpoint params (use_cache, use_memory, max_age) + source tag — [T033_search_tool_params.md](T033_search_tool_params.md)
- [x] T034: `scrape` tool/endpoint (use_cache) + provenance — [T034_scrape_tool_provenance.md](T034_scrape_tool_provenance.md)
- [x] T035: `memory.query` + `memory.store` tools/endpoints — [T035_memory_tools.md](T035_memory_tools.md)
- [x] T036: "Remember this" flag (enum short|long|permanent, default on) — [T036_remember_this_flag.md](T036_remember_this_flag.md)

## Implementation Order & Phases

Dependencies are noted in parentheses. Within a phase, tasks may run in the
listed order; a later phase should not start until the tasks it depends on are
`[x]`.

**Phase 1: Foundation & Config**

1. T001 — README: document the planned cache/memory architecture (no deps)
2. T038 — Remove one-shot `-mode search`/`-mode scrape`; keep browse + serve (no deps)
3. T002 — Revisit `max_open_conns` so the job system gets real concurrency (no deps)
4. T003 — Add `system_meta` table for active embed model+dim and migration state (no deps)
5. T004 — Config schema for role-keyed model endpoints, config.toml only (no deps)
6. T005 — OpenAI-compatible HTTP client for chat + embeddings (T004)
7. T006 — Config for cache/memory/tiering tunables (no deps)
8. T007 — Pure tier/expiry helper: sliding expiry, ceilings, promotion (T006)

**Phase 2: Unified Job System**

8. T008 — `jobs` table (extends schema.sql) (no deps)
9. T009 — Job store CRUD: enqueue/claim/complete/fail-with-backoff/reset-stale (T008)
10. T010 — Job runner: poller + worker pool + one-place typed handler registry (T009)
11. T011 — Reaper: startup + periodic stale-`running` reset, the crash-recovery path (T009)
12. T012 — Recurring-job registration as managed goroutines, not `init()` (T010)
13. T013 — Wire the job system into serve-mode start/stop lifecycle (T010, T011, T012, T002)

**Phase 3: Vector Store, Embeddings & Migration**

14. T014 — Vectors table + ANN index, Turso native vector column (T003)
15. T015 — Embed job type + worker; asymmetric doc/query prefixes; stamp model+dim (T014, T010, T005)
16. T016 — Vector search helper with exact-match fallback during migration (T014, T015)
17. T017 — Blue/green re-embed migration + boot-time config-vs-meta check (T014, T015, T016, T003, T012)

**Phase 4: Search Cache**

18. T018 — `search_cache` table, linked to vectors via cache_id (T014)
19. T019 — Query normalization, exact store/lookup, enqueue embed job on store (T018, T015, T007)
20. T020 — Semantic search-cache lookup via vector similarity gate (T019, T016)

**Phase 5: Scrape Cache & Content Hashing**

21. T021 — Scrape cache schema + cache-check-before-insert (fixes new-row-per-refetch) (T007)
22. T022 — Conditional refresh via ETag/Last-Modified/304 + cross-URL content dedupe (T021)
23. T023 — Wire scrape cache into `scraper.one` resolver (cache -> chromedp) + provenance (T021, T022)

**Phase 6: Memory, Distillation & Confidence Gating**

24. T024 — `memory_facts` table, linked to vectors (T014)
25. T025 — Distill job: extract multiple atomic facts per page with volatility + horizon (T024, T010, T005, T023)
26. T026 — Semantic upsert: update near-identical fact instead of insert (T024, T016, T025)
27. T027 — Memory retrieval: top-k relevant facts by similarity (T024, T016)
28. T028 — Confidence gating: similarity + freshness + LLM adjudication (default on) (T027, T005, T007)

**Phase 7: Tiering & Cleanup**

29. T029 — Apply sliding expiry + tier promotion on hits in all three stores (T007, T019, T023, T027)
30. T030 — Per-store cleanup jobs (delete where expires_at < now) + cascade vectors (T012, T018, T021, T024, T014)

**Phase 8: Resolver Chain**

31. T031 — Dispatch: URL present -> scrape path; text only -> search path, in code (T020, T023)
32. T032 — Search resolver chain memory -> search cache -> engines, source-tagged (T020, T028, T019)

**Phase 9: MCP/REST Surface**

33. T033 — `search` tool/endpoint: use_cache, use_memory, max_age params + source tag (T032, T031)
34. T034 — `scrape` tool/endpoint: use_cache param + provenance (T023, T031)
35. T035 — `memory.query` + `memory.store` tools/endpoints (T027, T028, T025)
36. T036 — "Remember this" flag: enum short|long|permanent selecting initial tier, default on (T035, T024, T007)

**Phase 10: Finalization**

37. T037 — README refresh (final behavior, config, job system, provenance) + end-to-end integration tests (all prior)

## Notes, Risks & Assumptions

- **Turso native vector search is pre-1.0 (RISK, called out in T014/T016/T017).**
  We commit fully to Turso native vector search — there is no fallback vector
  implementation. The exact vector column type and ANN index syntax must still be
  verified against current Turso docs before implementing (the bindings and driver
  name have moved recently — see the `store.go` comment and `database.driver`
  config); if the syntax has changed, adjust T014/T016 to match, but stay on Turso.
  The only degradation retained is migration-time (T016/T017): while a re-embed
  runs, semantic lookup is unavailable, so the search cache uses exact query match
  and memory simply misses to the web until the vectors are rebuilt.
- **`clean.go` does HTML content cleaning (`cleanHTML`), not TTL cleanup.** There
  is no existing cache-cleanup job to build on; T030 creates cleanup from scratch.
  Distillation (T025) consumes the cleaned `text_content` that `clean.go` already
  produces.
- **Behavior change — re-scrape.** Today `scraper.one` (scraper.go) always calls
  `SaveScrape`, inserting a new row with no cache check (documented in README
  "Known limits"). T021/T023 change this to a cache hit path. README must be
  updated (T001 early, T037 final).
- **Behavior change — DB concurrency.** `max_open_conns=1` in both `config.go`
  defaults and `config.toml` serialises writes. The job system's poller + worker
  pool need concurrent connections; T002 raises the default and documents the
  Turso implications, and T013 depends on it.
- **Deferred embeddings.** Rows are stored with a null vector and an embed job is
  enqueued (T015/T019/T026); exact-match works immediately and semantic search
  catches up. During a model/dim migration (T017) semantic search falls back to
  exact-match and flips back on when zero stale vectors remain.
- **Two orthogonal axes kept separate.** Tier (durability, chosen by the caller's
  remember flag → initial ceiling, T036/T007) and volatility (staleness rate,
  emitted by the distiller → freshness gate max_age, T025/T028) are never merged.
- **One-shot modes removed (T038).** The `-mode search` and `-mode scrape` one-shot
  test modes are removed; only `-mode browse` (manual profile warming) and
  `-mode serve` remain. All search/scrape now run under serve, where the job system
  is live (T013), so the embed/distill queue is always drained by the running
  process — the earlier one-shot best-effort concern no longer applies. T038 must
  preserve the underlying search/scrape functions that serve mode's REST/MCP
  handlers call; only the CLI dispatch and one-shot flags are removed.
- **Assumption — embed dim.** Start at Qwen3-Embedding-8B full dim 4096; Matryoshka
  truncation is deferred. The vectors table stamps model+dim per row so a later
  change is handled by the T017 migration.
- **Tests per feature** (go-agent convention): each task carries its own
  verification; T037 adds the cross-cutting end-to-end integration pass. New DB
  tables are exercised against SQLite as the existing suite already does.
