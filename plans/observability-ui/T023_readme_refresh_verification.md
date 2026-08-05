---
feature: Documentation & Verification
task_number: 023
description: README refresh (final UI, build, embedded serving) + end-to-end verification
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 023: README refresh + end-to-end verification

## Description

Final documentation and cross-cutting verification pass. Once all prior tasks
land, refresh `README.md` to describe the Observability UI as it actually behaves
(not the forward-looking T001 note), and verify the embedded binary end to end.

**README refresh:** document the built UI and every view (runs/searches/SERPs,
scrapes, provenance — URL pivot + whole-run causality graph, memory facts,
semantic explorer, jobs, caches, logs, stats, projection); the new read-only
`/api/*` endpoints added by this plan (provenance, run-causality, `/api/ui-config`,
semantic explorer, jobs, cache browsers, logs query, projection) and the extended
`/api/stats` (model+dim+migration, cache hit rates + tiers, job throughput); the
**access model (edge auth, no app-level auth)**; and config-driven polling with
live UI overrides. Update the file map (`web/`, the new Go files) and the build
instructions.

**Build steps (bake in the confirmed decision):** document the exact ordering —
`web/dist/` is **gitignored and generated**, so the build is `pnpm install` +
`pnpm build` in `web/` FIRST, THEN `go build`, wired as a `Taskfile.yaml` target
that chains them; a bare `go build` with no prior Vite build embeds nothing. Note
Node + Vite as build-time dependencies, the Svelte 5 + Vite client-only SPA
choice, and the client-side **PCA** projection (no Go dep).

**End-to-end verification:** with a serve binary built the documented way (via the
Taskfile target), confirm the SPA loads with `api_key` unset (shell + `/api/*`
served openly; auth is the edge's job), every view renders against real/seeded
data, and jobs/logs polling refreshes and honors the interval/enable UI controls.
Record the checks in the README's verification section (mirroring the existing
"Verification status" section).

## Goal

`README.md` matches the final UI, its endpoints, the access model (edge auth), and
the exact build ordering (Taskfile-driven Vite build before `go build`), and an
end-to-end pass confirms the embedded binary serves the SPA openly (auth at the
edge), renders all views, and polls jobs/logs with working UI controls.

## How to Verify

- `README.md` documents all views, the new/extended endpoints, the access model
  (edge auth, no app auth), config-driven polling, the `web/` + new-Go-file map,
  and the Taskfile build ordering (`pnpm build` → `go build`; `dist/`
  gitignored/generated), plus the Svelte 5 + Vite SPA and client-side PCA
  projection decisions.
- Building the documented way (the Taskfile target: `pnpm build` then `go build`)
  yields a binary that serves the SPA; a bare `go build` with no prior Vite build
  is called out as producing no embedded UI.
- A manual/scripted pass confirms: with `api_key` unset, the shell and `/api/*`
  are served openly (edge auth); each view renders; jobs and logs views refresh via
  polling and respond to the interval/enable controls.
- `go fmt`, `go vet`, `go test ./...` pass; the frontend builds and lints clean.

## Files to Touch

- `README.md`

## Dependencies

All prior tasks (T001–T026).
