---
feature: Foundation & Documentation
task_number: 038
description: Remove one-shot -mode search/-mode scrape (keep browse + serve)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 038: Remove one-shot `-mode search`/`-mode scrape`

## Description

Remove the one-shot CLI test modes so only `-mode browse` (manual profile
warming) and `-mode serve` (REST + MCP server) remain. Today `main.go` defaults
`-mode` to `search`, dispatches `search` to `searchMode`, and exposes one-shot
flags such as `-scrape`, `-terms`, `-engines`, `-typed`, `-results`, and the
search-timing overrides that only make sense for a one-shot term-list run. Under
the new design all search/scrape work happens under serve mode, where the job
system is live (T013), so the earlier one-shot best-effort concern no longer
applies.

Critically, this task must PRESERVE the underlying search and scrape functions
that serve mode's REST/MCP handlers call — `harvester.SearchTerms`,
`harvester.ScrapeRun`, `harvester.ScrapeURLs`, and everything in `harvest.go`,
`search.go`, `scraper.go`. Only the CLI dispatch path (`searchMode` in
`main.go`) and the flags that exist solely to feed it are removed. Change the
default `-mode` to `serve` (or `browse`), and update the unknown-mode error
message to list only the surviving modes.

Because this removes user-facing flags and a documented mode, update the README
"three modes" section and the run examples. This task is intentionally sequenced
early and carries no dependencies; the removed dispatch does not block the new
work, and doing it first keeps later serve-mode wiring clean.

## Goal

`-mode search` and `-mode scrape` no longer exist; `main.go` supports only
`browse` and `serve`; the one-shot-only flags are gone; and the shared
search/scrape functions used by serve-mode handlers are untouched.

## How to Verify

- `go build ./...` succeeds after removal.
- Running the binary with `-mode search` prints an unknown-mode error naming
  only `browse` and `serve`.
- `-mode serve` still starts the server and `web_search`/`web_scrape` still
  function through REST and MCP (existing behavior unchanged).
- `grep` shows no remaining references to `searchMode`, `-scrape`, `-terms`,
  `-engines`, `-typed`, or `-results` one-shot flags in `main.go`.
- The `harvester` search/scrape methods remain present and compile.
- README "three modes" section lists only browse and serve.
- `go vet` and existing tests pass.

## Files to Touch

- `main.go`
- `README.md`

## Dependencies

None.
