package main

import (
	"context"
	"database/sql"
	"strconv"
	"time"
)

// StatsView is a high-level snapshot of what the harvester has accumulated,
// plus scrape-size aggregates to spot bloat in the raw material we distil from.
type StatsView struct {
	Runs        int `json:"runs"`
	Searches    int `json:"searches"`
	URLs        int `json:"urls"`
	Scrapes     int `json:"scrapes"`
	MemoryFacts int `json:"memory_facts"`
	SearchCache int `json:"search_cache"`
	Vectors     int `json:"vectors"`
	PendingJobs int `json:"pending_jobs"`

	// Bloat signal: how large the scraped pages are. distill only sends the
	// first distillMaxChars of text_content, so a text average far above that
	// means a lot is being truncated; a large raw/text ratio means the cleaner
	// is leaving (or the pages carry) a lot of non-content.
	ScrapeTextAvgChars int `json:"scrape_text_avg_chars"`
	ScrapeTextMaxChars int `json:"scrape_text_max_chars"`
	ScrapeRawAvgChars  int `json:"scrape_raw_avg_chars"`
	ScrapeRawMaxChars  int `json:"scrape_raw_max_chars"`

	// Embedding state, read best-effort from system_meta: an unset key or an
	// in-flight migration leaves the field blank rather than failing the whole
	// snapshot. While Migrating is true the vector count is the old generation's.
	EmbedModel  string `json:"embed_model,omitempty"`
	EmbedDim    int    `json:"embed_dim,omitempty"`
	VectorTable string `json:"vector_table,omitempty"`
	Migrating   bool   `json:"vector_migration_in_progress"`

	SearchCacheStats CacheStats `json:"search_cache_stats"`
	ScrapeCacheStats CacheStats `json:"scrape_cache_stats"`
	Jobs             JobStats   `json:"jobs"`
}

// CacheStats describes one cache's shape: how many rows it holds, how they are
// spread across the durability tiers, and how often they have been reused.
//
// These are hit *counts*, not a hit rate. A rate needs total lookups — hits
// plus misses — and the schema records only hit_count on the rows that exist;
// a miss leaves no trace to count. Deriving a real rate would mean writing new
// counters on every lookup, which is a change to app state the read-only v1
// deliberately does not make (T020's stated fallback: counts and tiers).
type CacheStats struct {
	Rows         int            `json:"rows"`
	TotalHits    int            `json:"total_hits"`
	RowsWithHits int            `json:"rows_with_hits"`
	Expired      int            `json:"expired"`
	Tiers        map[string]int `json:"tiers"`
}

// JobStats summarises the background queue: what it holds, how much of it has
// had to be retried, and how long finished work took. Every field is derived
// from columns the job system already writes.
type JobStats struct {
	ByStatus map[string]int `json:"by_status"`
	ByType   map[string]int `json:"by_type"`
	// Retried counts rows that needed more than one attempt; MaxAttempts is the
	// worst case seen. Together they say whether the queue is fighting itself.
	Retried     int `json:"retried"`
	MaxAttempts int `json:"max_attempts"`
	// OldestPendingAt is when the longest-waiting runnable job was created —
	// the queue's backlog age.
	OldestPendingAt string `json:"oldest_pending_at,omitempty"`
	// AvgCompletionMS averages created_at → updated_at over the most recent
	// CompletedSampled finished jobs. Sampled, not exhaustive: the whole history
	// would be an unbounded scan for a number nobody reads that precisely.
	CompletedSampled int   `json:"completed_sampled"`
	AvgCompletionMS  int64 `json:"avg_completion_ms"`
}

// Stats gathers the snapshot with a handful of scalar queries. jobSample bounds
// the finished-job timing sample (config: observability.job_timing_sample).
func (s *Store) Stats(ctx context.Context, jobSample int) (*StatsView, error) {
	v := &StatsView{}
	scalars := []struct {
		dst   *int
		query string
	}{
		{&v.Runs, `SELECT COUNT(*) FROM runs`},
		{&v.Searches, `SELECT COUNT(*) FROM searches`},
		{&v.URLs, `SELECT COUNT(*) FROM urls`},
		{&v.Scrapes, `SELECT COUNT(*) FROM scrape_cache`},
		{&v.MemoryFacts, `SELECT COUNT(*) FROM memory_facts`},
		{&v.SearchCache, `SELECT COUNT(*) FROM search_cache`},
		{&v.PendingJobs, `SELECT COUNT(*) FROM jobs WHERE status = 'pending'`},
		{&v.ScrapeTextAvgChars, `SELECT COALESCE(CAST(AVG(length(text_content)) AS INTEGER), 0) FROM scrape_cache`},
		{&v.ScrapeTextMaxChars, `SELECT COALESCE(MAX(length(text_content)), 0) FROM scrape_cache`},
		{&v.ScrapeRawAvgChars, `SELECT COALESCE(CAST(AVG(length(raw_html)) AS INTEGER), 0) FROM scrape_cache`},
		{&v.ScrapeRawMaxChars, `SELECT COALESCE(MAX(length(raw_html)), 0) FROM scrape_cache`},
	}
	for _, sc := range scalars {
		if err := s.db.QueryRowContext(ctx, sc.query).Scan(sc.dst); err != nil {
			return nil, err
		}
	}
	// Vectors live in the active generation table, whose name is config-driven
	// and internally generated (safe to interpolate). Best-effort: a migration
	// in flight or a missing table just leaves the count at zero.
	if table, ok, err := s.MetaGet(ctx, metaVectorTable); err == nil && ok && table != "" {
		v.VectorTable = table
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&v.Vectors)
	}
	if err := s.embedMeta(ctx, v); err != nil {
		return nil, err
	}

	var err error
	if v.SearchCacheStats, err = s.cacheStats(ctx, "search_cache"); err != nil {
		return nil, err
	}
	if v.ScrapeCacheStats, err = s.cacheStats(ctx, "scrape_cache"); err != nil {
		return nil, err
	}
	if v.Jobs, err = s.jobStats(ctx, jobSample); err != nil {
		return nil, err
	}
	return v, nil
}

// embedMeta fills in the active embedding model, its dimension and whether a
// re-embed is running. Every key is optional: on a first run none of them
// exist yet, which is a blank field rather than an error.
func (s *Store) embedMeta(ctx context.Context, v *StatsView) error {
	model, _, err := s.MetaGet(ctx, metaEmbedModel)
	if err != nil {
		return err
	}
	v.EmbedModel = model

	dim, _, err := s.MetaGet(ctx, metaEmbedDim)
	if err != nil {
		return err
	}
	v.EmbedDim, _ = strconv.Atoi(dim)

	migration, _, err := s.MetaGet(ctx, metaMigration)
	if err != nil {
		return err
	}
	v.Migrating = migration != ""
	return nil
}

// cacheStats counts one cache's rows, hits, expiries and tier spread. Both
// caches carry the same tier/hit_count/expires_at columns, so one query shape
// serves both; table is a constant from the caller, never user input.
func (s *Store) cacheStats(ctx context.Context, table string) (CacheStats, error) {
	stats := CacheStats{Tiers: map[string]int{}}
	for _, tier := range []string{tierShort, tierLong, tierPermanent} {
		stats.Tiers[tier] = 0
	}
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(hit_count), 0), COALESCE(SUM(hit_count > 0), 0),
		        COALESCE(SUM(expires_at IS NOT NULL AND expires_at < ?), 0)
		   FROM `+table, nowRFC3339()).
		Scan(&stats.Rows, &stats.TotalHits, &stats.RowsWithHits, &stats.Expired)
	if err != nil {
		return stats, err
	}

	rows, err := s.db.QueryContext(ctx, `SELECT tier, COUNT(*) FROM `+table+` GROUP BY tier`)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var tier string
		var n int
		if err := rows.Scan(&tier, &n); err != nil {
			return stats, err
		}
		stats.Tiers[tier] = n
	}
	return stats, rows.Err()
}

// jobStats summarises the queue. The completion timing is averaged in Go rather
// than in SQL: the timestamps are RFC3339 strings with nanosecond precision,
// which SQLite's date functions do not parse reliably.
func (s *Store) jobStats(ctx context.Context, sample int) (JobStats, error) {
	stats := JobStats{ByStatus: map[string]int{}, ByType: map[string]int{}}
	byStatus, err := s.JobStatusCounts(ctx)
	if err != nil {
		return stats, err
	}
	stats.ByStatus = byStatus

	rows, err := s.db.QueryContext(ctx, `SELECT type, COUNT(*) FROM jobs GROUP BY type`)
	if err != nil {
		return stats, err
	}
	for rows.Next() {
		var jobType string
		var n int
		if err := rows.Scan(&jobType, &n); err != nil {
			rows.Close()
			return stats, err
		}
		stats.ByType[jobType] = n
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return stats, err
	}

	if err := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(SUM(attempts > 1), 0), COALESCE(MAX(attempts), 0) FROM jobs`).
		Scan(&stats.Retried, &stats.MaxAttempts); err != nil {
		return stats, err
	}
	// MIN over no rows is NULL, which is an empty queue, not an error.
	var oldest sql.NullString
	if err := s.db.QueryRowContext(ctx,
		`SELECT MIN(created_at) FROM jobs WHERE status = 'pending'`).Scan(&oldest); err != nil {
		return stats, err
	}
	stats.OldestPendingAt = oldest.String

	if sample <= 0 {
		return stats, nil
	}
	timings, err := s.db.QueryContext(ctx,
		`SELECT created_at, updated_at FROM jobs WHERE status = 'done'
		  ORDER BY updated_at DESC LIMIT ?`, sample)
	if err != nil {
		return stats, err
	}
	defer timings.Close()

	var total time.Duration
	for timings.Next() {
		var created, updated string
		if err := timings.Scan(&created, &updated); err != nil {
			return stats, err
		}
		start, errStart := time.Parse(time.RFC3339Nano, created)
		end, errEnd := time.Parse(time.RFC3339Nano, updated)
		if errStart != nil || errEnd != nil || end.Before(start) {
			continue // a hand-edited or malformed row should not skew the average
		}
		stats.CompletedSampled++
		total += end.Sub(start)
	}
	if err := timings.Err(); err != nil {
		return stats, err
	}
	if stats.CompletedSampled > 0 {
		stats.AvgCompletionMS = total.Milliseconds() / int64(stats.CompletedSampled)
	}
	return stats, nil
}
