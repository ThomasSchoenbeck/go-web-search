package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
	"time"
)

// CachedURL is one result URL stored in a search-cache entry.
type CachedURL struct {
	URL    string `json:"url"`
	Domain string `json:"domain,omitempty"`
	Rank   int    `json:"rank,omitempty"`
}

// SearchCacheEntry is a cached query result returned to the resolver.
type SearchCacheEntry struct {
	ID        string
	Query     string
	Results   []CachedURL
	Tier      string
	HitCount  int
	FetchedAt string
	ExpiresAt string
	Source    string
}

// normalizeQuery is the exact-match key: lowercase, whitespace-collapsed. It is
// intentionally conservative (no token reordering) so phrase intent survives;
// wording differences are caught by the semantic lookup instead.
func normalizeQuery(q string) string {
	return strings.Join(strings.Fields(strings.ToLower(q)), " ")
}

// StoreSearchCache upserts a query's results, resetting its tier window, and (on
// first insert) enqueues the deferred embedding of the query so a later
// semantic lookup can match it.
func (s *Store) StoreSearchCache(ctx context.Context, cfg TierConfig, query string, results []CachedURL, tier string) (string, error) {
	norm := normalizeQuery(query)
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	blob, err := json.Marshal(results)
	if err != nil {
		return "", err
	}
	newTier, exp, perm := nextExpiry(now, tier, 0, cfg)
	var expArg any
	if !perm {
		expArg = exp.Format(time.RFC3339Nano)
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE search_cache SET query = ?, results = ?, tier = ?, hit_count = 0,
		        expires_at = ?, fetched_at = ?, updated_at = ? WHERE query_norm = ?`,
		query, string(blob), newTier, expArg, nowStr, nowStr, norm)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		var id string
		if err := s.db.QueryRowContext(ctx, `SELECT id FROM search_cache WHERE query_norm = ?`, norm).Scan(&id); err != nil {
			return "", err
		}
		return id, nil
	}

	id := newID()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO search_cache (id, query_norm, query, results, tier, hit_count, expires_at, fetched_at, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, ?, ?, ?, ?)`,
		id, norm, query, string(blob), newTier, expArg, nowStr, nowStr, nowStr); err != nil {
		return "", err
	}
	if err := enqueueEmbed(ctx, s, ownerSearch, id, query); err != nil {
		return id, err
	}
	return id, nil
}

// LookupSearchCacheExact returns a cached entry whose normalized query matches
// exactly and is still fresh, recording the hit.
func (s *Store) LookupSearchCacheExact(ctx context.Context, cfg TierConfig, query string, maxAge time.Duration) (*SearchCacheEntry, bool, error) {
	e, ok, err := s.scanSearchCache(ctx, `WHERE query_norm = ?`, normalizeQuery(query))
	if err != nil || !ok {
		return nil, false, err
	}
	if !freshEnough(e.ExpiresAt, e.FetchedAt, maxAge) {
		return nil, false, nil
	}
	if err := s.touchSearchCache(ctx, cfg, e); err != nil {
		return nil, false, err
	}
	e.Source = "cache"
	return e, true, nil
}

// LookupSearchCacheSemantic returns the most similar cached query whose vector
// clears the similarity threshold and is still fresh. It is skipped while a
// re-embed migration is in flight (semantic degrades to exact-only).
func (s *Store) LookupSearchCacheSemantic(ctx context.Context, cfg TierConfig, mem MemoryConfig, queryVec []float32, maxAge time.Duration) (*SearchCacheEntry, bool, error) {
	table, ready, err := s.activeVectorTable(ctx)
	if err != nil || !ready || table == "" {
		return nil, false, err
	}
	hits, err := s.VectorSearch(ctx, table, ownerSearch, queryVec, mem.TopK)
	if err != nil {
		return nil, false, err
	}
	for _, h := range hits {
		if sim := 1 - h.Distance; sim < mem.SimilarityThreshold {
			break // hits are ordered nearest-first, so no later hit can pass
		}
		e, ok, err := s.scanSearchCache(ctx, `WHERE id = ?`, h.ID)
		if err != nil {
			return nil, false, err
		}
		if !ok || !freshEnough(e.ExpiresAt, e.FetchedAt, maxAge) {
			continue
		}
		if err := s.touchSearchCache(ctx, cfg, e); err != nil {
			return nil, false, err
		}
		e.Source = "cache"
		return e, true, nil
	}
	return nil, false, nil
}

func (s *Store) scanSearchCache(ctx context.Context, where string, arg any) (*SearchCacheEntry, bool, error) {
	var e SearchCacheEntry
	var results string
	var exp sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, query, results, tier, hit_count, expires_at, fetched_at FROM search_cache `+where,
		arg).Scan(&e.ID, &e.Query, &results, &e.Tier, &e.HitCount, &exp, &e.FetchedAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	e.ExpiresAt = exp.String
	if err := json.Unmarshal([]byte(results), &e.Results); err != nil {
		return nil, false, err
	}
	return &e, true, nil
}

// touchSearchCache records a hit: it bumps hit_count and slides the expiry
// window forward, promoting the tier if the hit count crossed the threshold.
func (s *Store) touchSearchCache(ctx context.Context, cfg TierConfig, e *SearchCacheEntry) error {
	hits := e.HitCount + 1
	now := time.Now().UTC()
	newTier, exp, perm := nextExpiry(now, e.Tier, hits, cfg)
	var expArg any
	if !perm {
		expArg = exp.Format(time.RFC3339Nano)
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE search_cache SET hit_count = ?, tier = ?, expires_at = ?, updated_at = ? WHERE id = ?`,
		hits, newTier, expArg, now.Format(time.RFC3339Nano), e.ID)
	e.HitCount = hits
	e.Tier = newTier
	return err
}

// freshEnough reports whether a row has not expired and, when maxAge > 0, was
// fetched within maxAge. It is shared by the cache lookups.
func freshEnough(expiresAt, fetchedAt string, maxAge time.Duration) bool {
	now := time.Now().UTC()
	if expiresAt != "" {
		if exp, err := time.Parse(time.RFC3339Nano, expiresAt); err == nil && now.After(exp) {
			return false
		}
	}
	if maxAge > 0 {
		if f, err := time.Parse(time.RFC3339Nano, fetchedAt); err == nil && now.Sub(f) > maxAge {
			return false
		}
	}
	return true
}
