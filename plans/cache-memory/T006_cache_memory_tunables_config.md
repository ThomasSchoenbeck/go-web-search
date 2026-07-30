---
feature: LLM Provider & Config
task_number: 006
description: Config for cache/memory/tiering tunables (TTLs, thresholds, gates, remember default)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 006: Config for cache/memory/tiering tunables

## Description

Add a configuration section to `config.go` and `config.toml` for the tunables
the cache, memory, and tiering features need. These are the values that vary and
must not be hardcoded: the per-tier durations/ceilings for sliding expiry
(short, long, permanent), hit-count promotion thresholds, the semantic
similarity threshold for memory upsert and retrieval (around 0.85, configurable),
the confidence-gate settings including whether LLM adjudication (gate 3) is on by
default (it is), the default "remember this" tier, cleanup job intervals, and the
default freshness `max_age` when volatility does not override it.

Follow the existing config patterns: a new struct wired into `Config`, `Duration`
for time values, snake_case TOML keys, sensible defaults in `defaultConfig`, and
matching documented entries in `config.toml`. Keep tier (durability) and
volatility (staleness) settings clearly separated so the two axes are not
conflated. This task only defines and parses config; the helpers and jobs that
consume these values are later tasks (T007, T028, T029, T030, T036).

## Goal

A cache/memory/tiering config section exists with documented defaults in both
`config.go` and `config.toml`, covering tier durations, promotion thresholds,
similarity threshold, gate defaults (gate-3 on), default remember tier, cleanup
intervals, and default freshness max_age.

## How to Verify

- `config.go` defines the new tunables struct wired into `Config` with defaults
  in `defaultConfig`.
- `config.toml` carries the same keys with explanatory comments.
- A unit test decodes a sample snippet and asserts values parse, and that
  omitted keys fall back to defaults.
- Tier settings and the volatility/freshness setting are separate fields, not a
  single merged value.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `config.go`
- `config.toml`
- `config_test.go`

## Dependencies

None.
