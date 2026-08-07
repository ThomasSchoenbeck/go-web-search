# go-web-search (prototype)

Local Go tool that drives a real Chrome instance through `chromedp` to run a list
of search terms against Google, Bing and DuckDuckGo, and writes the destination
URLs into a per-run artifact folder. Flat single-package layout; every file is
`package main`.

```
main.go          config load, flag overrides, mode dispatch
config.go        TOML config + defaults
lock.go          single-process lock over the data directory
runlog.go        run folder + console/file mirrored logger

browser.go       Chrome launch, persistent profile, UA handling, link collection
search.go        typed (search box) and direct-URL search flows
engine.go        per-engine table + flag parsing helpers
extract.go       redirect unwrapping, infrastructure filtering, dedupe
scraper.go       HTTP-first fetch with browser fallback, per-host politeness
clean.go         raw -> clean HTML -> flat text renderings
robots.go        robots.txt fetch, cache and RFC 9309 status handling
harvest.go       run orchestration shared by the server

store.go         Turso main database: runs, searches, raw SERPs, URLs
logstore.go      Turso log database: batched async writer + the read query
schema.sql       main schema (embedded)
schema_logs.sql  log schema (embedded)
searchcache.go   query -> result URLs, exact + semantic lookup, browse listing
scrapecache.go   URL -> content, conditional refresh, browse listing
memory.go        distilled facts: storage, retrieval, confidence gating
vectors.go       generation vector tables, cosine search, literal encode/decode
meta.go          system_meta key/value (active model, dim, migration state)
tier.go          short/long/permanent tiers with sliding expiry
retention.go     raw-content trimming and scheduled VACUUM

jobs.go          worker pool, poller, stale-job reaper
jobstore.go      the jobs table: claim/complete/retry + the monitor listing
wiring.go        job type -> handler registration
embed.go         deferred embedding jobs
distill.go       page -> atomic facts via the chat model
reembed.go       blue/green re-embed migration on a model or dim change
cleanup.go       expiry sweep
llm.go           OpenAI-compatible chat + embedding client
resolver.go      memory -> cache -> engines resolution order
gate.go          confidence gates over retrieved facts

server.go        REST + MCP routes, handlers, serve mode
webui.go         go:embed of web/dist, static serving, SPA fallback
provenance.go    URL pivot and whole-run causality reads
explorer.go      semantic nearest-neighbour probe
projection.go    bounded raw-vector dump for the 2-D scatter
stats.go         the /api/stats snapshot
testsupport.go   throwaway test environments, fixtures, -mode testserve

web/             the observability SPA (Svelte 5 + Vite + TypeScript)
plans/           design docs and task breakdowns per feature
```

## Modes

**`-mode browse`** opens a real window on the persistent profile and hands it to
you. Accept the consent dialogs, search a few things by hand, click into some
results. Finish by closing the window or pressing Enter — Chrome only flushes
cookies on a clean exit, and both paths are clean.

**`-mode serve`** starts the REST + MCP server *and the observability UI* on one
listener, and is the only way to run searches and scrapes. The former one-shot
`-mode search` / `-scrape` test modes were removed once caching and memory moved
the search/scrape flow behind the server, where the background job system is
live.

**`-mode testserve`** is the same HTTP surface with no Chrome, no job runner and
no vector boot, over whatever `-data` points at. It exists for the automated test
harness and is not a deployment mode.

```bash
go mod tidy
go run . -mode browse
go run . -mode serve
```

Note that `go run .` serves the UI only if `web/dist` was built first — see
[Build model](#observability-ui) below.

## Cache & memory (in progress)

The server minimizes web work through three Turso-backed stores plus a semantic
memory, all fed by a single crash-safe job system. The design and its task
breakdown live in `plans/cache-memory/`.

- **Search cache** — normalized query → result URLs, with the query embedded so
  a differently worded but equivalent question can hit a prior search. Short,
  freshness-sensitive TTL.
- **Scrape cache** — URL → fetched content, keyed exactly on URL, with a content
  hash and ETag/Last-Modified conditional refresh. Replaces the old per-fetch
  `scrapes` insert. No embedding.
- **Memory** — atomic facts distilled from scraped pages, each embedded, tiered
  and long-lived. Retrieval pulls the top-k relevant facts and the chat model
  synthesizes an answer; a hit that clears the confidence gates skips the web.

Every cache/memory row carries a tier (`short` / `long` / `permanent`) with
sliding expiry: a hit slides its expiry forward, and enough hits promote `short`
→ `long`. A write's `remember` flag picks the starting tier. A separate
volatility label, emitted by the distiller, drives a freshness gate independent
of the tier.

Vectors live in a dedicated table using Turso's native vector types. **There is
no ANN index:** tursogo 0.7.1 is the Rust Turso engine, which implements
`F32_BLOB` columns and `vector32()` / `vector_distance_cos()` but not libSQL's
`libsql_vector_idx` or `vector_top_k()`. Similarity search is therefore an exact
linear scan over every row of an owner kind — cheap at the scale of a local
research tool, and the place to reintroduce an approximate index if this ever
moves to a libSQL backend. Embeddings and distillation run as deferred jobs — a
row is stored first and its vector filled in later — so writes never block on the
model service. Embedding uses Qwen3-Embedding-8B via a self-hosted, OpenAI-compatible
llama.cpp endpoint configured under `[[llm.model]]`; a model or dimension change
triggers a blue/green re-embed migration.

## Observability UI

A read-only web UI for inspecting everything the harvester stores and does. It is
a Svelte 5 + Vite **client-only SPA written in TypeScript** — not SvelteKit, no
SSR, no server adapter — built to static assets and embedded into the Go binary,
served by the existing `-mode serve` listener alongside the current REST and MCP
routes. Single binary, single service, no separate deployable. The design and its
task breakdown live in `plans/observability-ui/`.

Routing is History-API clean URLs with a server-side fallback, so every view can
be linked to directly and a deep link survives a reload. A persistent navigation
shell renders from one route table, so a route cannot exist without a way to
reach it.

| Route | View |
| --- | --- |
| `/runs`, `/runs/{id}` | runs list, and a run's URLs, searches and scrapes |
| `/runs/{id}/searches` | per-engine detail for a run |
| `/runs/{id}/causality` | the whole-run chain: searches → URLs → scrapes → facts |
| `/searches/{id}` | the stored SERP, rendered in a fully sandboxed iframe |
| `/scrapes/{id}` | raw / clean / text toggle, stored images, cache metadata |
| `/provenance?url=` | pivot on one URL: what found it, what it produced |
| `/facts`, `/facts/{id}` | distilled memory facts and their source material |
| `/explore` | semantic nearest-neighbour probe with cosine distance |
| `/projection` | the embedding space projected to 2-D |
| `/jobs` | the background queue: status, attempts, backoff, locks |
| `/cache/searches`, `/cache/scrapes` | both caches: tier, expiry, hit count, sizes |
| `/logs` | the log database, newest first, as a filterable tail |
| `/stats` | counts, size aggregates, cache tiers, embedding and job state |

Two views refresh themselves: **jobs and logs poll**, seeded from config and
overridable live (see below). Nothing else auto-refreshes.

**The 2-D projection is computed in the browser.** `/api/projection` returns
bounded raw embedding vectors and the SPA reduces them with **PCA** — chosen over
UMAP/t-SNE because it is deterministic, so the same store always draws the same
picture. It is a local helper in `web/src/lib/projection.ts`, not a dependency,
and there is no linear algebra on the Go side. The plot says so, but it bears
repeating: two dimensions cannot carry what a 4096-dimensional space knows, so
distances there are indicative, not exact.

**Stored HTML is never injected.** Raw SERPs and cleaned page bodies render in
`<iframe sandbox="" srcdoc=...>`, so untrusted stored markup cannot script the
inspection UI that displays it.

**Cache reuse is reported as hit counts, not hit rates.** The caches record
`hit_count` on the rows that exist and nothing about lookups that missed, so a
true rate is not derivable from stored data — and counting misses would mean
writing new counters on every read, which a read-only UI does not do. The stats
dashboard shows counts and tier distributions, and says why there is no rate.

**Inspection only in v1.** The UI reads existing tables and adds read-only JSON
endpoints where none exists. It triggers no searches, scrapes or distillation and
performs no destructive operations; the existing write endpoints
(`/api/memory/store`, `/api/distill/preview`, `/api/vacuum`) are out of scope for it.

**Access model — edge auth, no app-level auth.** The SPA shell and every `/api/*`
route are served openly; authentication is delegated to an edge layer (reverse
proxy, gateway, or trusted network) in front of the binary. There is no login
screen and no token handling in the SPA. The existing `server.api_key` bearer
stays supported for anyone exposing the binary directly, but the assumed
observability-UI deployment leaves it unset. This is explicitly not a hardened
public-auth model.

**Build model — the frontend build runs before `go build`.** The frontend lives
under `web/` and is built with Node + Vite (via pnpm) into `web/dist/`, which is
then embedded into the binary with `go:embed`. `web/dist/` is **gitignored and
generated at build time — it is not committed**, so a plain `go build` with no
prior frontend build embeds no UI. The Node/Vite step must therefore run first.
`Taskfile.yaml` makes that a dependency rather than something you are trusted to
remember: every binary target depends on the `web` target.

```bash
task build            # web -> dist/ for windows-amd64, linux-amd64, linux-arm64
task web              # just the SPA, into web/dist/
```

By hand, the ordering is the whole point:

```bash
pnpm --dir web install --frozen-lockfile
pnpm --dir web build      # emits web/dist/
go build .                # embeds web/dist/
```

In development the Vite dev server serves the SPA instead and proxies `/api`,
`/mcp` and `/healthz` to the serve listener, so no rebuild is needed while
iterating. The proxy targets `localhost:8082`, which is `server.addr` in both the
shipped `config.toml` and the compiled defaults; override it with
`VITE_PROXY_TARGET` when the backend runs elsewhere. Those three are pinned
together by a test, because a mismatch is silent — the dev server would proxy to
a port nothing is listening on.

A binary compiled without that first step still runs: it serves a plain-text
"Observability UI not built" notice with the build commands, and REST and MCP on
the same listener are unaffected. Only `web/dist/.gitkeep` is committed, which is
what keeps a clean checkout compiling — `go:embed` errors on an empty directory,
and needs its `all:` prefix to see a dotfile at all.

**Node, pnpm and Vite are build-time dependencies** — required to build a release
binary, not to run one.

**Every UI setting lives in `config.toml`, under `[observability]`,** and reaches
the SPA through `GET /api/ui-config` at runtime rather than being baked in at
build time:

| Key | Default | What it bounds |
| --- | --- | --- |
| `poll_interval` | `5s` | how often the jobs and logs views refresh |
| `poll_enabled` | `false` | whether those views start polling at all |
| `projection_sample_cap` | `2000` | vectors the 2-D projection may pull |
| `causality_max_urls` | `200` | URLs in one run's causality graph before it truncates |
| `job_timing_sample` | `200` | finished jobs `/api/stats` averages a duration over |

The first two are session defaults: each polling view has a dropdown for the
interval and a button to toggle polling, which override for the moment and are
never written back. If config says polling starts off, the views start off.

`/api/ui-config` is built field by field rather than by marshalling the config
struct, so adding a setting cannot leak one by accident; `api_key` is never
exposed, and a test asserts the response carries those keys and nothing else.

### Tests

Three suites, all runnable with `task test`, which builds the SPA first because
the end-to-end run serves the embedded assets:

```bash
go test ./...            # backend, including every endpoint
pnpm --dir web test      # frontend unit tests (Vitest)
pnpm --dir web test:e2e  # end-to-end (Playwright, drives the real binary)
```

Every suite runs against a **throwaway main DB and log DB** and removes them
afterwards — no suite ever opens `./data`. Go tests get theirs from the helper
in `testsupport.go`; the Playwright harness creates a temp directory, starts the
binary in `-mode testserve` (browserless, seeded with a small fixture dataset)
on a free port, and deletes the directory on completion whether the run passed,
failed or was interrupted. `-mode testserve` exists only for that harness.

## Concurrency

Engines run in parallel per term, each in its own tab, but staggered:
engine *N* waits `engine_stagger * N + rand(engine_jitter)` before starting.
Firing all three simultaneously from one profile is the burst pattern that gets
you challenged; the stagger costs a couple of seconds and breaks the
correlation. Set both to `0s` for true simultaneity.

Tabs matter here beyond parallelism: `Emulation.setUserAgentOverride` and the
stealth script are both *per-target*, so `newTab` reapplies them. Without that a
second tab would leak `HeadlessChrome` while the first looked clean.

Scraping groups URLs by host. Hosts run in parallel up to `max_domains`; URLs
within a host run one after another with `per_domain_delay`. That is the point
of grouping by domain rather than firing N workers at a flat queue — it stays
fast without hammering any single server.

What actually runs concurrently:

| Stage | Concurrency | Bound by |
| --- | --- | --- |
| Engines within a term | parallel, staggered | `engine_stagger`, `engine_jitter` |
| Terms | sequential | `min_delay` / `max_delay` between them |
| Hosts while scraping | parallel | `max_domains` (default 8) |
| URLs within one host | sequential, deliberately | `per_domain_delay`, robots `Crawl-delay` |
| Browser fallback tabs | parallel | `max_browser_tabs` (default 3) |
| Database writes | serialised by default | `database.max_open_conns` |

The last two are ceilings rather than politeness. Each fallback tab is a Chrome
renderer process, so unbounded fallback on a page of JS-heavy results would open
a tab per URL. And `max_open_conns = 1` is the conservative setting while the
Turso bindings are beta — writes happen after the network fetch and take
microseconds, so serialising them costs almost nothing, but raise it if you want
Turso's concurrency doing real work.

Sequential-within-a-host is the one piece of deliberate slowness, and it is what
you asked for by grouping on domains: it is what makes the crawl polite.

## Scraping

Plain HTTP first. If the response is HTML but yields almost no text (under
`min_text_chars`, or containing an empty `id="root"` / `id="__next"` mount
point), it is re-fetched through the browser — that is what a client-rendered
app looks like from the outside.

robots.txt is fetched once per host and cached. Status handling follows RFC 9309
via `robotstxt.FromStatusAndBytes`: 4xx allows, 5xx disallows. An unreachable
robots.txt allows, as mainstream crawlers do. `Crawl-delay` is honoured when
`respect_crawl_delay` is set. Disallowed URLs are still recorded, with
`robots_allowed = 0`, so you can see what was skipped and why.

Each scrape stores three renderings: `raw_html` as received, `clean_html` (body
only, with script/style/nav/header/footer/aside/form/comments removed), and
`text_content` flattened and whitespace-collapsed. Images found in the body are
stored inline as JSON on the cache row, with alt text and declared dimensions —
URLs only, no bytes downloaded. (The former per-fetch `scrapes` and
`scrape_images` tables are gone: a scrape is now one `scrape_cache` row keyed on
URL.)

## REST API

Work-triggering routes — these are the only ones that write or reach the network:

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/search` | `{terms, scrape, use_cache, use_memory, max_age_seconds, remember}` → source-tagged URLs and/or memory answers |
| POST | `/api/scrape` | `{urls}` or `{run_id}`, plus `use_cache, max_age_seconds, remember` → scrape results with provenance |
| POST | `/api/memory/query` | `{question}` → a synthesized answer when the confidence gates pass |
| POST | `/api/memory/store` | `{text, source_url, volatility, remember}` → fact id |
| POST | `/api/distill/preview` | run the distiller over one scrape with overridable settings, storing nothing |
| POST | `/api/vacuum` | VACUUM on demand, reporting how much the file shrank |

Read-only routes. The observability UI uses only these, and never sends an
`Authorization` header:

| Method | Path | Purpose |
| --- | --- | --- |
| GET | `/api/runs` | recent runs |
| GET | `/api/runs/{id}` | run summary with counts |
| GET | `/api/runs/{id}/urls` | URLs found, best rank first |
| GET | `/api/runs/{id}/searches` | per engine detail |
| GET | `/api/runs/{id}/scrapes` | scrape ids |
| GET | `/api/runs/{id}/causality` | the whole-run graph, truncated at `causality_max_urls` |
| GET | `/api/searches/{id}/raw` | stored SERP HTML |
| GET | `/api/scrapes/{id}` | cleaned document, `?raw=1` for the original |
| GET | `/api/memory/facts`, `/{id}` | distilled facts; the detail adds its source scrape's sizes |
| GET | `/api/provenance?url=` | one URL's backward and forward chain |
| GET | `/api/explore?q=&k=` | nearest neighbours across facts and cached queries, with distance |
| GET | `/api/projection?limit=&offset=` | raw embedding vectors, capped by `projection_sample_cap` |
| GET | `/api/jobs?status=&type=` | the queue, plus whole-queue counts by status |
| GET | `/api/cache/searches?tier=&q=` | cached queries: tier, expiry, hits, results summary |
| GET | `/api/cache/scrapes?tier=&q=` | cached pages: tier, expiry, hits, content sizes |
| GET | `/api/logs?run_id=&level=&source=` | the separate log database, newest first |
| GET | `/api/stats` | counts, size aggregates, embedding state, cache tiers, job throughput |
| GET | `/api/ui-config` | the non-secret `[observability]` settings the SPA reads at startup |
| GET | `/healthz` | liveness |

Every list endpoint takes `limit` and `offset`; `limit` is clamped to [1,500] and
defaults to 50, so an unset or hostile limit cannot ask for a whole table. An
empty result is an empty list and a 200, never an error — "nothing points at this
yet" is a real observation. Where vectors are involved the answer is tri-state:
available, no table yet, or a re-embed migration in flight, each reported as a
note rather than a failure.

Set `server.api_key` to require `Authorization: Bearer <key>` on everything
except `/healthz`. The observability deployment leaves it unset — see the access
model above.

## MCP

Streamable HTTP at `/mcp`, on the same listener. Tools: `web_search`,
`web_scrape`, `get_scrape`, `get_run`, `memory_query`, `memory_store`.
`web_search` consults memory first and tags every result by source
(memory|cache|live). Both write tools are synchronous and
bounded by `scrape.max_urls` and `scrape.total_timeout`, so a call cannot run
past an LLM tool timeout; when the cap truncates a result the response says so.
`web_scrape` returns text snippets capped at `snippet_chars` alongside the ids,
so a model can judge relevance without a second round trip and still fetch the
full document when it wants one.

## Running on a headless Ubuntu server

The profile always lands wherever Chrome runs, so `-userdata` stays server-side
no matter which device you drive it from. Chrome inherits `DISPLAY` from the
shell that starts the harvester, so a virtual display is all it takes:

```bash
sudo apt install -y xvfb x11vnc openbox novnc websockify \
                    fonts-liberation fonts-dejavu fonts-noto-color-emoji

Xvfb :1 -screen 0 1600x900x24 &
DISPLAY=:1 openbox &
x11vnc -display :1 -localhost -rfbauth ~/.vnc/passwd -forever -shared &
websockify --web=/usr/share/novnc 6080 localhost:5900 &

DISPLAY=:1 go run . -mode browse
```

Then open `http://<server>:6080/vnc.html` from any browser - Windows or Android,
nothing to install on the client. Chrome always renders into the same virtual
display, so the profile sees one consistent screen size regardless of which
device warmed it.

Fonts matter: a minimal server install has almost none, and Chrome will both
render boxes and present a very unusual font fingerprint without them. A window
manager matters too, or consent dialogs come up undecorated and misbehave.

Note that a virtual display means software rendering, so WebGL will report
SwiftShader or llvmpipe - a known automation signal that no amount of profile
warming hides. If the server is a VPS rather than a box on your own LAN, its IP
reputation will likely dominate everything else here.

## Shutdown behaviour

Chrome only flushes cookies on a clean exit, so all three exit paths are
handled:

- **Closing the browser window** is clean - Chrome flushes on its way out. The
  program notices the connection drop and exits, so no terminal is needed. This
  is the path to use from a tablet.
- **Pressing Enter** in browse mode asks Chrome to close itself.
- **SIGINT/SIGTERM** in serve mode stops accepting requests and drains in-flight
  ones (capped at 20 seconds), cancels the job context so every worker goroutine
  winds down, then closes Chrome gracefully. The signal context is deliberately
  not the parent of the browser context, because cancelling that kills Chrome
  outright - the exact ungraceful exit that loses the session.

A second Ctrl-C is not honoured; the graceful Chrome close is capped at 15
seconds.

## Verification status

`go vet` is clean and three suites pass: **96 Go tests**, **226 frontend unit
tests** (Vitest) and **82 end-to-end tests** (Playwright).

The Go tests run against a **real Turso database** — driver `turso`, tursogo
0.7.1, the same engine as production — created fresh under a temp directory per
run and deleted afterwards. That covers the schema and every statement in the
store, cache, memory, vector, job, provenance, projection and log paths, plus
each endpoint through the real `http.Handler`. No suite ever opens `./data`.

The Playwright suite spawns the **actual compiled binary** in `-mode testserve`
on a free port over a throwaway database, so the embedded SPA, the router, the
SPA fallback and every read endpoint are exercised as shipped, not as mocks.

### End-to-end pass on the embedded binary

Built the documented way (`task web`, then `go build .`) and exercised over a
throwaway data directory, with **no `Authorization` header sent on any request**:

- The SPA shell and its hashed Vite assets are served at `/`, and deep links
  (`/runs`, `/logs`, `/projection`) resolve through the fallback to the app.
- Every read endpoint the UI uses answers 200 unauthenticated with `api_key`
  unset — runs, run detail, searches, causality, raw SERP, scrape detail, memory
  facts, provenance, explorer, projection, jobs (filtered and not), both cache
  browsers, logs (filtered and not), stats and ui-config. Auth is the edge's job.
- An unregistered `/api/...` path stays a 404 instead of being answered with
  `index.html` — a typo'd endpoint must not look like a working one.
- Polling behaves: the jobs and logs views start paused when config says so, the
  dropdown changes the cadence live, the toggle starts and stops the refresh, and
  leaving the view stops it — verified by watching real request traffic.
- A binary compiled **without** a prior Vite build serves a plain-text "not
  built" notice with the build commands and a 503, while `/api/*` and `/healthz`
  keep working. The build-ordering rule is enforced by behaviour, not just docs.

### Still unverified

**A full `-mode serve` run has not been exercised here**: that path launches real
Chrome and needs a reachable llama.cpp endpoint, so the browser-driven search,
the scraper against live sites, distillation and real embeddings are covered by
type-checking and unit tests but not by an end-to-end run. `-mode testserve`
deliberately starts no browser, no job runner and no vector boot, so those three
subsystems are the gap. The MCP SDK's localhost/DNS-rebinding protection has also
not been tried from another host on the LAN.

Go's flag package needs `-headless=true`, not `-headless true`. The spaced form
sets the bool and then treats `true` as a positional argument, silently stopping
flag parsing.

## Known limits

- First result page only; no pagination.
- API calls are synchronous by choice: the request blocks until the work is done
  rather than returning a job id. This bounds *when you get an answer*, not how
  the fetching runs — see Concurrency.
- Scrapes are cached by URL with conditional refresh, so repeated calls reuse
  stored content (or a cheap 304) instead of refetching.
- Don't point `-userdata` at your real Chrome profile — Chrome holds a
  `SingletonLock` on it. Keep a pristine copy of the harvest profile; once one
  gets flagged, restoring beats re-warming from nothing.
- An interrupt takes effect between queries, not during one, so it can take up
  to `-query-timeout` to land.
- No proxy or user-agent rotation.
- The observability UI is read-only and has no live push: jobs and logs poll, and
  there is no SSE or WebSocket endpoint. An interval short enough to feel live is
  a request per view per tick.
- Vector search is an exact linear scan (no ANN index), and `/api/projection`
  reads whole embeddings. Both are bounded by config rather than by an index, so
  the caps matter more as the store grows.

Automated querying is against the terms of service of all three engines. Keep
the delays generous and the volume low.
