---
feature: Unified Job System
task_number: 008
description: Add jobs table to schema
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 008: Add `jobs` table to schema

## Description

Add the `jobs` table to `schema.sql`. It backs the unified DB-backed job system.
Columns: `id` (TEXT primary key, UUIDv7 like the rest of the schema), `type`
(the job type discriminator, e.g. embed, distill, cleanup, re-embed migration),
`payload` (JSON text), `status` (one of pending, running, done, failed),
`attempts` (integer), `run_after` (RFC3339, for backoff scheduling),
`locked_at` (RFC3339 nullable, set when a worker claims a row), plus the audit
columns `created_at`, `created_by`, `updated_at`, `updated_by`.

Follow existing `schema.sql` conventions: `IF NOT EXISTS`, TEXT UUID primary
keys, RFC3339 UTC timestamps, snake_case. Add indexes that the claim query will
need — at minimum one supporting the "find the next runnable pending job whose
run_after has passed" lookup, and one on `status` for the reaper's stale-running
scan. This task adds schema only; CRUD is T009.

## Goal

`schema.sql` defines a `jobs` table with the fields above, audit columns, and the
indexes needed for claiming and for stale-running scans.

## How to Verify

- `schema.sql` defines `jobs` with `id`, `type`, `payload`, `status`,
  `attempts`, `run_after`, `locked_at`, and the four audit columns, all
  `IF NOT EXISTS`.
- At least two supporting indexes exist (claim lookup and status scan).
- Applying the schema to a fresh SQLite database succeeds.
- `go build ./...` succeeds (schema is embedded).

## Files to Touch

- `schema.sql`

## Dependencies

None.
