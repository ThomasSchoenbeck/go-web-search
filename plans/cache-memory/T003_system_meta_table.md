---
feature: Foundation & Documentation
task_number: 003
description: Add system_meta key/value table (active embed model+dim, migration state)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 003: Add `system_meta` key/value table

## Description

Introduce a small key/value table, `system_meta`, in the main schema. It holds
process-level state that must survive restarts, most importantly the currently
active embedding model name and dimension, and the re-embed migration state
(idle vs in-progress). The boot-time model/dim check (T017) compares the
configured embed model against the values recorded here to decide whether a
blue/green re-embed migration is needed.

Follow the existing schema conventions in `schema.sql`: `IF NOT EXISTS` so it
doubles as a migration on a fresh file, `id TEXT PRIMARY KEY`, RFC3339 UTC
timestamps, snake_case columns, and the audit columns
(`created_at`, `created_by`, `updated_at`, `updated_by`) per the go-agent
convention. A key/value shape (unique `key`, `value` text) is the simplest fit;
explain in the task that a typed table is unnecessary because the set of keys is
tiny and read rarely.

Add minimal accessor helpers (get value by key, set/upsert value by key) so
later tasks can read and write meta without duplicating SQL. Place them in a new
`meta.go` file to keep `store.go` focused on the harvest tables.

## Goal

`system_meta` exists in the schema with audit columns, and Go helpers can read
and upsert a value by key.

## How to Verify

- `schema.sql` defines `system_meta` with `IF NOT EXISTS`, a unique `key`, a
  `value` column, and the four audit columns.
- Applying the schema to a fresh SQLite database creates the table without
  error.
- A unit test in `meta_test.go` upserts a key, reads it back, overwrites it, and
  confirms the new value and an updated `updated_at`.
- `go fmt`, `go vet`, and `go test` pass.

## Files to Touch

- `schema.sql`
- `meta.go` [NEW]
- `meta_test.go` [NEW]

## Dependencies

None.
