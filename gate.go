package main

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// volatilityMaxAge maps a fact's volatility label to how long it may be trusted
// before a freshness gate rejects it, independent of the durability tier. A
// stable fact returns 0, meaning only its tier expiry governs.
func volatilityMaxAge(v string) time.Duration {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "time-sensitive", "volatile", "time_sensitive":
		return 24 * time.Hour
	default:
		return 0
	}
}

// MemoryDebug is the diagnostic breakdown returned when a memory query runs with
// debug=true. A miss otherwise collapses to found:false with no signal; this
// explains it — which gate stopped the query, how close the nearest fact was,
// and what the adjudicating model actually said.
type MemoryDebug struct {
	// The gate the query stopped at: "ok" on success, else one of
	// "table", "embed", "gate1_similarity", "gate2_freshness",
	// "gate3_adjudication".
	StoppedAt string `json:"stopped_at"`
	// The vectors table searched, and whether a re-embed migration is in flight
	// (which degrades every semantic read regardless of the query).
	Table     string `json:"table,omitempty"`
	Migrating bool   `json:"migrating,omitempty"`
	// The exact text sent to the embed model for the query (query_prefix applied).
	QueryEmbedded string `json:"query_embedded,omitempty"`
	// The thresholds the run was judged against, so the candidate scores below
	// are interpretable.
	SimilarityThreshold float64 `json:"similarity_threshold"`
	TopK                int     `json:"top_k"`
	// The k nearest facts, nearest first, INCLUDING those below the threshold —
	// so a near-miss shows exactly how far short it fell.
	Candidates []DebugCandidate `json:"candidates,omitempty"`
	// Gate 3 detail, present only once the query reached adjudication.
	Adjudication *DebugAdjudication `json:"adjudication,omitempty"`
}

// DebugCandidate is one retrieved fact with the numbers that decided its fate.
type DebugCandidate struct {
	Text            string  `json:"text"`
	SourceURL       string  `json:"source_url,omitempty"`
	Volatility      string  `json:"volatility,omitempty"`
	Similarity      float64 `json:"similarity"`
	PassesThreshold bool    `json:"passes_threshold"`
	Fresh           bool    `json:"fresh"`
	FetchedAt       string  `json:"fetched_at,omitempty"`
	ExpiresAt       string  `json:"expires_at,omitempty"`
}

// DebugAdjudication captures gate 3: the raw chat-model reply (before the
// INSUFFICIENT check) and whether it was judged sufficient to answer.
type DebugAdjudication struct {
	Enabled       bool   `json:"enabled"`
	ModelResponse string `json:"model_response"`
	Sufficient    bool   `json:"sufficient"`
}

// MemoryAnswer retrieves the memory facts a question is near and returns them
// for the caller to use:
//
//	gate 1 - vector similarity threshold
//	gate 2 - freshness (tier expiry + volatility max-age)
//
// It returns the fresh, similar facts (answer is their joined text) and leaves
// synthesis to the caller. When gate3_enabled, the answer is additionally
// upgraded to an LLM synthesis if the model judges the facts sufficient — that
// step never withholds the facts. ok is false whenever gates 1-2 fail or a
// re-embed migration is in flight. When debug is true, a *MemoryDebug explaining
// the outcome is also returned, and the query is a read-only probe: it records
// no hits, so it cannot promote a fact or slide its expiry.
func MemoryAnswer(ctx context.Context, store *Store, llm *LLMClient, cfg TierConfig, mem MemoryConfig, question string, debug bool) (answer string, used []MemoryFact, dbg *MemoryDebug, ok bool, err error) {
	if debug {
		dbg = &MemoryDebug{SimilarityThreshold: mem.SimilarityThreshold, TopK: mem.TopK}
	}

	table, ready, err := store.activeVectorTable(ctx)
	if err != nil || !ready || table == "" {
		if dbg != nil {
			dbg.StoppedAt = "table"
			dbg.Table = table
			dbg.Migrating = err == nil && !ready
		}
		return "", nil, dbg, false, err
	}
	if dbg != nil {
		dbg.Table = table
	}

	qvecs, err := llm.Embed(ctx, []string{question}, true, "memory query")
	if err != nil {
		if dbg != nil {
			dbg.StoppedAt = "embed"
		}
		return "", nil, dbg, false, err
	}
	if dbg != nil {
		dbg.QueryEmbedded = llm.EmbedQueryPrefix() + question
	}

	cands, err := store.RetrieveCandidates(ctx, table, qvecs[0], mem.TopK)
	if err != nil {
		if dbg != nil {
			dbg.StoppedAt = "gate1_similarity"
		}
		return "", nil, dbg, false, err
	}

	// gate 1: similarity threshold.
	var facts []MemoryFact
	for _, c := range cands {
		if c.Similarity >= mem.SimilarityThreshold {
			facts = append(facts, c)
		}
	}
	if dbg != nil {
		dbg.Candidates = debugCandidates(cands, mem.SimilarityThreshold)
	}
	if len(facts) == 0 {
		if dbg != nil {
			dbg.StoppedAt = "gate1_similarity"
		}
		return "", nil, dbg, false, nil
	}

	// gate 2: freshness.
	var fresh []MemoryFact
	for _, f := range facts {
		if !freshEnough(f.ExpiresAt, f.FetchedAt, volatilityMaxAge(f.Volatility)) {
			continue
		}
		fresh = append(fresh, f)
	}
	if len(fresh) == 0 {
		if dbg != nil {
			dbg.StoppedAt = "gate2_freshness"
		}
		return "", nil, dbg, false, nil
	}

	// Memory returns the fresh, similar facts and leaves synthesis to the caller.
	// Gate 3, when enabled, only upgrades the answer to a synthesis — it never
	// withholds the facts, and an adjudication error falls back to them.
	answer = joinFacts(fresh)
	if mem.Gate3Enabled {
		if syn, sufficient, modelOut, aerr := adjudicate(ctx, llm, question, fresh); aerr == nil {
			if sufficient {
				answer = syn
			}
			if dbg != nil {
				dbg.Adjudication = &DebugAdjudication{Enabled: true, ModelResponse: modelOut, Sufficient: sufficient}
			}
		} else if dbg != nil {
			dbg.Adjudication = &DebugAdjudication{Enabled: true, ModelResponse: "", Sufficient: false}
		}
	} else if dbg != nil {
		dbg.Adjudication = &DebugAdjudication{Enabled: false}
	}
	if !debug {
		recordMemoryHits(ctx, store, cfg, fresh)
	}
	if dbg != nil {
		dbg.StoppedAt = "ok"
	}
	return answer, fresh, dbg, true, nil
}

// debugCandidates annotates the nearest facts for the debug view: each one's
// similarity plus the two booleans (threshold, freshness) that decided its fate.
func debugCandidates(cands []MemoryFact, threshold float64) []DebugCandidate {
	out := make([]DebugCandidate, 0, len(cands))
	for _, c := range cands {
		out = append(out, DebugCandidate{
			Text:            c.Text,
			SourceURL:       c.SourceURL,
			Volatility:      c.Volatility,
			Similarity:      c.Similarity,
			PassesThreshold: c.Similarity >= threshold,
			Fresh:           freshEnough(c.ExpiresAt, c.FetchedAt, volatilityMaxAge(c.Volatility)),
			FetchedAt:       c.FetchedAt,
			ExpiresAt:       c.ExpiresAt,
		})
	}
	return out
}

const adjudicateSystemPrompt = `You answer a question strictly and only from the provided facts.
- If the facts contain enough to answer, respond with the answer and nothing else.
- If they do not, respond with exactly: INSUFFICIENT
Do not use outside knowledge. Do not guess.`

// adjudicate asks the chat model to answer strictly from the facts. It returns
// the answer (empty when insufficient) plus the model's raw reply, so the debug
// path can show exactly what the model said before the INSUFFICIENT check.
func adjudicate(ctx context.Context, llm *LLMClient, question string, facts []MemoryFact) (answer string, sufficient bool, raw string, err error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\n\nFacts:\n", question)
	for _, f := range facts {
		fmt.Fprintf(&b, "- %s\n", f.Text)
	}
	out, err := llm.Chat(ctx, adjudicateSystemPrompt, b.String())
	if err != nil {
		return "", false, "", err
	}
	out = strings.TrimSpace(out)
	if out == "" || strings.EqualFold(out, "INSUFFICIENT") || strings.HasPrefix(strings.ToUpper(out), "INSUFFICIENT") {
		return "", false, out, nil
	}
	return out, true, out, nil
}

func joinFacts(facts []MemoryFact) string {
	var b strings.Builder
	for i, f := range facts {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("- ")
		b.WriteString(f.Text)
	}
	return b.String()
}

func recordMemoryHits(ctx context.Context, store *Store, cfg TierConfig, facts []MemoryFact) {
	for i := range facts {
		_ = store.touchMemoryFact(ctx, cfg, &facts[i])
	}
}
