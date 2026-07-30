package main

import (
	"context"
	"database/sql"
	"time"
)

// MemoryFact is one atomic, self-contained fact distilled from a scraped page.
type MemoryFact struct {
	ID         string
	Text       string
	SourceURL  string
	Volatility string
	Tier       string
	HitCount   int
	FetchedAt  string
	ExpiresAt  string
	Similarity float64
}

// StoreFact embeds a fact and stores it with semantic upsert: if a near-
// identical fact already exists (cosine similarity above the upsert threshold),
// the existing fact is refreshed in place instead of inserting a duplicate. This
// also collapses cross-lingual duplicates. Embedding is synchronous here because
// distillation already runs on a background worker.
func (s *Store) StoreFact(ctx context.Context, cfg TierConfig, mem MemoryConfig, llm *LLMClient, text, sourceURL, volatility, tier string) (string, error) {
	table, ready, err := s.activeVectorTable(ctx)
	if err != nil {
		return "", err
	}
	vecs, err := llm.Embed(ctx, []string{text}, false)
	if err != nil {
		return "", err
	}
	vec := vecs[0]
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)

	// Semantic upsert: refresh the nearest existing fact when it is close enough.
	if ready && table != "" {
		if hits, err := s.VectorSearch(ctx, table, ownerMemory, vec, 1); err == nil && len(hits) > 0 {
			if sim := 1 - hits[0].Distance; sim >= mem.UpsertThreshold {
				id := hits[0].ID
				existing, ok, _ := s.scanFact(ctx, id)
				keepTier := tier
				keepHits := 0
				if ok {
					keepTier, keepHits = existing.Tier, existing.HitCount
				}
				newTier, exp, perm := nextExpiry(now, keepTier, keepHits, cfg)
				var expArg any
				if !perm {
					expArg = exp.Format(time.RFC3339Nano)
				}
				if _, err := s.db.ExecContext(ctx,
					`UPDATE memory_facts SET text = ?, source_url = ?, volatility = ?, tier = ?,
					        expires_at = ?, fetched_at = ?, updated_at = ? WHERE id = ?`,
					text, sourceURL, volatility, newTier, expArg, nowStr, nowStr, id); err != nil {
					return "", err
				}
				if err := s.UpsertVector(ctx, table, ownerMemory, id, vec, llm.EmbedModelName(), llm.EmbedDim()); err != nil {
					return "", err
				}
				return id, nil
			}
		}
	}

	id := newID()
	newTier, exp, perm := nextExpiry(now, tier, 0, cfg)
	var expArg any
	if !perm {
		expArg = exp.Format(time.RFC3339Nano)
	}
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO memory_facts (id, text, source_url, volatility, tier, hit_count, expires_at, fetched_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
		id, text, sourceURL, volatility, newTier, expArg, nowStr, nowStr, nowStr); err != nil {
		return "", err
	}
	if table != "" {
		if err := s.UpsertVector(ctx, table, ownerMemory, id, vec, llm.EmbedModelName(), llm.EmbedDim()); err != nil {
			return id, err
		}
	}
	return id, nil
}

// RetrieveFacts returns the facts most similar to the query vector that clear
// the similarity threshold, nearest first.
func (s *Store) RetrieveFacts(ctx context.Context, mem MemoryConfig, table string, queryVec []float32) ([]MemoryFact, error) {
	hits, err := s.VectorSearch(ctx, table, ownerMemory, queryVec, mem.TopK)
	if err != nil {
		return nil, err
	}
	var out []MemoryFact
	for _, h := range hits {
		sim := 1 - h.Distance
		if sim < mem.SimilarityThreshold {
			break // nearest-first, so nothing later can pass
		}
		f, ok, err := s.scanFact(ctx, h.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		f.Similarity = sim
		out = append(out, *f)
	}
	return out, nil
}

func (s *Store) scanFact(ctx context.Context, id string) (*MemoryFact, bool, error) {
	var f MemoryFact
	var src, vol, exp sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, text, source_url, volatility, tier, hit_count, expires_at, fetched_at
		   FROM memory_facts WHERE id = ?`, id).
		Scan(&f.ID, &f.Text, &src, &vol, &f.Tier, &f.HitCount, &exp, &f.FetchedAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	f.SourceURL = src.String
	f.Volatility = vol.String
	f.ExpiresAt = exp.String
	return &f, true, nil
}

// touchMemoryFact records a hit: bump hit_count and slide the expiry window.
func (s *Store) touchMemoryFact(ctx context.Context, cfg TierConfig, f *MemoryFact) error {
	hits := f.HitCount + 1
	now := time.Now().UTC()
	newTier, exp, perm := nextExpiry(now, f.Tier, hits, cfg)
	var expArg any
	if !perm {
		expArg = exp.Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE memory_facts SET hit_count = ?, tier = ?, expires_at = ?, updated_at = ? WHERE id = ?`,
		hits, newTier, expArg, now.Format(time.RFC3339Nano), f.ID)
	f.HitCount = hits
	f.Tier = newTier
	return err
}
