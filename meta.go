package main

import (
	"context"
	"database/sql"
	"fmt"
)

// system_meta keys used by the app.
const (
	metaEmbedModel = "embed_model"     // active embedding model id
	metaEmbedDim   = "embed_dim"       // active embedding dimension
	metaMigration  = "vector_migration" // "" or the id/state of an in-flight re-embed
)

// MetaGet returns the value for a system_meta key. found is false when the key
// is absent, which callers use to detect a first run.
func (s *Store) MetaGet(ctx context.Context, key string) (value string, found bool, err error) {
	err = s.db.QueryRowContext(ctx, `SELECT value FROM system_meta WHERE key = ?`, key).Scan(&value)
	switch {
	case err == sql.ErrNoRows:
		return "", false, nil
	case err != nil:
		return "", false, fmt.Errorf("reading meta %q: %w", key, err)
	}
	return value, true, nil
}

// MetaSet upserts a system_meta key.
func (s *Store) MetaSet(ctx context.Context, key, value string) error {
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO system_meta (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, nowRFC3339()); err != nil {
		return fmt.Errorf("writing meta %q: %w", key, err)
	}
	return nil
}
