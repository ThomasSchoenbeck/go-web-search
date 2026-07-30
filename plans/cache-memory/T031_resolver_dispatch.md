---
feature: Resolver Chain
task_number: 031
description: Dispatch — URL present -> scrape path; text only -> search path
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 031: Resolver dispatch

## Description

Add the top-level dispatch that decides, in code, which resolver path a request
takes: if a URL is present, it is a scrape request and goes down the scrape path
(scrape cache -> chromedp, T023); if only text is present, it is a search request
and goes down the search path (memory -> search cache -> engines, T032). This
logic lives in code, not in tool descriptions — the MCP/REST tools are thin
wrappers over it.

Put the dispatcher in a new `resolver.go`. It is the single entry point the
search/scrape tools (T033/T034) call. Follow go-agent conventions: context-first,
small interfaces over the search-cache, scrape-cache, memory, and engine paths so
the dispatcher is testable with fakes and does not hard-depend on the whole
harvester. This task establishes the dispatch and the scrape-path wiring; the
full search-path chain is T032.

## Goal

A code-level dispatcher routes URL-present requests to the scrape path and
text-only requests to the search path.

## How to Verify

- `resolver.go` exposes a dispatch entry point that inspects the request and
  routes to scrape vs search.
- A request carrying a URL invokes the scrape path; a text-only request invokes
  the search path.
- The decision is in code (no reliance on tool-description wording).
- `resolver_test.go` covers both routing branches with fake path implementations.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `resolver.go` [NEW]
- `resolver_test.go` [NEW]

## Dependencies

T020, T023.
