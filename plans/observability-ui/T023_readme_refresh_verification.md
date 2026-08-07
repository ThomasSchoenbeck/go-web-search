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

## Note added during implementation

**The refresh had to correct facts beyond the UI.** Bringing the README in line
with reality meant fixing four claims that had gone stale or were wrong:

1. **"Vectors live in a dedicated table with the ANN index."** False, and
   `vectors.go` had said so all along: the Rust Turso engine has no
   `libsql_vector_idx`, so similarity search is an exact linear scan. Corrected,
   with the scaling consequence stated under Known limits.
2. **Images "land in `scrape_images`."** That table is gone — images are stored
   inline as JSON on the `scrape_cache` row.
3. **"Type-checked against stub packages" / "SQL executed against real SQLite
   3.45" / "Not verified: any of it running."** All three predate the test
   harness. There is no stubs directory, the dependencies are real, and the Go
   tests run against a real Turso database while Playwright drives the compiled
   binary. Replaced with what is actually true, and with a precise statement of
   what remains unverified (a full `-mode serve` run: Chrome, the job runner and
   a live model endpoint).
4. **SIGINT "writes `urls.txt`."** Nothing writes `urls.txt` any more; that
   described the removed one-shot search mode.

The file map was a third of the source files; it now covers all of them, grouped
by subsystem.

**Verification actually performed** (recorded in the README's Verification
status section):

- Built the documented way — `task web`, then `go build .`.
- Ran the binary over a throwaway data directory and issued 27 HTTP checks with
  **no `Authorization` header**: the SPA shell and its hashed assets, three deep
  links through the SPA fallback, every read endpoint the UI uses (including the
  filtered forms of jobs and logs), and two negative cases proving an
  unregistered `/api/...` stays a 404 rather than returning `index.html`.
  All 27 passed; the temp directory was removed afterwards.
- Confirmed the build-ordering rule **by behaviour**: rebuilt with `web/dist`
  reduced to its committed `.gitkeep`, and the binary served the 503 "not built"
  notice while `/api/stats` and `/healthz` kept answering 200. The real build was
  restored immediately.
- Polling with its UI controls is covered by the Playwright specs against the
  real binary (start paused per config, interval dropdown, toggle, and no
  requests after leaving the view).

**Not verified, and the README says so:** a full `-mode serve` run. It launches
real Chrome and needs a reachable llama.cpp endpoint, so browser-driven search,
live scraping, distillation and real embeddings remain covered by unit tests
only. `-mode testserve` deliberately starts no browser, job runner or vector
boot.

**One inconsistency found, then fixed on request:** `config.go` defaulted
`server.addr` to `:8081`, while the shipped `config.toml` and the Vite dev proxy
both used `:8082` — so with no config file present the dev server proxied `/api`
to a port nothing was listening on. The compiled default is now `:8082`, since
every other source of truth already said so (including vite.config.ts's own
comment, which described the default it did not actually have). A new
`config_test.go` pins all three together; it was confirmed to fail on drift
before being kept.

## Files to Touch

- `README.md`

## Dependencies

All prior tasks (T001–T026).
