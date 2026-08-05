---
feature: Scrape Cache & Content Hashing
task_number: 023
description: Scrape resolver wiring in scraper.one (cache -> chromedp) + provenance
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 023: Scrape resolver wiring in `scraper.one`

## Description

Rewire `scraper.one` (scraper.go) so the scrape cache is consulted before doing
any network work. The resolver order for the scrape path is: scrape cache
(T021/T022) first; on a fresh cache hit, return the cached content without
fetching; on a miss or expiry, fall through to the existing fetch path (HTTP,
then chromedp browser fallback) and store the result in the cache. This replaces
today's behavior where `scraper.one` always inserts a new `scrapes` row with no
cache check.

Add provenance to the outcome: each scrape result must indicate whether it came
from the cache or was fetched live (and, when fetched, how — http vs browser, as
`fetched_with` already records). The `ScrapeOutcome` shape in `scraper.go` gains
a provenance/source field so callers and the MCP/REST layer (T034) can surface
it. Keep robots.txt handling and the existing fetch/clean logic intact — only the
cache short-circuit and provenance are added here.

## Goal

`scraper.one` returns cached content on a fresh hit and only fetches on
miss/expiry, storing new fetches in the cache, and every outcome carries
provenance (cache vs live).

## How to Verify

- A second scrape of the same URL within its freshness window returns the cached
  content and does not perform an HTTP or browser fetch (verified via a fetch
  counter or `httptest` in a test).
- A miss/expired entry fetches live and stores the result in `scrape_cache`.
- `ScrapeOutcome` includes a provenance field set to cache or live.
- Robots handling and the browser-fallback heuristic still work.
- Tests cover cache-hit-no-fetch, miss-then-store, and provenance values.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `scraper.go`
- `scraper_test.go` [NEW]

## Dependencies

T021, T022.
