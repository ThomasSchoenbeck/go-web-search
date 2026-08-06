package main

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
)

// Vector storage lives in its own table, keyed by the owner row's id plus an
// owner_kind discriminator, so a fact or a search-cache row references exactly
// one vector without carrying the embedding column itself. The table is created
// at runtime (not in schema.sql) because its dimension is config-driven and a
// model/dimension change spins up a new generation table (see reembed.go).
//
// tursogo v0.7.1 is the Rust Turso engine, not libSQL: it implements F32_BLOB
// columns and the vector32()/vector_distance_cos() functions, but NOT libSQL's
// libsql_vector_idx ANN index or vector_top_k(). There is therefore no
// approximate index here; VectorSearch does an exact linear scan with
// vector_distance_cos, which is fine at the scale of a local research tool. If
// this ever moves to a libSQL backend, add the ANN index in ensureVectorTable
// and switch VectorSearch back to vector_top_k.

const (
	metaVectorTable = "vector_table" // active vectors table name
	metaVectorGen   = "vector_gen"   // monotonic generation counter for table names

	ownerMemory = "memory"
	ownerSearch = "search"
)

// VectorHit is one nearest-neighbour result. Distance is cosine distance in
// [0,2]; cosine similarity is 1 - Distance.
type VectorHit struct {
	ID       string
	Distance float64
}

// ensureVectorTable creates a generation table and its indexes if absent. name
// is internally generated and dim is an int, so the fmt-built DDL is safe.
func (s *Store) ensureVectorTable(ctx context.Context, name string, dim int) error {
	// No ANN index: the Rust Turso engine has no libsql_vector_idx, so search is
	// an exact linear scan (see VectorSearch). Each statement runs on its own so
	// a driver rejection names exactly which one failed rather than a single
	// opaque error.
	steps := []struct {
		what string
		ddl  string
	}{
		{
			"create table (F32_BLOB column)",
			fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %[1]s (
    id         TEXT NOT NULL,
    owner_kind TEXT NOT NULL,
    embedding  F32_BLOB(%[2]d) NOT NULL,
    model      TEXT NOT NULL,
    dim        INTEGER NOT NULL,
    created_at TEXT NOT NULL
)`, name, dim),
		},
		{
			"unique owner index",
			fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS %[1]s_owner_idx ON %[1]s (owner_kind, id)`, name),
		},
	}
	for _, step := range steps {
		if _, err := s.db.ExecContext(ctx, step.ddl); err != nil {
			return fmt.Errorf("vector table %s: %s failed: %w", name, step.what, err)
		}
	}
	return nil
}

// UpsertVector replaces any existing vector for (ownerKind, id) with a fresh one.
func (s *Store) UpsertVector(ctx context.Context, table, ownerKind, id string, embedding []float32, model string, dim int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE owner_kind = ? AND id = ?`, table), ownerKind, id); err != nil {
		return fmt.Errorf("clearing old vector: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		fmt.Sprintf(`INSERT INTO %s (id, owner_kind, embedding, model, dim, created_at)
		             VALUES (?, ?, vector32(?), ?, ?, ?)`, table),
		id, ownerKind, vectorLiteral(embedding), model, dim, nowRFC3339()); err != nil {
		return fmt.Errorf("inserting vector: %w", err)
	}
	return tx.Commit()
}

// DeleteVector removes the vector for an owner row (used on cascade delete).
func (s *Store) DeleteVector(ctx context.Context, table, ownerKind, id string) error {
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`DELETE FROM %s WHERE owner_kind = ? AND id = ?`, table), ownerKind, id)
	return err
}

// VectorSearch returns the k nearest owner ids of a kind to the query vector.
// The Rust Turso engine has no ANN index, so this is an exact linear scan:
// vector_distance_cos against every row of the kind, ordered by distance. At the
// scale of a local research tool that is cheap; if the table ever grows large,
// this is the place to reintroduce an approximate index.
func (s *Store) VectorSearch(ctx context.Context, table, ownerKind string, query []float32, k int) ([]VectorHit, error) {
	if k <= 0 {
		k = 8
	}
	lit := vectorLiteral(query)
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT v.id, vector_distance_cos(v.embedding, vector32(?)) AS dist
		               FROM %s v
		              WHERE v.owner_kind = ?
		              ORDER BY dist
		              LIMIT ?`, table),
		lit, ownerKind, k)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	var out []VectorHit
	for rows.Next() {
		var h VectorHit
		if err := rows.Scan(&h.ID, &h.Distance); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// activeVectorTable reports the table semantic reads should use and whether the
// vector store is ready (no re-embed migration in flight). While a migration
// runs, ready is false and callers degrade: the search cache falls back to exact
// match and memory misses to the web.
func (s *Store) activeVectorTable(ctx context.Context) (table string, ready bool, err error) {
	table, ok, err := s.MetaGet(ctx, metaVectorTable)
	if err != nil {
		return "", false, err
	}
	if !ok || table == "" {
		return "", false, nil
	}
	migrating, _, err := s.MetaGet(ctx, metaMigration)
	if err != nil {
		return "", false, err
	}
	return table, migrating == "", nil
}

// vectorLiteral formats a float slice as the '[..]' text vector32() parses.
func vectorLiteral(v []float32) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

// parseVectorLiteral reads the '[a,b,c]' text back into floats — the inverse of
// vectorLiteral, and the only way out of an F32_BLOB column: the engine's
// vector_extract() returns this form, and decoding the blob bytes instead would
// mean hard-coding a storage layout the engine never promised.
func parseVectorLiteral(s string) ([]float32, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]float32, len(parts))
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 32)
		if err != nil {
			return nil, fmt.Errorf("parsing vector component %d: %w", i, err)
		}
		out[i] = float32(f)
	}
	return out, nil
}

// isNoSuchTable reports whether an error is a missing-table error, which the
// re-embed pass treats as "no rows of this owner kind yet".
func isNoSuchTable(err error) bool {
	return err != nil && err != sql.ErrNoRows && strings.Contains(strings.ToLower(err.Error()), "no such table")
}
