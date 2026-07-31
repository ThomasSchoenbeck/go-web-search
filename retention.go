package main

import (
	"context"
	"fmt"
	"time"
)

// TrimRawContent is the unified sweep that clears large, already-processed blobs
// nothing reads after processing: a scrape's raw_html/clean_html (nulled, but
// text_content, etag and last_modified are kept, so re-distill and conditional
// refresh still work) and a search's SERP HTML (its side-table row deleted). A
// blob is cleared only once it is older than maxAge AND not among the newest
// keepLast rows of its kind, which stay for debugging. Nulling frees pages for
// reuse; a later VACUUM (or auto_vacuum) actually shrinks the file.
func (s *Store) TrimRawContent(ctx context.Context, maxAge time.Duration, keepLast int) error {
	if keepLast < 0 {
		keepLast = 0
	}
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339Nano)

	if _, err := s.db.ExecContext(ctx,
		`UPDATE scrape_cache SET raw_html = NULL, clean_html = NULL
		  WHERE created_at < ?
		    AND id NOT IN (SELECT id FROM scrape_cache ORDER BY created_at DESC LIMIT ?)
		    AND (raw_html IS NOT NULL OR clean_html IS NOT NULL)`, cutoff, keepLast); err != nil {
		return fmt.Errorf("trim scrape_cache: %w", err)
	}
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM search_raw
		  WHERE created_at < ?
		    AND id NOT IN (SELECT id FROM search_raw ORDER BY created_at DESC LIMIT ?)`, cutoff, keepLast); err != nil {
		return fmt.Errorf("trim search_raw: %w", err)
	}
	return nil
}

// Vacuum rewrites the database file to return freed pages to the OS. It holds an
// exclusive lock for its whole duration, so it is best run at startup or off
// peak, not while the app is under load.
func (s *Store) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, "VACUUM")
	return err
}
