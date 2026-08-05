package main

import "context"

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
}

// Stats gathers the snapshot with a handful of scalar queries.
func (s *Store) Stats(ctx context.Context) (*StatsView, error) {
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
		_ = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&v.Vectors)
	}
	return v, nil
}
