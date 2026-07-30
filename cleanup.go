package main

import (
	"context"
	"fmt"
)

const jobTypeCleanup = "cleanup"

// cleanupHandler deletes expired rows from every store. It runs on the recurring
// schedule registered in registerJobs.
func cleanupHandler(store *Store) JobHandler {
	return func(ctx context.Context, _ string) error {
		return store.CleanupExpired(ctx)
	}
}

// CleanupExpired removes rows whose sliding expiry has passed from the three
// stores, cascading vector deletion for the vectored ones. Permanent rows have a
// NULL expires_at and are never touched.
func (s *Store) CleanupExpired(ctx context.Context) error {
	now := nowRFC3339()
	table, _, _ := s.activeVectorTable(ctx)

	searchIDs, err := s.deleteExpired(ctx, "search_cache", now)
	if err != nil {
		return err
	}
	memIDs, err := s.deleteExpired(ctx, "memory_facts", now)
	if err != nil {
		return err
	}
	if _, err := s.deleteExpired(ctx, "scrape_cache", now); err != nil {
		return err
	}

	if table != "" {
		for _, id := range searchIDs {
			_ = s.DeleteVector(ctx, table, ownerSearch, id)
		}
		for _, id := range memIDs {
			_ = s.DeleteVector(ctx, table, ownerMemory, id)
		}
	}
	return nil
}

// deleteExpired removes expired rows from a store and returns their ids so the
// caller can cascade vector deletion. table is an internal constant, so the
// fmt-built SQL is safe.
func (s *Store) deleteExpired(ctx context.Context, table, now string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id FROM %s WHERE expires_at IS NOT NULL AND expires_at < ?`, table), now)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, id := range ids {
		if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, table), id); err != nil {
			return nil, err
		}
	}
	return ids, nil
}
