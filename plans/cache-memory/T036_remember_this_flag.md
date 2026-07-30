---
feature: MCP/REST Surface
task_number: 036
description: "Remember this" flag (enum short|long|permanent, default on)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 036: "Remember this" flag

## Description

Add the "remember this" flag to the memory store surface (T035). It is an enum —
`short`, `long`, or `permanent` — that selects the INITIAL tier (durability) of a
stored fact, and it is on by default (storing remembers at a default tier from
T006 unless told otherwise). This is the durability axis's entry point: the flag
sets the starting tier ceiling that sliding expiry and promotion (T007/T029) then
operate on. It is explicitly NOT the volatility/freshness axis — those come from
the distiller (T025) — so the two must not be conflated in the request shape or
the stored row.

Wire the flag through `memory.store` (and any place facts are stored on a
caller's explicit request) into the tier field of `memory_facts` (T024) via the
tier helper (T007). Validate the enum and fall back to the configured default
when omitted. Update the tool/endpoint request shape and MCP description in
`server.go`.

## Goal

`memory.store` accepts a `short|long|permanent` remember flag that sets a fact's
initial tier, defaults to on with the configured default tier, and is kept
separate from volatility.

## How to Verify

- The memory store request exposes a validated `short|long|permanent` enum that
  maps to the stored fact's initial tier.
- Omitting the flag stores at the configured default tier (remember-on-by-
  default); an invalid value is rejected.
- The flag sets tier only; volatility/validity-horizon fields are untouched by
  it.
- Tests assert each enum value sets the expected initial tier, the default
  applies when omitted, and volatility is unaffected.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `server.go`
- `memory.go`
- `server_test.go`

## Dependencies

T035, T024, T007.
