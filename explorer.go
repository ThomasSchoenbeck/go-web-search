package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
)

// The semantic explorer is a raw nearest-neighbour probe over the vector store,
// deliberately unlike /api/memory/query: no confidence gating, no synthesis, no
// answer. It embeds arbitrary text once and reports what sits closest to it in
// embedding space, across BOTH owner kinds — distilled facts and cached search
// queries — with the cosine distance shown rather than hidden behind a verdict.
//
// VectorSearch is an exact linear scan (the Rust Turso engine has no ANN index),
// so k stays bounded.

const (
	exploreDefaultK = 10
	exploreMaxK     = 100
)

// queryEmbedder is the slice of LLMClient the explorer needs. Declared here, at
// the point of use, so tests can supply a deterministic stub instead of a live
// model endpoint.
type queryEmbedder interface {
	Embed(ctx context.Context, texts []string, asQuery bool, purpose string) ([][]float32, error)
}

// Neighbor is one vector-space hit, resolved to something readable.
type Neighbor struct {
	OwnerKind string  `json:"owner_kind"` // memory | search
	ID        string  `json:"id"`
	Distance  float64 `json:"distance"`
	// Similarity is 1 − distance, carried so the UI does not have to know the
	// metric's polarity.
	Similarity float64 `json:"similarity"`
	Text       string  `json:"text"`
	SourceURL  string  `json:"source_url,omitempty"`
	Tier       string  `json:"tier,omitempty"`
	// ResultCount is set for cached-search neighbours: how many URLs it holds.
	ResultCount int `json:"result_count,omitempty"`
}

// ExploreResult is the endpoint payload. Available is false when the vector
// store cannot answer, in which case Neighbors is empty and Note explains why —
// this is a normal, expected state, not an error.
type ExploreResult struct {
	Query     string     `json:"query"`
	K         int        `json:"k"`
	Available bool       `json:"available"`
	Note      string     `json:"note,omitempty"`
	Neighbors []Neighbor `json:"neighbors"`
	// Counts of what was found per owner kind, before the global top-k cut.
	MemoryHits int `json:"memory_hits"`
	SearchHits int `json:"search_hits"`
}

const (
	noVectorsNote   = "no vectors have been stored yet, so there is nothing to search"
	migratingNote   = "a re-embed migration is in progress; semantic search is unavailable until it finishes"
	exploreEmptyMsg = "no neighbours found"
)

// Explore embeds the query once and returns the globally nearest neighbours
// across both owner kinds, nearest first.
func (s *Store) Explore(ctx context.Context, emb queryEmbedder, query string, k int) (*ExploreResult, error) {
	if k <= 0 {
		k = exploreDefaultK
	}
	if k > exploreMaxK {
		k = exploreMaxK
	}
	result := &ExploreResult{Query: query, K: k, Neighbors: []Neighbor{}}

	// Check availability before embedding: when the store cannot answer there is
	// no point paying for an embedding call.
	table, ready, err := s.activeVectorTable(ctx)
	if err != nil {
		return nil, err
	}
	if table == "" {
		result.Note = noVectorsNote
		return result, nil
	}
	if !ready {
		result.Note = migratingNote
		return result, nil
	}
	result.Available = true

	vectors, err := emb.Embed(ctx, []string{query}, true, "semantic explorer")
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 || len(vectors[0]) == 0 {
		return nil, errors.New("embedding the query returned no vector")
	}
	vec := vectors[0]

	// k from each kind, then a global cut, so the result really is the k nearest
	// overall rather than k/2 of each.
	memHits, err := s.VectorSearch(ctx, table, ownerMemory, vec, k)
	if err != nil {
		if isNoSuchTable(err) {
			result.Available = false
			result.Note = noVectorsNote
			return result, nil
		}
		return nil, err
	}
	searchHits, err := s.VectorSearch(ctx, table, ownerSearch, vec, k)
	if err != nil {
		return nil, err
	}
	result.MemoryHits = len(memHits)
	result.SearchHits = len(searchHits)

	neighbors, err := s.resolveMemoryHits(ctx, memHits)
	if err != nil {
		return nil, err
	}
	cached, err := s.resolveSearchHits(ctx, searchHits)
	if err != nil {
		return nil, err
	}
	neighbors = append(neighbors, cached...)

	sort.SliceStable(neighbors, func(i, j int) bool { return neighbors[i].Distance < neighbors[j].Distance })
	if len(neighbors) > k {
		neighbors = neighbors[:k]
	}
	result.Neighbors = neighbors
	return result, nil
}

// resolveMemoryHits turns memory vector ids into fact text. A vector whose fact
// has since been deleted is skipped rather than rendered as a blank row.
func (s *Store) resolveMemoryHits(ctx context.Context, hits []VectorHit) ([]Neighbor, error) {
	if len(hits) == 0 {
		return nil, nil
	}
	ids := make([]any, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, text, COALESCE(source_url, ''), tier
		   FROM memory_facts WHERE id IN (`+placeholders(len(ids))+`)`, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type factRow struct{ text, sourceURL, tier string }
	byID := map[string]factRow{}
	for rows.Next() {
		var id string
		var f factRow
		if err := rows.Scan(&id, &f.text, &f.sourceURL, &f.tier); err != nil {
			return nil, err
		}
		byID[id] = f
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Neighbor, 0, len(hits))
	for _, h := range hits {
		f, ok := byID[h.ID]
		if !ok {
			continue
		}
		out = append(out, Neighbor{
			OwnerKind:  ownerMemory,
			ID:         h.ID,
			Distance:   h.Distance,
			Similarity: 1 - h.Distance,
			Text:       f.text,
			SourceURL:  f.sourceURL,
			Tier:       f.tier,
		})
	}
	return out, nil
}

// resolveSearchHits turns search vector ids into their cached query and size.
func (s *Store) resolveSearchHits(ctx context.Context, hits []VectorHit) ([]Neighbor, error) {
	if len(hits) == 0 {
		return nil, nil
	}
	ids := make([]any, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, query, tier, results FROM search_cache WHERE id IN (`+placeholders(len(ids))+`)`, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type cacheRow struct {
		query, tier, results string
	}
	byID := map[string]cacheRow{}
	for rows.Next() {
		var id string
		var c cacheRow
		if err := rows.Scan(&id, &c.query, &c.tier, &c.results); err != nil {
			return nil, err
		}
		byID[id] = c
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]Neighbor, 0, len(hits))
	for _, h := range hits {
		c, ok := byID[h.ID]
		if !ok {
			continue
		}
		out = append(out, Neighbor{
			OwnerKind:   ownerSearch,
			ID:          h.ID,
			Distance:    h.Distance,
			Similarity:  1 - h.Distance,
			Text:        c.query,
			Tier:        c.tier,
			ResultCount: countCachedURLs(c.results),
		})
	}
	return out, nil
}

// countCachedURLs counts entries in a stored results JSON array without
// unmarshalling into the full CachedURL shape.
func countCachedURLs(raw string) int {
	if raw == "" {
		return 0
	}
	var urls []json.RawMessage
	if err := json.Unmarshal([]byte(raw), &urls); err != nil {
		return 0
	}
	return len(urls)
}
