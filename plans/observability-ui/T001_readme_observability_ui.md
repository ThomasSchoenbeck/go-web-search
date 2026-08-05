---
feature: Foundation — Build, Embed & Serving
task_number: 001
description: Document the planned Observability UI (what it is, build step, embedded serving) in README
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 001: Document the planned Observability UI in README

## Description

Add an early, forward-looking section to `README.md` describing the
Observability & Data-Inspection UI this plan introduces, before any of it is
built (the planner's "documentation is mandatory, placed early" rule). The
current README documents modes (browse/serve), the REST/MCP surface, and the
cache/memory system; this task frames the coming UI so the README is honest
about the direction from the start.

The section should describe, in plain terms: that a read-only Svelte single-page
app (Svelte 5 + Vite, client-only SPA — not SvelteKit) is served by the existing
`-mode serve` listener alongside the current REST + MCP routes; that it inspects
runs/searches/SERPs, scrapes, provenance, memory facts, the semantic explorer,
jobs, caches, logs, stats, and (later) an embeddings 2-D projection; that it is
inspection-only in v1 (no action-triggering, no destructive ops); and that the
access model is **edge auth with no app-level auth** — the SPA and `/api/*` are
served openly and authentication is delegated to an edge/reverse-proxy/trusted
network, with the existing `server.api_key` bearer left optional for direct
exposure.

Critically, document the **build model**: the frontend lives under a new `web/`
directory and is built with Node + Vite into `web/dist/`, which is then embedded
into the Go binary with `go:embed`. `web/dist/` is **gitignored and generated at
build time** — it is NOT committed. Therefore a plain `go build` with no prior
`pnpm build` will fail to find (or will embed an empty) `dist/`; the Node/Vite
build step must run BEFORE `go build` in CI/local builds. State that Node + Vite
are new build-time dependencies.

This is a documentation-only task. It sets expectations; T023 refreshes the
README to match final behavior.

## Goal

`README.md` contains a new section (for example "Observability UI (planned)")
that a new reader can use to understand what the project is growing into: the
embedded read-only Svelte SPA served on the serve listener, the views it will
offer, the Svelte 5 + Vite client-only SPA choice, the edge-auth (no app auth) model,
and — explicitly — that `web/dist/` is gitignored and must be built by a Node/Vite
step (wired via `Taskfile.yaml`) before `go build` embeds it.

## How to Verify

- `README.md` renders with a clearly labeled new section covering: the embedded
  read-only SPA on the serve listener, the list of planned views, read-only v1
  scope, and the edge-auth (no app-level auth) model.
- The section explicitly states that `web/dist/` is gitignored/generated and that
  the Node/Vite build must precede `go build` (a bare `go build` without a prior
  build embeds nothing/fails).
- Node + Vite are named as new build-time dependencies.
- No source files change; `go build ./...` still succeeds unchanged.

## Files to Touch

- `README.md`

## Dependencies

None.
