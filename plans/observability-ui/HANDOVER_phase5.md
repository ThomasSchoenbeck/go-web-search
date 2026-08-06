# Handover — Observability UI, Phase 5

Disposable working document for the session that implements Phase 5. Delete it
when Phase 5 is `[x]`. Everything here is what a cold session would otherwise
spend a long time rediscovering.

## State

Phases 1–4 are complete and green: T001–T013, T024–T027 are `[x]` in
[index.md](index.md). **Phase 5 is T014–T020** (jobs endpoint + monitor, both
cache browsers, logs query + viewer, stats dashboard). Phase 6 (T021/T022,
projection) and Phase 7 (T023, README + verification) follow.

Read first, in this order: `index.md` (decisions, grounding, notes), then each
task file you are about to implement. The **Notes, Risks & Assumptions** section
of `index.md` carries standing rules that override intuition.

## Verification loop (definition of done, every task)

```bash
go vet ./... && go test ./...     # backend
pnpm --dir web check              # svelte-check: 0 errors, 0 warnings
pnpm --dir web test               # Vitest
pnpm --dir web build              # must precede e2e: e2e serves the embedded SPA
pnpm --dir web test:e2e           # Playwright
```

`task test` runs all of it in the right order. Current baseline: **154 Vitest**,
**49 Playwright**, Go suite green.

`gofmt -l` reports pre-existing drift in `browser.go`, `cache_test.go`,
`config.go`, `meta.go`. Not yours — do not reformat them. Check only files you
touched.

## Backend shape

Flat `package main` at the repo root. No internal packages.

- **Routes** are registered in `newAPIServer` (`server.go`), all behind
  `corsMiddleware(withAuth(mux))`. `withAuth` is a no-op when `api_key` is unset,
  which is the assumed edge-auth deployment.
- Handlers reply with `writeJSON(w, status, v)` / `writeErr(w, status, err)`.
- **Register the catch-all as `/`, never `GET /`** — `GET /` is ambiguous against
  the method-less `/mcp` and ServeMux *panics at registration*.
- `apiServer` fields: `cfg`, `h` (harvester → `h.store`), `llm`, `resolver`,
  `http`, `embed` (a `queryEmbedder`; the LLM client in real modes, a stub in
  tests).
- New read queries live next to their feature: `provenance.go`, `explorer.go`.
  Follow that — `jobs`-related reads belong in a jobs file, not `store.go`.
- Config lives in `config.go` + `config.toml` under `[observability]`. Existing
  keys: `poll_interval`, `poll_enabled`, `projection_sample_cap`,
  `causality_max_urls`. **New caps go here, never hardcoded.** Non-secret UI
  values are exposed through `GET /api/ui-config` (`UIConfig` in `server.go`),
  which is built field-by-field on purpose so a new config field cannot leak.

## Frontend shape

`web/`, Svelte 5 + Vite + TypeScript, pnpm, everything exact-pinned.

```
src/lib/
  request.ts   same-origin getJson/getText, ApiError{status}. No auth header, ever.
  api.ts       endpoint types + resource factories. createResource → {loading,error,data}.
  poll.ts      createPoller(task,{intervalMs,enabled}) → start/stop/setIntervalMs/toggle
  uiconfig.ts  loadUIConfig() — memoized read of /api/ui-config
  routes.ts    THE route table + navEntries
  router.ts    History-API router: path/search stores, navigate(), matchRoute()
  format.ts    formatDuration/formatTimestamp/truncate/formatChars
  graph.ts     causality tree shaping
  apiStub.ts   test-only fetch stub
src/components/Nav.svelte
src/views/*.svelte
```

**Adding a view is three edits:** a `RouteDef` in `routes.ts` (more specific
patterns first — first match wins), a `NavEntry` if it is a top-level
destination, and a branch in the match chain in `App.svelte`. A unit test in
`routes.test.ts` already asserts nav↔route consistency and ordering, so getting
this wrong fails fast.

**View pattern** used by every existing view — copy it:

```svelte
let thing = $derived(thingResource(id))   // rebuilt when the id/prop changes
$effect(() => { void thing.reload() })    // fetches; reads the resource, not $thing
```

Never read `$thing` inside that effect — it loops.

Every element a test touches carries a `data-testid`. Loading / error / empty are
three *distinct* rendered states, each with its own testid.

## Test harness

`testsupport.go` (a normal file, not `_test.go`, because `-mode testserve` uses it):

- `newTestEnv(dir)` → throwaway temp dir + main DB + log DB. `Close()` removes it.
- `env.Server()` → `*apiServer` over those stores, no browser session.
- `newTestServer(t)` (in `uiconfig_test.go`) → `(srv, env)` with teardown registered.
- `seedTestData(ctx, store)` → the fixture dataset (below).
- `seedVectors(ctx, store)` → active vector table + embeddings for the fact and
  the cached search.
- `stubEmbedder{}` → deterministic hash-based 8-dim vectors, so the explorer works
  with no model endpoint. Identical text always embeds identically.
- `testServeMode` → `-mode testserve`: browserless HTTP server, no job runner, no
  vector boot. Seeds when `HARVESTER_TEST_SEED=1`.

Playwright `globalSetup` (`web/tests/e2e/fixtures.ts`) builds the binary, makes a
temp dir, starts `-mode testserve -data <tmp> -port <free>`, publishes
`E2E_BASE_URL` via `process.env`, and deletes everything in teardown. Specs use
`baseUrl('/path')` and the ids in `fixtures-data.ts`.

### Seeded fixture inventory (main DB)

| Table | Rows |
| --- | --- |
| `runs` | 1 seeded (`…0001`) **plus one the testserve process records for itself** |
| `searches` | 2 — google/typed/200 (`…0002`), bing/direct/429 blocked with error (`…000c`) |
| `search_raw` | 1, for `…0002` only — `…000c` is the 404 path |
| `urls` | 2 (`…0003` example.com, `…0004` example.org) |
| `search_urls` | 2, ranks 1 and 2, both from `…0002` |
| `scrape_cache` | 2 — `…0008` full content + 1 image + cache metadata, `…000d` HTTP 404 error, no content |
| `memory_facts` | 1 (`…0009`), source_url = `https://example.com/fixture-one` |
| `search_cache` | 1 (`…000a`), query `fixture term`, 1 result |
| `jobs` | 1 pending `distill` |
| vectors | `vectors_test` table, embeddings for the fact and the search-cache row |

**`runs` counts include the testserve process's own row.** Never assert an exact
`runs` count in e2e; assert on `searches`/`urls`/`scrapes`/`memory_facts`.

## Phase 5 landmines (read before starting)

**T018 changes `newAPIServer`'s wiring and has three call sites, not one.**
Serve mode, plus `testEnv.Server()` and `testServeMode` — both in
`testsupport.go`. Miss them and the Go tests and the entire Playwright harness
stop compiling.

**`LogStore` is write-only.** `logstore.go` has `Write`/`Close` and a batching
goroutine; there is no read path, and `apiServer` holds only the main `Store`.
T018 adds both the query and the wiring. Log schema:

```
logs(id, run_id, level, source, message, created_at)   -- indexes on (run_id,created_at), (level,created_at)
```

**`seedTestData` writes only to the main DB — the log DB is empty.** T018/T019
must seed log rows or the logs viewer has nothing to render and its filters
cannot be exercised in e2e.

**T015/T019 restore the poller's end-to-end coverage.** `lib/poll.ts` is
complete and unit-tested (8 tests), but nothing has driven it from the UI since
the T005 smoke page was replaced by real views. These two tasks wire the interval
dropdown and the on/off toggle, seeded from `/api/ui-config`
(`pollIntervalMs`, `pollEnabled`), overridable live, never written back.

**T020 carries a known open question.** The caches store `hit_count` per row but
not total lookups, so a true hit *rate* may not be derivable. The task must
resolve rate-vs-count while staying read-only — present counts and tier
distributions rather than inventing a denominator, unless lightweight counters
are added deliberately.

**T014/T016 mostly have data already**: 1 pending job, 1 `search_cache` row,
2 `scrape_cache` rows. Extend `seedTestData` if a filter needs more variety
(e.g. a failed job, a `long`/`permanent` tier row) rather than seeding per-spec.

**Vector availability is tri-state.** Where vectors are involved: available /
no table / migration in flight. Report "unknown", never "absent" — the backend
did not make that claim. `activeVectorTable` returns `(table, ready, err)`.

## Gotchas already paid for

- **Go marshals empty slices as `null`, not `[]`.** Every list resource in
  `api.ts` normalises with `?? []`. Do the same for new ones.
- **`URLSearchParams` encodes a space as `+`,** not `%20`. Build `apiStub` keys
  exactly the way `api.ts` builds the URL, or the stub silently 404s.
- `apiStub` uses `'json' in route`, not `route.json ?? {}` — a `null` body is
  meaningful and must survive.
- **`go:embed` needs the `all:` prefix** to include dotfiles, and errors on an
  empty dir. `web/public/.gitkeep` exists so every Vite build restores
  `web/dist/.gitkeep`; do not delete either.
- Vitest needs `resolve: { conditions: ['browser'] }` or Svelte resolves to its
  server build and `mount()` throws.
- Playwright evaluates its config *before* `globalSetup`, so the base URL cannot
  be a config value — it travels via `process.env`.
- Untrusted stored HTML renders in `<iframe sandbox="" srcdoc={…}>`, never
  injected. Applies to any new view showing stored markup.
- Test harnesses spawn the built binary, never `go run` (its child survives the
  kill on Windows and locks the temp DB).

## Standing rules

Read-only v1: no new pipeline behaviour, no writes, no triggering searches,
scrapes or distillation. Every setting in `config.toml`. Exact-pinned deps,
`pnpm audit` clean, peer ranges checked before pinning. Surgical diffs — match
surrounding style, do not refactor adjacent code. Tests are part of each task,
not a phase. All tests run against throwaway databases that are fully removed
afterwards; `./data` is never touched.

When a task's stated facts turn out to be wrong (T008 claimed "no backend
changes" but needed seven fields; T003's suggested embed pattern did not exist),
implement the correct thing, then **correct the task file and `index.md`** rather
than leaving the plan asserting something false.
