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
// VERIFY (Turso, beta): the vector column type F32_BLOB(dim), the ANN index via
// libsql_vector_idx, and the vector32()/vector_top_k()/vector_distance_cos()
// functions are the documented Turso/libSQL forms. Confirm they are implemented
// in the pinned tursogo v0.7.1 before relying on this; the function names are
// centralised here so a rename is a one-file change.

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
	ddl := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %[1]s (
    id         TEXT NOT NULL,
    owner_kind TEXT NOT NULL,
    embedding  F32_BLOB(%[2]d) NOT NULL,
    model      TEXT NOT NULL,
    dim        INTEGER NOT NULL,
    created_at TEXT NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS %[1]s_owner_idx ON %[1]s (owner_kind, id);
CREATE INDEX IF NOT EXISTS %[1]s_ann ON %[1]s (libsql_vector_idx(embedding, 'metric=cosine'));`,
		name, dim)
	if _, err := s.db.ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("creating vector table %s: %w", name, err)
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
// It over-fetches from the ANN index and then filters by owner_kind, because the
// index is shared across kinds and vector_top_k ranks globally.
func (s *Store) VectorSearch(ctx context.Context, table, ownerKind string, query []float32, k int) ([]VectorHit, error) {
	if k <= 0 {
		k = 8
	}
	overfetch := k * 4
	if overfetch < 20 {
		overfetch = 20
	}
	lit := vectorLiteral(query)
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT v.id, vector_distance_cos(v.embedding, vector32(?)) AS dist
		               FROM vector_top_k('%[1]s_ann', vector32(?), ?) AS t
		               JOIN %[1]s v ON v.rowid = t.id
		              WHERE v.owner_kind = ?
		              ORDER BY dist
		              LIMIT ?`, table),
		lit, lit, overfetch, ownerKind, k)
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

// isNoSuchTable reports whether an error is a missing-table error, which the
// re-embed pass treats as "no rows of this owner kind yet".
func isNoSuchTable(err error) bool {
	return err != nil && err != sql.ErrNoRows && strings.Contains(strings.ToLower(err.Error()), "no such table")
}
