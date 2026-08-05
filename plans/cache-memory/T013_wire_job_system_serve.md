---
feature: Unified Job System
task_number: 013
description: Wire job system into serve-mode lifecycle (start/stop)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 013: Wire the job system into serve-mode lifecycle

## Description

Start the job runner (poller + worker pool + reaper + recurring jobs) when serve
mode starts, and stop it cleanly when serve mode shuts down. Serve mode is
launched from `serveWithBrowser`/`serveMode`; the runner should start after the
store is open and stop as part of the graceful shutdown alongside the HTTP
server. Because all search/scrape now runs under serve (one-shot modes removed,
T038), the queue is always drained by the running process.

This depends on T002 having raised `max_open_conns` so the poller and workers get
real concurrent connections. Construct the runner with the registered handlers
and recurring jobs, tie its context to the serve-mode `stop` context, and ensure
shutdown cancels the runner and waits for in-flight workers before the process
exits. Log that the job system started and stopped.

## Goal

Serve mode starts the job runner on boot and stops it on shutdown, with in-flight
jobs allowed to finish, using the raised connection limit from T002.

## How to Verify

- Starting `-mode serve` logs that the job system started; the poller runs.
- SIGINT/SIGTERM shuts down the HTTP server and the job runner together; the
  process exits without leaking goroutines.
- The runner is constructed with the connection concurrency enabled by T002.
- An integration-style test (or manual run) confirms an enqueued job is picked up
  while serve mode is running and that shutdown cancels cleanly.
- `go fmt`, `go vet`, and existing tests pass.

## Files to Touch

- `main.go`
- `server.go`

## Dependencies

T010, T011, T012, T002.
