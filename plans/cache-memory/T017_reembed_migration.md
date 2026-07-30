---
feature: Vector Store, Embeddings & Migration
task_number: 017
description: Re-embed migration (blue/green) + boot-time model/dim check
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 017: Re-embed migration (blue/green) + boot-time model/dim check

## Description

At boot, compare the configured embed model+dim (T004/T006) against the active
values recorded in `system_meta` (T003). If they differ, a re-embed migration is
required: every stored vector was produced with a now-outdated model/dim and must
be rebuilt. Implement this as a blue/green migration — enqueue re-embed work (a
job type, reusing the embed worker path) to rebuild vectors under the new model
while the old vectors remain readable, and flip `system_meta` to the new model
only when zero stale vectors remain.

While the migration runs, `system_meta` marks it in progress; the vector search
helper (T016) reports semantic unavailable, so the search cache degrades to exact
query match and memory misses to the web until vectors are rebuilt. Register the
migration check as a managed startup/recurring behavior (T012) so it runs on boot
and can resume if interrupted. Put migration orchestration in a new
`reembed.go`.

The default is Qwen3-Embedding-8B at dim 4096; Matryoshka truncation is deferred,
but because vectors are stamped per row (T014) a future model/dim change is
handled by exactly this path.

## Goal

On boot, a model/dim mismatch between config and `system_meta` triggers a
blue/green re-embed that rebuilds all vectors and flips the active model only when
no stale vectors remain, marking the migration in progress meanwhile.

## How to Verify

- Boot with config model/dim matching `system_meta`: no migration runs.
- Boot with a changed model/dim: `system_meta` is marked in-progress, re-embed
  jobs are enqueued, and old vectors stay readable during the run.
- When the last stale vector is rebuilt, `system_meta` flips to the new
  model/dim and the in-progress flag clears; T016 then reports semantic available
  again.
- A test drives a simulated model change end to end (stubbed embedder) and
  asserts the state transitions and that stale-vector count reaches zero before
  the flip.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `reembed.go` [NEW]
- `reembed_test.go` [NEW]

## Dependencies

T014, T015, T016, T003, T012.
