---
feature: Vector Store, Embeddings & Migration
task_number: 015
description: Embed job type + worker (asymmetric prefixes, stamp model+dim)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 015: Embed job type + worker

## Description

Add the `embed` job type and its handler, registered with the job runner (T010).
The worker takes a job payload identifying an owner row (which store, which id,
and the text to embed plus its kind), calls the embeddings endpoint through the
LLM client (T005), and writes the resulting vector into the `vectors` table
(T014), stamping the `model` and `dim` it used.

The embedding model is Qwen3-Embedding-8B served via llama.cpp, full dim 4096.
Use asymmetric prefixes: facts and cached documents are embedded as documents,
query text is embedded as a query. The worker must apply the correct prefix for
the kind of text it is embedding. Deferred embedding is the pattern: data rows
are stored with a null vector and an embed job enqueued; this worker fills the
vector in later, so exact-match works immediately and semantic catches up.

Put the worker in a new `embed.go`. Follow go-agent conventions: context-first,
explicit errors (a failed embed returns an error so the job retries with
backoff), small interfaces.

## Goal

An `embed` job handler embeds an owner row's text with the correct asymmetric
prefix and stores the vector stamped with model and dim.

## How to Verify

- `embed.go` registers an `embed` handler with the runner and, given a payload,
  produces and stores a vector via T014.
- The document vs query prefix is chosen by the text kind in the payload.
- The stored vector row carries the model name and dim (4096 for the default
  model).
- `embed_test.go` with a stubbed LLM client asserts: correct endpoint call,
  correct prefix applied, vector stored with model+dim, and that an embed failure
  returns an error (so the job retries).
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `embed.go` [NEW]
- `embed_test.go` [NEW]

## Dependencies

T014, T010, T005.
