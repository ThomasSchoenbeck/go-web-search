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

// MemoryAnswer runs the confidence gates and, when they pass, returns an answer
// synthesized from memory so the caller can skip a web search:
//
//	gate 1 - vector similarity threshold (applied in RetrieveFacts)
//	gate 2 - freshness (tier expiry + volatility max-age)
//	gate 3 - LLM adjudication: does the retrieved knowledge actually answer it?
//
// ok is false (and the caller should hit the web) whenever any gate fails or a
// re-embed migration is in flight.
func MemoryAnswer(ctx context.Context, store *Store, llm *LLMClient, cfg TierConfig, mem MemoryConfig, question string) (answer string, used []MemoryFact, ok bool, err error) {
	table, ready, err := store.activeVectorTable(ctx)
	if err != nil || !ready || table == "" {
		return "", nil, false, err
	}
	qvecs, err := llm.Embed(ctx, []string{question}, true)
	if err != nil {
		return "", nil, false, err
	}
	facts, err := store.RetrieveFacts(ctx, mem, table, qvecs[0])
	if err != nil {
		return "", nil, false, err
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
		return "", nil, false, nil
	}

	// gate 3: adjudication + synthesis. Disabled means we trust gates 1-2 and
	// hand back the facts verbatim.
	if !mem.Gate3Enabled {
		recordMemoryHits(ctx, store, cfg, fresh)
		return joinFacts(fresh), fresh, true, nil
	}
	ans, sufficient, err := adjudicate(ctx, llm, question, fresh)
	if err != nil {
		return "", nil, false, err
	}
	if !sufficient {
		return "", nil, false, nil
	}
	recordMemoryHits(ctx, store, cfg, fresh)
	return ans, fresh, true, nil
}

const adjudicateSystemPrompt = `You answer a question strictly and only from the provided facts.
- If the facts contain enough to answer, respond with the answer and nothing else.
- If they do not, respond with exactly: INSUFFICIENT
Do not use outside knowledge. Do not guess.`

func adjudicate(ctx context.Context, llm *LLMClient, question string, facts []MemoryFact) (answer string, sufficient bool, err error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Question: %s\n\nFacts:\n", question)
	for _, f := range facts {
		fmt.Fprintf(&b, "- %s\n", f.Text)
	}
	out, err := llm.Chat(ctx, adjudicateSystemPrompt, b.String())
	if err != nil {
		return "", false, err
	}
	out = strings.TrimSpace(out)
	if out == "" || strings.EqualFold(out, "INSUFFICIENT") || strings.HasPrefix(strings.ToUpper(out), "INSUFFICIENT") {
		return "", false, nil
	}
	return out, true, nil
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
