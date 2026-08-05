---
feature: LLM Provider & Config
task_number: 005
description: OpenAI-compatible HTTP client (chat completions + embeddings)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 005: OpenAI-compatible HTTP client

## Description

Build one OpenAI-compatible HTTP client, in a new `llm.go` file, that talks to
the endpoints configured in T004. It exposes two operations: chat completions
(used by distillation and gate-3 adjudication) and embeddings (used by the
embedder). It is constructed from the role-keyed endpoint config, sends the
optional api_key as a bearer token when present, and targets the standard
OpenAI-compatible paths that llama.cpp serves.

Follow go-agent conventions: standard library `net/http` and `encoding/json`
only (no SDK), context-first method signatures, explicit error handling, small
consumption-site interfaces so callers (distill, embed, gate) can depend on a
narrow surface and tests can stub it. Requests must honor context cancellation
and a timeout. Keep request/response DTOs internal to this file.

This task delivers the transport and DTOs. The embedder worker (T015),
distiller (T025), and gate (T028) are separate tasks that call into it.

## Goal

A single OpenAI-compatible client can perform chat completions and embeddings
against the configured role endpoints, with bearer auth when an api_key is set,
context-aware and timeout-bounded.

## How to Verify

- `llm.go` defines a client constructed from the T004 config with chat and embed
  methods, each taking `context.Context` first.
- A unit test using `httptest.Server` asserts: chat and embed requests hit the
  expected paths, send `Authorization: Bearer` only when an api_key is set,
  encode the request body correctly, and decode a mocked response.
- Context cancellation aborts an in-flight request in a test.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `llm.go` [NEW]
- `llm_test.go` [NEW]

## Dependencies

T004.
