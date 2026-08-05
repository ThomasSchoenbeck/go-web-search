---
feature: Foundation & Documentation
task_number: 002
description: Revisit database concurrency (raise max_open_conns) for the job system
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 002: Revisit database concurrency (raise `max_open_conns`)

## Description

Today `max_open_conns` defaults to `1` in both `config.go` (`defaultConfig`) and
`config.toml`, which serialises all database writes. That was the conservative
choice while the Turso Go bindings are beta. The unified job system introduced
later in this plan runs a poller goroutine plus a worker pool that need genuine
concurrent database access, so a single connection would bottleneck (and could
deadlock) the queue.

Raise the default to a small, sensible value that gives the job system real
concurrency while staying safe for a local single-file Turso database. Update
the inline comments in both `config.go` and `config.toml`, and the "Database
writes" row/notes in `README.md`, so the reasoning is documented rather than
silently changed. The `store.go` header comment that explains the single
connection also needs to be reconciled with the new default.

Do not change how connections are opened (`openStore` already calls
`SetMaxOpenConns`); this task is about the default value and its documentation.

## Goal

The default `max_open_conns` is raised above `1` in both the Go defaults and the
sample config, with comments explaining that the job system needs concurrency
and noting the Turso beta caveat.

## How to Verify

- `defaultConfig` in `config.go` returns a `MaxOpenConns` greater than 1.
- `config.toml` sets `max_open_conns` to the same new value with an updated
  comment.
- The `store.go` package/type comment no longer claims writes are always
  serialised, or is reworded to reflect the configurable default.
- `README.md` Concurrency section reflects the new default.
- `go build ./...` and `go vet ./...` pass; existing tests still pass.

## Files to Touch

- `config.go`
- `config.toml`
- `store.go`
- `README.md`

## Dependencies

None.
