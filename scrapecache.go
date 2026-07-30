package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"
)

// scrapeCacheRow is the cache metadata for a URL, used to decide a hit and to
// drive conditional refresh.
type scrapeCacheRow struct {
	ID           string
	URL          string
	Status       int
	ContentType  string
	FetchedWith  string
	Title        string
	TextChars    int
	ImageCount   int
	ContentHash  string
	ETag         string
	LastModified string
	Tier         string
	HitCount     int
	ExpiresAt    string
	FetchedAt    string
}

// contentHash is a stable fingerprint of a page's cleaned content, used to tell
// whether a refetch actually changed anything.
func contentHash(text, cleanHTML string) string {
	src := text
	if src == "" {
		src = cleanHTML
	}
	if src == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(src))
	return hex.EncodeToString(sum[:])
}

// cacheOutcome builds the per-URL outcome for a cache hit. provenance is
// "cache" for a fresh hit and "cache-revalidated" for a 304.
func cacheOutcome(r *scrapeCacheRow, provenance string) ScrapeOutcome {
	return ScrapeOutcome{
		ScrapeID:    r.ID,
		URL:         r.URL,
		Status:      r.Status,
		FetchedWith: provenance,
		Title:       r.Title,
		TextChars:   r.TextChars,
		Images:      r.ImageCount,
	}
}

// SaveScrape upserts a scrape keyed by URL (the exact cache key), replacing the
// old insert-every-fetch behaviour. It stamps a content hash and a fresh tier
// window and stores images inline as JSON.
func (s *Store) SaveScrape(ctx context.Context, cfg TierConfig, rec ScrapeRecord) (string, error) {
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	hash := contentHash(rec.Text, rec.CleanHTML)
	imgs, err := json.Marshal(rec.Images)
	if err != nil {
		return "", err
	}
	allowed := 0
	if rec.RobotsAllowed {
		allowed = 1
	}
	newTier, exp, perm := nextExpiry(now, tierShort, 0, cfg)
	var expArg any
	if !perm {
		expArg = exp.Format(time.RFC3339Nano)
	}

	res, err := s.db.ExecContext(ctx,
		`UPDATE scrape_cache SET run_id = ?, http_status = ?, content_type = ?, fetched_with = ?,
		        robots_allowed = ?, title = ?, raw_html = ?, clean_html = ?, text_content = ?,
		        images = ?, content_hash = ?, etag = ?, last_modified = ?, error = ?,
		        tier = ?, hit_count = 0, expires_at = ?, fetched_at = ?, duration_ms = ?, updated_at = ?
		  WHERE url = ?`,
		nullable(rec.RunID), rec.HTTPStatus, rec.ContentType, rec.FetchedWith,
		allowed, rec.Title, rec.RawHTML, rec.CleanHTML, rec.Text,
		string(imgs), hash, rec.ETag, rec.LastModified, rec.Err,
		newTier, expArg, nowStr, rec.Duration.Milliseconds(), nowStr,
		rec.URL)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n > 0 {
		var id string
		if err := s.db.QueryRowContext(ctx, `SELECT id FROM scrape_cache WHERE url = ?`, rec.URL).Scan(&id); err != nil {
			return "", err
		}
		return id, nil
	}

	id := newID()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO scrape_cache (id, url, run_id, http_status, content_type, fetched_with,
		     robots_allowed, title, raw_html, clean_html, text_content, images, content_hash,
		     etag, last_modified, error, tier, hit_count, expires_at, fetched_at, duration_ms,
		     created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?, ?, ?, ?)`,
		id, rec.URL, nullable(rec.RunID), rec.HTTPStatus, rec.ContentType, rec.FetchedWith,
		allowed, rec.Title, rec.RawHTML, rec.CleanHTML, rec.Text, string(imgs), hash,
		rec.ETag, rec.LastModified, rec.Err, newTier, expArg, nowStr, rec.Duration.Milliseconds(),
		nowStr, nowStr); err != nil {
		return "", err
	}
	return id, nil
}

// LookupScrapeCache returns the cache row for a URL if present.
func (s *Store) LookupScrapeCache(ctx context.Context, rawURL string) (*scrapeCacheRow, bool, error) {
	var r scrapeCacheRow
	var status, hits sql.NullInt64
	var ct, fw, title, text, imgs, hash, etag, lm, exp sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, url, http_status, content_type, fetched_with, title, text_content, images,
		        content_hash, etag, last_modified, tier, hit_count, expires_at, fetched_at
		   FROM scrape_cache WHERE url = ?`, rawURL).
		Scan(&r.ID, &r.URL, &status, &ct, &fw, &title, &text, &imgs,
			&hash, &etag, &lm, &r.Tier, &hits, &exp, &r.FetchedAt)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	r.Status = int(status.Int64)
	r.ContentType = ct.String
	r.FetchedWith = fw.String
	r.Title = title.String
	r.TextChars = len(text.String)
	r.ContentHash = hash.String
	r.ETag = etag.String
	r.LastModified = lm.String
	r.HitCount = int(hits.Int64)
	r.ExpiresAt = exp.String
	if imgs.String != "" {
		var arr []imageRef
		if json.Unmarshal([]byte(imgs.String), &arr) == nil {
			r.ImageCount = len(arr)
		}
	}
	return &r, true, nil
}

// touchScrapeCache records a hit: it bumps hit_count and slides the expiry
// window forward. refreshFetched is set on a 304 revalidation, where the server
// confirmed the copy is current, so fetched_at moves too.
func (s *Store) touchScrapeCache(ctx context.Context, cfg TierConfig, r *scrapeCacheRow, refreshFetched bool) error {
	hits := r.HitCount + 1
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	newTier, exp, perm := nextExpiry(now, r.Tier, hits, cfg)
	var expArg any
	if !perm {
		expArg = exp.Format(time.RFC3339Nano)
	}
	var err error
	if refreshFetched {
		_, err = s.db.ExecContext(ctx,
			`UPDATE scrape_cache SET hit_count = ?, tier = ?, expires_at = ?, fetched_at = ?, updated_at = ? WHERE id = ?`,
			hits, newTier, expArg, nowStr, nowStr, r.ID)
	} else {
		_, err = s.db.ExecContext(ctx,
			`UPDATE scrape_cache SET hit_count = ?, tier = ?, expires_at = ?, updated_at = ? WHERE id = ?`,
			hits, newTier, expArg, nowStr, r.ID)
	}
	r.HitCount = hits
	r.Tier = newTier
	return err
}

// GetScrape returns one cached scrape by id. includeRaw is opt-in because the
// raw HTML is usually far larger than everything else combined.
func (s *Store) GetScrape(ctx context.Context, scrapeID string, includeRaw bool) (*ScrapeDetail, error) {
	var d ScrapeDetail
	var robots int
	var status, dur sql.NullInt64
	var runID, ct, fw, title, clean, text, raw, imgs, errStr sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, url, run_id, http_status, content_type, fetched_with, robots_allowed,
		        title, clean_html, text_content, raw_html, images, error, duration_ms, created_at
		   FROM scrape_cache WHERE id = ?`, scrapeID).
		Scan(&d.ID, &d.URL, &runID, &status, &ct, &fw, &robots, &title, &clean, &text, &raw,
			&imgs, &errStr, &dur, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	d.RunID = runID.String
	d.HTTPStatus = int(status.Int64)
	d.ContentType = ct.String
	d.FetchedWith = fw.String
	d.RobotsAllowed = robots == 1
	d.Title = title.String
	d.CleanHTML = clean.String
	d.Text = text.String
	d.Error = errStr.String
	d.DurationMS = dur.Int64
	if includeRaw {
		d.RawHTML = raw.String
	}
	if imgs.String != "" {
		var arr []imageRef
		if json.Unmarshal([]byte(imgs.String), &arr) == nil {
			for _, im := range arr {
				d.Images = append(d.Images, ImageRow{URL: im.URL, Alt: im.Alt, Width: im.Width, Height: im.Height})
			}
		}
	}
	return &d, nil
}

// RunScrapeIDs lists the scrape ids attributed to a run.
func (s *Store) RunScrapeIDs(ctx context.Context, runID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM scrape_cache WHERE run_id = ? ORDER BY created_at`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
