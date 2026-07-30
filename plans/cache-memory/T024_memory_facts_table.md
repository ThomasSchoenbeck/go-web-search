---
feature: Memory, Distillation & Confidence Gating
task_number: 024
description: Add memory_facts table
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 024: Add `memory_facts` table

## Description

Add the `memory_facts` table to `schema.sql`. It stores atomic facts distilled
from scraped pages. It is a vectored, tiered store: each fact is embedded
(deferred, via T015) with its vector in the dedicated `vectors` table (T014),
referenced by the fact's id — no embedding column on this table.

Columns: `id`, the fact text, a `volatility` value (staleness rate emitted by the
distiller, T025 — the freshness axis), a validity horizon (when the fact should
be considered stale), a `source_url` / provenance reference back to where it came
from, tier and expiry fields for sliding expiry (T029 — the durability axis),
hit_count for promotion, and the audit columns. Keep volatility (staleness) and
tier (durability) as separate columns; they are orthogonal axes. Follow
`schema.sql` conventions: `IF NOT EXISTS`, TEXT UUID primary keys, RFC3339
timestamps, snake_case, and add indexes the retrieval and cleanup paths need.

This task is schema only; distillation is T025, upsert T026, retrieval T027.

## Goal

`memory_facts` exists with fact text, separate volatility and validity-horizon
fields, tier/expiry/hit_count, provenance, and audit columns; its vector lives in
`vectors`, not on this row.

## How to Verify

- `schema.sql` defines `memory_facts` with the fields above, `IF NOT EXISTS`,
  and no embedding column.
- Volatility and tier are distinct columns.
- Applying the schema to a fresh SQLite database succeeds.
- `go build ./...` succeeds.

## Files to Touch

- `schema.sql`

## Dependencies

T014.
