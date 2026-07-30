---
feature: LLM Provider & Config
task_number: 004
description: Config schema for named model endpoints keyed by role (chat|embed)
created: 2026-07-30
last_updated: 2026-07-30
completed:
status: [ ]
---

# [ ] Task 004: Config schema for role-keyed model endpoints

## Description

Add configuration for LLM provider endpoints to `config.go` and `config.toml`.
The design keeps all provider configuration in `config.toml` only — there are no
CLI flags for it. Model it as a flat list of named, role-keyed endpoints: each
entry has a name, an endpoint URL, a kind, and an optional api_key, and is keyed
by role. The two roles are `chat` (used by distillation and the gate-3
adjudication) and `embed` (used by the embedder). A single OpenAI-compatible
client (T005) is selected per role.

The current environment uses one llama.cpp server at
`http://192.168.178.64:8080` serving both roles, so the default/sample config
should reflect that as a concrete example. Add a new config struct section
(for example `[[llm]]` entries or an `[llm]` table with role keys) following the
existing `BurntSushi/toml` patterns in `config.go`, wire it into the top-level
`Config` struct, and add matching entries to `config.toml`. Respect the existing
"unknown keys are an error" behavior in `loadConfig`.

This task defines and parses the config only; the client that consumes it is
T005.

## Goal

`config.go` defines a role-keyed list of named LLM endpoints (name, endpoint,
kind, optional api_key), it is part of `Config`, and `config.toml` carries the
current llama.cpp example for both chat and embed roles.

## How to Verify

- `config.go` has a new config type for LLM endpoints wired into `Config`.
- `config.toml` defines at least a chat and an embed endpoint pointing at the
  llama.cpp example URL.
- Loading `config.toml` populates the new fields; an unknown key under the LLM
  section still triggers the existing `Undecoded` error.
- A unit test decodes a sample TOML snippet and asserts the endpoints parse with
  correct role, name, endpoint, kind, and api_key.
- `go fmt`, `go vet`, `go test` pass.

## Files to Touch

- `config.go`
- `config.toml`
- `config_test.go` [NEW]

## Dependencies

None.
