---
feature: Foundation — Build, Embed & Serving
task_number: 024
description: Frontend test harness — Vitest unit + Playwright e2e, isolated test DB, teardown fixtures
created: 2026-08-05
last_updated: 2026-08-05
completed:
status: [ ]
blocker_note:
---

# [ ] Task 024: Test harness — Vitest + Playwright + isolated DB & teardown

## Description

Stand up the shared testing infrastructure the whole plan depends on, once, so
every later view and endpoint task can add its own tests without re-inventing
setup. All tooling is pinned to exact versions and passes the same `pnpm audit`
/ CVE / supply-chain gate as T002. This task sets up the harness only; the
per-feature tests themselves live in their own tasks (each view and endpoint
task lists its unit/e2e tests in its How-to-Verify).

All harness code and tests are **TypeScript** (`.ts` configs and specs), pinned to
exact versions and passing the audit gate. Two layers:

- **Unit (Vitest).** Add Vitest (TypeScript config, `vitest.config.ts`) with the
  Svelte testing library so component and module logic can be unit-tested. Provide
  a `pnpm test` script. Unit test files are colocated with the code as `*.test.ts`
  (e.g. `web/src/lib/api.test.ts`, `web/src/views/RunsList.test.ts`). The standard
  is: every non-trivial function or component behavior gets a unit test. Type-
  checking (`svelte-check`/`tsc`, from T002's `check` script) covers the tests too.
- **End-to-end (Playwright).** Add Playwright (`playwright.config.ts`) with a
  config that boots the app and drives real pages. E2E specs live under
  `web/tests/e2e/*.spec.ts`. Provide a `pnpm test:e2e` script. The standard the
  plan holds every page to: each page is loaded and **every interactive element —
  every button, link, and input — is exercised** at least once.

**Isolated databases and full teardown (hard requirement).** Tests must never
touch a developer's or production data. The harness must:

- Bring up the Go server against a **throwaway database created per test run**
  (a temp main DB and a temp log DB), seeded with a small fixture dataset, and
  **delete all of it on completion** — pass, fail, or interrupt. No residual test
  data, no shared DB file.
- Expose this as reusable fixtures: a Playwright global-setup/teardown (or
  per-worker fixture) that starts the server on an ephemeral port pointed at the
  temp DBs and tears everything down afterward, and a matching helper for Go
  endpoint tests (temp DB + cleanup) so backend tests follow the same
  isolate-and-teardown rule.
- Make the server's data directory / DB paths overridable for tests (via config
  or flags the harness already supports) so the temp location can be injected.

Document how to run the suites (`pnpm test`, `pnpm test:e2e`, `go test ./...`)
and the isolation guarantees. Wire the three suites into the build/CI ordering
so they run in the pipeline (T023 covers the final README/CI pass).

## Goal

A pinned, audited Vitest + Playwright harness exists with `pnpm test` and
`pnpm test:e2e` scripts and reusable fixtures that run every test against an
isolated throwaway database (main + log), seed a fixture dataset, and remove all
test data on completion — so subsequent view and endpoint tasks only add their
own specs.

## How to Verify

- `pnpm test` runs Vitest (with at least one example unit test) and passes; the
  test tooling is pinned to exact versions in `web/package.json` and clears
  `pnpm audit`.
- `pnpm test:e2e` runs Playwright against the app, using a fixture that starts
  the Go server on an ephemeral port pointed at temp DBs; an example spec loads a
  page and asserts on it, and passes.
- After a run (including a forced failure/interrupt), no temp DB files or test
  rows remain — teardown removed the throwaway main DB and log DB entirely; no
  shared/production DB was touched.
- A Go endpoint test using the shared temp-DB helper runs under `go test ./...`
  and cleans up after itself.
- Running the suites requires no manual DB setup; paths are injected by the
  fixtures.

## Files to Touch

- `web/package.json`
- `web/vitest.config.ts` [NEW]
- `web/playwright.config.ts` [NEW]
- `web/tests/e2e/fixtures.ts` [NEW]
- `web/tests/e2e/smoke.spec.ts` [NEW]
- `web/src/lib/example.test.ts` [NEW]
- `testsupport.go` [NEW]

## Dependencies

T002, T003, T005.
