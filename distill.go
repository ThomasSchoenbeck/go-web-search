package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
)

const jobTypeDistill = "distill"

// distillMaxChars bounds how much page text is sent to the model, keeping the
// prompt within a sane context window.
const distillMaxChars = 8000

type distillPayload struct {
	ScrapeID string `json:"scrape_id"`
	Tier     string `json:"tier"`
	Detail   string `json:"detail,omitempty"`
	Focus    string `json:"focus,omitempty"`
}

type extractedFact struct {
	Text       string `json:"text"`
	Volatility string `json:"volatility"`
}

// enqueueDistill schedules the extraction of memory facts from a stored scrape.
// It is invoked when the caller asked to remember a scrape (default on). detail
// ("brief"/"normal"/"thorough") and focus (free-text guidance) shape the distil
// prompt; empty values fall back to the balanced default.
func enqueueDistill(ctx context.Context, store *Store, scrapeID, tier, detail, focus string) error {
	buf, err := json.Marshal(distillPayload{ScrapeID: scrapeID, Tier: tier, Detail: detail, Focus: focus})
	if err != nil {
		return err
	}
	_, err = store.EnqueueJob(ctx, jobTypeDistill, string(buf))
	return err
}

// distillHandler loads a scrape's text, extracts multiple atomic facts with a
// volatility label each, and stores them via semantic upsert.
func distillHandler(store *Store, llm *LLMClient, cfg TierConfig, mem MemoryConfig, logger *log.Logger) JobHandler {
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
		text := detail.Text
		if len(text) > distillMaxChars {
			text = text[:distillMaxChars]
		}
		logger.Printf("     distill: extracting knowledge from scrape %s (%d chars, detail=%s) %s", p.ScrapeID, len(text), distillDetail(p.Detail), detail.URL)
		facts, _, err := extractFacts(ctx, llm, distillSystemPrompt(p.Detail, p.Focus), text, chatParams{})
		if err != nil {
			return err
		}
		logger.Printf("     distill: %d facts extracted from %s", len(facts), detail.URL)
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

// distillDetail normalises the detail knob to one of the three known levels,
// defaulting anything unrecognised (including "") to "normal".
func distillDetail(detail string) string {
	switch strings.ToLower(strings.TrimSpace(detail)) {
	case "brief":
		return "brief"
	case "thorough":
		return "thorough"
	default:
		return "normal"
	}
}

// distillSystemPrompt builds the extraction prompt from the detail level and an
// optional free-text focus. "normal" and "brief" tell the model to consolidate
// rather than enumerate, which is what keeps a schedule page from exploding into
// hundreds of near-identical facts; "thorough" restores exhaustive splitting.
func distillSystemPrompt(detail, focus string) string {
	b := strings.Builder{}
	b.WriteString(`You extract atomic, self-contained facts from web page text.
Rules:
- Each fact must stand alone: no pronouns, no "the article", no references to other facts.
- Label each fact's volatility as "stable" (rarely changes) or "time-sensitive" (schedules, prices, standings, current office-holders, anything that can change soon).
- Ignore navigation, boilerplate and opinion.`)
	switch distillDetail(detail) {
	case "brief":
		b.WriteString("\n- Be highly selective: return only the few most important, high-value facts (roughly 5-15). Consolidate lists and tables into a handful of summary facts; never emit one fact per row or cell.")
	case "thorough":
		b.WriteString("\n- Be exhaustive: split compound statements and extract every distinct fact.")
	default: // normal
		b.WriteString("\n- Extract the key facts and skip trivia. Consolidate repetitive list/table rows into concise summary facts rather than one fact per row; avoid near-duplicate facts.")
	}
	if f := strings.TrimSpace(focus); f != "" {
		b.WriteString("\n- Focus on: " + f + ". Prioritise facts relevant to this and skip unrelated content.")
	}
	b.WriteString("\nRespond with ONLY a JSON array, no prose:\n[{\"text\":\"...\",\"volatility\":\"stable|time-sensitive\"}]")
	return b.String()
}

// extractFacts runs one distill call with the given system prompt and (already
// truncated) text, returning the parsed facts and the call's token usage. p
// carries any per-call model overrides (used by the preview).
func extractFacts(ctx context.Context, llm *LLMClient, systemPrompt, text string, p chatParams) ([]extractedFact, ChatUsage, error) {
	raw, usage, err := llm.chat(ctx, systemPrompt, text, p)
	if err != nil {
		return nil, usage, err
	}
	body := jsonArray(raw)
	if body == "" {
		return nil, usage, nil
	}
	var facts []extractedFact
	if err := json.Unmarshal([]byte(body), &facts); err != nil {
		return nil, usage, fmt.Errorf("distill: parsing model output: %w", err)
	}
	return facts, usage, nil
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
