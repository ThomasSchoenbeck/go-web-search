---
feature: Search Cache
task_number: 018
description: Add search_cache table
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 018: Add `search_cache` table

## Description

Add the `search_cache` table to `schema.sql`. It maps a normalized query to the
set of result URLs found for it, so a repeated query can be answered without
hitting the engines. It is a vectored store: the query is embedded (deferred, via
T015) and its vector lives in the dedicated `vectors` table (T014), referenced by
this row's id — there is no embedding column on `search_cache` itself.

Columns: `id`, the normalized query text (with a unique constraint so exact
lookups are cheap), the result URLs (JSON text or a child table — pick the
simpler fit and note the choice), tier and expiry fields for sliding expiry
(T029), a hit_count for promotion, and the audit columns. Follow `schema.sql`
conventions: `IF NOT EXISTS`, TEXT UUID primary keys, RFC3339 timestamps,
snake_case. Add an index for the exact normalized-query lookup.

This task is schema only; store/lookup logic is T019 and the semantic lookup is
T020.

## Goal

`search_cache` exists with a normalized query key, result URLs, tier/expiry/
hit_count fields, audit columns, and an index supporting exact lookup; its vector
lives in the `vectors` table, not on this row.

## How to Verify

- `schema.sql` defines `search_cache` with the fields above, `IF NOT EXISTS`,
  and a unique index on the normalized query.
- There is no embedding column on `search_cache`; vectors reference it via
  `vectors` (T014).
- Applying the schema to a fresh SQLite database succeeds.
- `go build ./...` succeeds.

## Files to Touch

- `schema.sql`

## Dependencies

T014.
