package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const jobTypeDistill = "distill"

// distillMaxChars bounds how much page text is sent to the model, keeping the
// prompt within a sane context window.
const distillMaxChars = 8000

type distillPayload struct {
	ScrapeID string `json:"scrape_id"`
	Tier     string `json:"tier"`
}

type extractedFact struct {
	Text       string `json:"text"`
	Volatility string `json:"volatility"`
}

// enqueueDistill schedules the extraction of memory facts from a stored scrape.
// It is invoked when the caller asked to remember a scrape (default on).
func enqueueDistill(ctx context.Context, store *Store, scrapeID, tier string) error {
	buf, err := json.Marshal(distillPayload{ScrapeID: scrapeID, Tier: tier})
	if err != nil {
		return err
	}
	_, err = store.EnqueueJob(ctx, jobTypeDistill, string(buf))
	return err
}

// distillHandler loads a scrape's text, extracts multiple atomic facts with a
// volatility label each, and stores them via semantic upsert.
func distillHandler(store *Store, llm *LLMClient, cfg TierConfig, mem MemoryConfig) JobHandler {
	return func(ctx context.Context, payload string) error {
		var p distillPayload
		if err := json.Unmarshal([]byte(payload), &p); err != nil {
			return fmt.Errorf("distill payload: %w", err)
		}
		detail, err := store.GetScrape(ctx, p.ScrapeID, false)
		if err != nil {
			return err
		}
		if strings.TrimSpace(detail.Text) == "" {
			return nil // nothing to distill
		}
		facts, err := extractFacts(ctx, llm, detail.Text)
		if err != nil {
			return err
		}
		tier := normalizeTier(p.Tier, mem.RememberDefault)
		for _, f := range facts {
			if strings.TrimSpace(f.Text) == "" {
				continue
			}
			if _, err := store.StoreFact(ctx, cfg, mem, llm, f.Text, detail.URL, f.Volatility, tier); err != nil {
				return err
			}
		}
		return nil
	}
}

const distillSystemPrompt = `You extract atomic, self-contained facts from web page text.
Rules:
- Each fact must stand alone: no pronouns, no "the article", no references to other facts.
- Split compound statements into separate facts.
- Label each fact's volatility as "stable" (rarely changes) or "time-sensitive" (schedules, prices, standings, current office-holders, anything that can change soon).
- Ignore navigation, boilerplate and opinion.
Respond with ONLY a JSON array, no prose:
[{"text":"...","volatility":"stable|time-sensitive"}]`

func extractFacts(ctx context.Context, llm *LLMClient, text string) ([]extractedFact, error) {
	if len(text) > distillMaxChars {
		text = text[:distillMaxChars]
	}
	raw, err := llm.Chat(ctx, distillSystemPrompt, text)
	if err != nil {
		return nil, err
	}
	body := jsonArray(raw)
	if body == "" {
		return nil, nil
	}
	var facts []extractedFact
	if err := json.Unmarshal([]byte(body), &facts); err != nil {
		return nil, fmt.Errorf("distill: parsing model output: %w", err)
	}
	return facts, nil
}

// jsonArray extracts the outermost [...] span from a model reply, tolerating
// stray prose or code fences around it.
func jsonArray(s string) string {
	start := strings.IndexByte(s, '[')
	end := strings.LastIndexByte(s, ']')
	if start < 0 || end < 0 || end <= start {
		return ""
	}
	return s[start : end+1]
}
