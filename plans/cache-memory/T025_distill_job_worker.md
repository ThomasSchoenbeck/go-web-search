---
feature: Memory, Distillation & Confidence Gating
task_number: 025
description: Distill job type + worker (multiple atomic facts, volatility + validity horizon)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 025: Distill job type + worker

## Description

Add the `distill` job type and its handler, registered with the job runner
(T010). Given a scraped page, the worker calls the chat endpoint (T005) to
extract MULTIPLE atomic facts from the page's cleaned `text_content` (the output
`clean.go` already produces). For each fact the model also emits a volatility
value (how fast the fact goes stale) and a validity horizon (until when it should
be trusted). Each extracted fact is then stored into `memory_facts` (via the
semantic upsert, T026), which enqueues an embed job for its vector.

Distillation consumes the cleaned text from the scrape path (T023), so a distill
job is naturally enqueued after a successful scrape. Put the worker in a new
`distill.go`. Follow go-agent conventions: context-first, explicit errors so a
failed distill retries with backoff, a small interface over the chat client for
testability. Keep volatility (staleness, set here) separate from tier
(durability, set on store) — do not merge them.

## Goal

A `distill` job extracts multiple atomic facts from a page's cleaned text, each
with a volatility value and validity horizon, and stores them into memory.

## How to Verify

- `distill.go` registers a `distill` handler that, given a scrape/page reference,
  produces multiple atomic facts via the chat client.
- Each fact carries a volatility value and a validity horizon distinct from any
  durability/tier setting.
- Facts are stored via the T026 upsert path (which enqueues embed jobs).
- `distill_test.go` with a stubbed chat client asserts: multiple facts parsed
  from one page, volatility+horizon captured, facts handed to the store, and that
  a chat failure returns an error (job retries).
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `distill.go` [NEW]
- `distill_test.go` [NEW]

## Dependencies

T024, T010, T005, T023.
