# Handover — memory retrieval rework

**Session:** 2026-09-30 · **Branch:** `ui-design` · **Baseline commit:** `d46d614` ("first pass, half done")
**Status:** all changes staged, uncommitted. `go build ./...`, `go vet ./...`, `go test ./...` all pass.

> ⚠️ **This codebase is an OLDER snapshot than the running prototype.** The prototype
> has features this baseline lacks (`llm.chat_concurrency`, `retention.log_max_age`,
> `retention.log_keep_last`, a Gemma-based chat model). When you rebase/port this work
> onto the latest, those come from the *other* side — see [Sync notes](#sync-notes).

---

## TL;DR

Two things, both about how `memory_query` behaves:

1. **Debug flag** — a miss used to collapse to `found: false` with no signal. `debug: true`
   now returns a full breakdown of which gate stopped the query, per-fact similarity
   (including near-misses the threshold would have discarded), and the adjudicator's raw reply.
2. **Retrieval-first** — `memory_query` now returns the *facts* that clear the mechanical
   gates (similar + fresh), each with its similarity, and lets the **caller** synthesize.
   The LLM synthesis step (gate 3) is off by default and, when on, only *upgrades* the answer
   — it never withholds the facts. Plus a new **`memory_explore`** MCP tool exposing the raw
   nearest-neighbour probe that was previously REST-only.

**The guiding decision:** retrieval is the reliable primitive; synthesis is the risky part.
Keep the mechanical gates (similarity, freshness); stop letting a small chat model veto
facts that already cleared them.

---

## Changed files (7, all staged)

| File | What changed |
|---|---|
| `gate.go` | Debug types + `MemoryAnswer` rework (both changes live here) |
| `memory.go` | `RetrieveFacts` → `RetrieveCandidates` (no threshold filter) |
| `llm.go` | New `EmbedQueryPrefix()` accessor |
| `server.go` | Debug request/response fields; new `ExploreRequest` + `mcpExplore` + `memory_explore` tool; updated `memory_query` tool description |
| `resolver.go` | `MemoryAnswer` call updated for the new signature (`debug=false`) |
| `config.go` | `Gate3Enabled` default `true` → `false` |
| `config.toml` | `gate3_enabled` `true` → `false` |

(Incidental: `.claude/settings.json` is also staged — tooling/permissions, not part of this feature.)

---

## Change 1 — Debug flag

A memory miss gave no reason. This surfaces exactly where it stopped.

- **`gate.go`** — new types `MemoryDebug`, `DebugCandidate`, `DebugAdjudication` + helper
  `debugCandidates()`. `MemoryAnswer` gains a `debug bool` parameter and a `*MemoryDebug`
  return value. On every early exit it stamps `StoppedAt` with one of
  `table` / `embed` / `gate1_similarity` / `gate2_freshness` / `ok`. It also records:
  the vectors table + whether a re-embed migration is in flight (`Migrating`), the exact
  text the query was embedded as (`QueryEmbedded`, query_prefix applied), the thresholds,
  and — crucially — **all k nearest candidates with their similarity, including the ones below
  the threshold**, plus per-candidate `passes_threshold` and `fresh`.
- **`memory.go`** — `RetrieveFacts(ctx, mem, table, vec)` (which filtered on the threshold and
  `break`-ed at the first miss) is replaced by
  `RetrieveCandidates(ctx, table, vec, k)`: returns the k nearest facts with cosine
  similarity `1 - distance` and **no threshold filtering**, so a near-miss is visible.
  The threshold now lives in `gate.go` (gate 1 loop) instead of the store.
- **`llm.go`** — `EmbedQueryPrefix()` so the debug view can show the exact embedded text.
- **`server.go`** — `MemoryQueryRequest` gains `Debug bool`; `MemoryQueryResponse` gains
  `Debug *MemoryDebug`. `runMemoryQuery` threads `req.Debug` through and sets `resp.Debug`.
  Exposed on **both** REST (`POST /api/memory/query`) and MCP (`memory_query` tool).
- **`gate.go`** — a `debug` query is **read-only**: it skips `recordMemoryHits`, so probing
  cannot promote a fact or slide its expiry.

## Change 2 — Retrieval-first + `memory_explore`

- **`gate.go`** — `MemoryAnswer` gate-3 section rewritten. `ok` is now decided by **gates 1–2
  alone**. `answer` defaults to `joinFacts(fresh)` (the raw facts, one per line). Gate 3
  (`mem.Gate3Enabled`) is now purely an upgrade:

  ```go
  answer = joinFacts(fresh)
  if mem.Gate3Enabled {
      if syn, sufficient, modelOut, aerr := adjudicate(ctx, llm, question, fresh); aerr == nil {
          if sufficient {
              answer = syn          // upgrade only — facts are still returned in `used`
          }
          ...
      } // on aerr: fall back to the facts, never fail the query
  }
  ```

  `adjudicate()` now also returns the model's **raw reply** (for the debug view).
- **`config.go` / `config.toml`** — `gate3_enabled` flipped to `false`. Synthesis is opt-in.
- **`server.go`** — new `ExploreRequest{Text, K}` and `mcpExplore` wrapping the existing
  `store.Explore` (raw top-k across **memory facts AND cached searches**, no gating, no
  freshness filter, each neighbour tagged `owner_kind`). Registered as the **`memory_explore`**
  MCP tool. The `memory_query` tool description was updated to say it returns the facts for the
  caller to synthesize.

---

## API / behavior surface (what matters when merging)

- **`MemoryAnswer` signature changed** — now
  `(answer, used []MemoryFact, dbg *MemoryDebug, ok, err)`. One caller: `resolver.go:55`
  (passes `debug=false`).
- **MCP tools:** `memory_query` unchanged in name; `memory_explore` is **new**
  (input `{text, k}` → `ExploreResult`).
- **`memory_query` response:** `answer` is now the *joined facts* by default (was a
  synthesized answer, or empty when gate 3 vetoed). `debug` field is new.
- **`resolver.go` `web_search` memory branch** now fires more readily — a hit returns the
  joined facts (source `"memory"`) instead of falling through to cache/live when the
  adjudicator said INSUFFICIENT.
- **Config key `gate3_enabled`** default is now `false` (set `true` to get LLM-synthesized
  answers back as an upgrade).

## Verification

`go build ./...` ✓ · `go vet ./...` ✓ · `go test ./...` → `ok go-web-search` ✓
(No test references the MCP tool list or `MemoryAnswer` internals, so the new tool and the
signature change broke nothing.)

## Sync notes

- **This baseline is behind the prototype.** The running prototype's `config.toml` carries
  keys this build does not declare — `llm.chat_concurrency`, `retention.log_max_age`,
  `retention.log_keep_last`. The loader is **strict** (`config.go` rejects any unknown key),
  so running *this* binary against the prototype's config fails at startup with
  `unknown keys [...]`. Reconcile by either porting those three features into this tree or
  removing the keys from the config you run against.
- **Chat model:** the prototype uses a smaller Gemma E4B for distillation/chat; it under-extracts
  facts vs. the Qwen model. This is a **separate open issue** (extraction prompt/model tuning),
  not touched here.

## Follow-ups (not done)

- **UI "Memory" view** in the SPA (`web/`) to run/explore memory queries and browse facts in
  the browser — the natural home for the new debug + explore output. Hinted, not built.
- **Entity-aware retrieval** — the *real* fix for "give me every fact about person X" (the
  single-query-vector + hard 0.85 threshold + `top_k=8` design is a poor fit for it). Deferred.
