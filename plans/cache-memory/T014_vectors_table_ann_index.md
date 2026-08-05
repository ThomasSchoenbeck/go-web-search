---
feature: Vector Store, Embeddings & Migration
task_number: 014
description: Dedicated vectors table + ANN index (Turso native vector)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 014: Dedicated vectors table + ANN index

## Description

Create a single dedicated `vectors` table that holds every embedding in the
system, rather than putting embedding columns on the data tables. Each vector row
has: `id`, a foreign key to the owning row plus a discriminator for which store
it belongs to (search_cache or memory_facts — scrape_cache is NOT vectored), the
embedding itself as a Turso native vector column, the `model` name and `dim` it
was produced with, and the audit columns. The ANN index lives on this table.
Vectors must cascade-delete when their owner is deleted (T030 relies on this).

This is the one place committed fully to Turso native vector search — there is no
fallback vector implementation. The exact native vector column type and ANN index
syntax are pre-1.0 and have moved recently (see the `store.go` driver comment and
the `database.driver` config); VERIFY the current Turso vector/ANN syntax against
current Turso docs before writing the DDL, and match the installed
`turso.tech/database/tursogo` version. Add the table and index to `schema.sql`
and add a small `vectors.go` with insert/delete-by-owner helpers.

## Goal

A dedicated `vectors` table with a Turso native vector column, an ANN index, per
-row `model`+`dim` stamping, an owner FK with a store discriminator, and cascade
delete on owner removal, plus Go helpers to insert and delete vectors.

## How to Verify

- Turso native vector column type and ANN index syntax are confirmed against
  current Turso docs and the installed driver version before implementation
  (note the verified syntax in the task's implementation notes).
- `schema.sql` defines `vectors` with the native vector column, `model`, `dim`,
  owner FK + store discriminator, audit columns, and an ANN index.
- Deleting an owner row removes its vector rows (cascade verified in a test, or
  enforced by an explicit delete helper if native cascade is unavailable).
- `vectors.go` insert/delete helpers are context-first and covered by a test
  against the running Turso driver (or documented SQLite-limited coverage where
  native vectors are Turso-only).
- `go fmt`, `go vet` pass.

## Files to Touch

- `schema.sql`
- `vectors.go` [NEW]
- `vectors_test.go` [NEW]

## Dependencies

T003.
