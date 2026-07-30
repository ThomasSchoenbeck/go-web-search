# go-web-search (prototype)

Local Go tool that drives a real Chrome instance through `chromedp` to run a list
of search terms against Google, Bing and DuckDuckGo, and writes the destination
URLs into a per-run artifact folder. Flat single-package layout; every file is
`package main`.

```
main.go          config load, flag overrides, mode dispatch, run loop
config.go        TOML config + defaults
store.go         Turso main database: runs, searches, raw SERPs, URLs
logstore.go      Turso log database with a batched async writer
lock.go          single-process lock over the data directory
schema.sql       main schema (embedded)
schema_logs.sql  log schema (embedded)
browser.go       Chrome launch, persistent profile, UA handling, link collection
search.go        typed (search box) and direct-URL search flows
engine.go        per-engine table + flag parsing helpers
extract.go       redirect unwrapping, infrastructure filtering, dedupe
runlog.go        run folder + console/file mirrored logger
```

## Modes

**`-mode browse`** opens a real window on the persistent profile and hands it to
you. Accept the consent dialogs, search a few things by hand, click into some
results. Finish by closing the window or pressing Enter — Chrome only flushes
cookies on a clean exit, and both paths are clean.

**`-mode serve`** starts the REST + MCP server on one listener, and is the only
way to run searches and scrapes. The former one-shot `-mode search` / `-scrape`
test modes were removed once caching and memory moved the search/scrape flow
behind the server, where the background job system is live.

```bash
go mod tidy
go run . -mode browse
go run . -mode serve
```

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

Vectors live in a dedicated table with the ANN index, using Turso's native
vector search. Embeddings and distillation run as deferred jobs — a row is
stored first and its vector filled in later — so writes never block on the model
service. Embedding uses Qwen3-Embedding-8B via a self-hosted, OpenAI-compatible
llama.cpp endpoint configured under `[[llm.model]]`; a model or dimension change
triggers a blue/green re-embed migration.

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
`text_content` flattened and whitespace-collapsed. Images found in the body land
in `scrape_images` with alt text and declared dimensions — URLs only, no bytes
downloaded.

## REST API

| Method | Path | Purpose |
| --- | --- | --- |
| POST | `/api/search` | `{terms, scrape, use_cache, use_memory, max_age_seconds, remember}` → source-tagged URLs and/or memory answers |
| POST | `/api/scrape` | `{urls}` or `{run_id}`, plus `use_cache, max_age_seconds, remember` → scrape results with provenance |
| POST | `/api/memory/query` | `{question}` → a synthesized answer when the confidence gates pass |
| POST | `/api/memory/store` | `{text, source_url, volatility, remember}` → fact id |
| GET | `/api/runs` | recent runs |
| GET | `/api/runs/{id}` | run summary with counts |
| GET | `/api/runs/{id}/urls` | URLs found, best rank first |
| GET | `/api/runs/{id}/searches` | per engine detail |
| GET | `/api/runs/{id}/scrapes` | scrape ids |
| GET | `/api/searches/{id}/raw` | stored SERP HTML |
| GET | `/api/scrapes/{id}` | cleaned document, `?raw=1` for the original |
| GET | `/healthz` | liveness |

Set `server.api_key` to require `Authorization: Bearer <key>` on everything
except `/healthz`.

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
- **SIGINT/SIGTERM** stops the search loop between queries, writes `urls.txt`
  with whatever was collected, then closes Chrome gracefully. The signal context
  is deliberately not the parent of the browser context, because cancelling that
  kills Chrome outright - the exact ungraceful exit that loses the session.

A second Ctrl-C is not honoured; graceful close is capped at 15 seconds.

## Verification status

`go vet` is clean and nine unit tests pass (URL unwrapping and filtering,
results-parameter rules, flag parsing, landed-query matching, host grouping,
relative-URL resolution, whitespace collapsing, content-type detection).

The whole program is type-checked against stub packages carrying the exact
upstream signatures for chromedp, cdproto, the MCP SDK, robotstxt, x/net/html,
BurntSushi/toml and google/uuid.

All SQL — schema plus every statement in `store.go` and `logstore.go`, including
the `RunURLs` GROUP BY and the `GetScrape` join — was executed against real
SQLite 3.45 with representative data. Turso is SQLite-compatible, so that is
genuine coverage.

**Not verified: any of it running.** The search path was confirmed against real
Chrome earlier, but the database layer, scraper, REST API and MCP server have
never executed. Type-checking against stubs proves the API usage compiles; it
proves nothing about behaviour. Expect the first run to surface things — most
likely the Turso driver name, the robotstxt helper signature, and whether the
MCP SDK's localhost/DNS-rebinding protection needs configuring for LAN access.

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

Automated querying is against the terms of service of all three engines. Keep
the delays generous and the volume low.
