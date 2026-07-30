---
feature: Memory, Distillation & Confidence Gating
task_number: 028
description: Confidence gating (similarity, freshness, LLM adjudication)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 028: Confidence gating

## Description

Implement the confidence gate that decides whether retrieved memory (T027) is
good enough to answer a query WITHOUT falling back to a web search. Three gates,
in order, in a new `gate.go`:

1. Similarity gate — the best fact's similarity must exceed the configured
   threshold (around 0.85, from T006).
2. Freshness gate — the fact must be within its `max_age`, where volatility
   (T025) drives the effective max_age (more volatile facts expire faster); this
   is the staleness axis, kept separate from tier/durability.
3. LLM adjudication (gate 3) — a chat call (T005) asks the model whether the
   retrieved facts actually answer the query; default ON (from T006).

Only if all enabled gates pass does the system skip the web search and answer
from memory. Each gate is independently toggleable via config. Follow go-agent
conventions: context-first, a small interface over the chat client for testing,
explicit errors.

## Goal

A gate that passes only when similarity, freshness (volatility-driven max_age),
and (when enabled) LLM adjudication all agree the retrieved facts answer the
query, with gate-3 defaulting on.

## How to Verify

- `gate.go` exposes a context-first gate taking retrieved facts + query and
  returning a pass/fail (with reason).
- Similarity below threshold fails at gate 1; stale-by-volatility fails at gate
  2; a "no, this does not answer it" adjudication fails at gate 3.
- Gate 3 is on by default and can be disabled via config; disabling it skips the
  chat call.
- `gate_test.go` with a stubbed chat client covers each gate's pass and fail
  paths and the all-pass case.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `gate.go` [NEW]
- `gate_test.go` [NEW]

## Dependencies

T027, T005, T007.
