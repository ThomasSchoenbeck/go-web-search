package main

import (
	"context"
	"fmt"
)

// The projection dump hands the browser raw embeddings so it can lay them out in
// 2-D itself (T022 computes a PCA client-side). Nothing is projected here: that
// would mean a linear-algebra dependency in Go for a picture only the browser
// ever draws.
//
// This is the heaviest read in the API — whole embeddings, not distances — so it
// is capped by observability.projection_sample_cap and paginated. A dim-4096
// model at the default cap is already tens of megabytes of JSON; the cap is a
// real limit, not a formality.

// ProjectionPoint is one embedding, labelled enough for a scatter to annotate it
// without a second round trip.
type ProjectionPoint struct {
	ID        string    `json:"id"`
	OwnerKind string    `json:"owner_kind"` // memory | search
	Label     string    `json:"label"`
	SourceURL string    `json:"source_url,omitempty"`
	Vector    []float32 `json:"vector"`
}

// ProjectionDump is the endpoint payload. Available is false when the vector
// store cannot answer, in which case Points is empty and Note says why — a
// normal state, not an error, exactly as the explorer reports it.
type ProjectionDump struct {
	Available bool   `json:"available"`
	Note      string `json:"note,omitempty"`
	Model     string `json:"model,omitempty"`
	Dim       int    `json:"dim,omitempty"`
	// Limit is the cap actually applied after clamping; Offset is the page start.
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
	// Total counts every vector in the store, per owner kind, so the view can say
	// how much of the space it is showing.
	Total     map[string]int    `json:"total"`
	Truncated bool              `json:"truncated"`
	Points    []ProjectionPoint `json:"points"`
}

// VectorProjection returns a bounded page of raw embeddings with their labels.
// sampleCap comes from config; limit is clamped to it, and 0 means "the cap".
func (s *Store) VectorProjection(ctx context.Context, sampleCap, limit, offset int) (*ProjectionDump, error) {
	if sampleCap <= 0 {
		sampleCap = 1
	}
	if limit <= 0 || limit > sampleCap {
		limit = sampleCap
	}
	if offset < 0 {
		offset = 0
	}
	dump := &ProjectionDump{Limit: limit, Offset: offset, Total: map[string]int{}, Points: []ProjectionPoint{}}

	table, ready, err := s.activeVectorTable(ctx)
	if err != nil {
		return nil, err
	}
	if table == "" {
		dump.Note = noVectorsNote
		return dump, nil
	}
	if !ready {
		dump.Note = migratingNote
		return dump, nil
	}
	dump.Available = true

	total, err := s.vectorTotals(ctx, table)
	if err != nil {
		if isNoSuchTable(err) {
			// The meta key names a table that is gone: the same "nothing to show"
			// the explorer reports rather than a 500.
			dump.Available = false
			dump.Note = noVectorsNote
			return dump, nil
		}
		return nil, err
	}
	dump.Total = total

	// Ordered by (owner_kind, id) so paging is stable — ids are UUIDv7, so this
	// is also insertion order within a kind.
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, owner_kind, vector_extract(embedding), model, dim
		               FROM %s ORDER BY owner_kind, id LIMIT ? OFFSET ?`, table),
		limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byKind := map[string][]string{}
	var points []ProjectionPoint
	for rows.Next() {
		var p ProjectionPoint
		var literal, model string
		var dim int
		if err := rows.Scan(&p.ID, &p.OwnerKind, &literal, &model, &dim); err != nil {
			return nil, err
		}
		vec, err := parseVectorLiteral(literal)
		if err != nil {
			return nil, err
		}
		p.Vector = vec
		dump.Model, dump.Dim = model, dim
		byKind[p.OwnerKind] = append(byKind[p.OwnerKind], p.ID)
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	labels, err := s.projectionLabels(ctx, byKind)
	if err != nil {
		return nil, err
	}
	for _, p := range points {
		label, ok := labels[p.OwnerKind+"/"+p.ID]
		if !ok {
			// A vector whose owning row was deleted: skipped rather than plotted
			// as an anonymous dot, matching the explorer.
			continue
		}
		p.Label, p.SourceURL = label.text, label.sourceURL
		dump.Points = append(dump.Points, p)
	}

	seen := offset + len(points)
	sum := 0
	for _, n := range total {
		sum += n
	}
	dump.Truncated = seen < sum
	return dump, nil
}

// vectorTotals counts the whole store per owner kind, which is what makes
// "showing 200 of 5000" possible.
func (s *Store) vectorTotals(ctx context.Context, table string) (map[string]int, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT owner_kind, COUNT(*) FROM %s GROUP BY owner_kind`, table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	total := map[string]int{}
	for rows.Next() {
		var kind string
		var n int
		if err := rows.Scan(&kind, &n); err != nil {
			return nil, err
		}
		total[kind] = n
	}
	return total, rows.Err()
}

type projectionLabel struct{ text, sourceURL string }

// projectionLabels resolves owner ids to something readable, one query per kind
// rather than one per point.
func (s *Store) projectionLabels(ctx context.Context, byKind map[string][]string) (map[string]projectionLabel, error) {
	labels := map[string]projectionLabel{}
	for kind, ids := range byKind {
		if len(ids) == 0 {
			continue
		}
		var query string
		switch kind {
		case ownerMemory:
			query = `SELECT id, text, COALESCE(source_url, '') FROM memory_facts WHERE id IN (`
		case ownerSearch:
			query = `SELECT id, query, '' FROM search_cache WHERE id IN (`
		default:
			continue // an owner kind nothing here knows how to label
		}
		args := make([]any, len(ids))
		for i, id := range ids {
			args[i] = id
		}
		rows, err := s.db.QueryContext(ctx, query+placeholders(len(ids))+`)`, args...)
		if err != nil {
			return nil, err
		}
		for rows.Next() {
			var id string
			var l projectionLabel
			if err := rows.Scan(&id, &l.text, &l.sourceURL); err != nil {
				rows.Close()
				return nil, err
			}
			labels[kind+"/"+id] = l
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return labels, nil
}
