---
feature: Foundation — Build, Embed & Serving
task_number: 004
description: Deployment/access model (edge auth, no app auth) + expose non-secret UI config from config.toml
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 004: Access model (edge auth, no app auth) + UI config exposure

## Description

The observability UI is deployed behind an **edge** (reverse proxy / gateway /
trusted network) that enforces authentication, so **the app adds no auth of its
own**: no login screen, no token entry, no bearer handling in the SPA. In the
assumed deployment `server.api_key` is left unset, so `withAuth` in `server.go`
already serves everything tokenless — the SPA shell and every `/api/*` route load
openly and the edge is responsible for access control. The existing
`api_key`/`withAuth` path stays intact and supported for anyone exposing the
binary directly; it is simply not what the UI relies on. This is explicitly not a
hardened public-auth model.

This task therefore does two things, neither of which is a login UI:

1. **Confirm and document the access model.** Verify the no-key serve path
   (shell + `/api/*` reachable without a token), and document the edge-auth
   assumption plus how to re-enable `api_key` for direct exposure (cross-ref the
   README task T001).
2. **Expose non-secret UI settings from `config.toml` to the SPA.** All app
   settings live in `config.toml` (per the externalized-config decision). Add a
   read-only `GET /api/ui-config` that returns only the non-secret UI tunables the
   SPA needs — the default poll interval, whether polling is enabled by default,
   and the projection sample cap — sourced from new config fields. Never expose
   `api_key` or other secrets. The shared API layer (T005) reads this once to seed
   its defaults; the UI can override interval/polling for the moment (T015/T019).

Add the corresponding fields to the config struct (`config.go`) and to
`config.toml` (a dedicated block, e.g. `[observability]`, with sensible defaults —
polling may default to off). Register the route with `writeJSON` like the other
handlers. Read-only: no writes, no secrets.

## Goal

Serve mode with `api_key` unset serves the SPA and `/api/*` openly (auth delegated
to the edge), the `api_key` path still works when set, and a new read-only
`GET /api/ui-config` returns the non-secret UI settings (poll interval default,
poll-enabled default, projection sample cap) from `config.toml` — with no secret
ever exposed.

## How to Verify

- With `api_key` unset: GET `/`, static assets, and `/api/stats` all succeed
  without a token; the SPA has no login/token UI.
- With `api_key` set: the existing `withAuth` behavior is unchanged (bearer
  required on `/api/*`); this task does not add UI to handle that case.
- `GET /api/ui-config` returns the configured poll interval, poll-enabled flag,
  and projection sample cap, and never includes `api_key` or other secrets.
- New config fields exist in `config.go` and `config.toml` with documented
  defaults; changing them in `config.toml` changes the `/api/ui-config` response.
- `go fmt`, `go vet`, `go test` pass; a test covers `/api/ui-config` (correct
  fields, no secret leakage) against an isolated temporary database that is torn
  down afterward.

## Files to Touch

- `server.go`
- `config.go`
- `config.toml`
- `uiconfig_test.go` [NEW]

## Dependencies

T003.
