package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

const statsJobSample = 200

func TestStatsCountsTheSeededData(t *testing.T) {
	env := seededEnv(t)

	stats, err := env.Store.Stats(context.Background(), statsJobSample)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Searches != 2 || stats.URLs != 2 || stats.MemoryFacts != 1 {
		t.Errorf("counts = %+v", stats)
	}
	if stats.Scrapes != 3 || stats.SearchCache != 2 {
		t.Errorf("cache counts = scrapes %d, search_cache %d", stats.Scrapes, stats.SearchCache)
	}
	if stats.PendingJobs != 1 {
		t.Errorf("pending_jobs = %d, want 1", stats.PendingJobs)
	}
	if stats.ScrapeTextAvgChars == 0 || stats.ScrapeRawMaxChars == 0 {
		t.Errorf("size aggregates missing: %+v", stats)
	}
}

// Every embedding-meta field is optional: a first run has none of them, and
// that is blank, not an error.
func TestStatsEmbedMetaIsBestEffort(t *testing.T) {
	env := seededEnv(t)

	stats, err := env.Store.Stats(context.Background(), statsJobSample)
	if err != nil {
		t.Fatalf("Stats with no meta set: %v", err)
	}
	if stats.EmbedModel != "" || stats.EmbedDim != 0 || stats.Migrating {
		t.Errorf("unset meta should leave the fields blank: %+v", stats)
	}
}

func TestStatsReportsModelDimAndMigration(t *testing.T) {
	env := seededEnv(t)
	ctx := context.Background()
	if err := seedVectors(ctx, env.Store); err != nil {
		t.Fatalf("seeding vectors: %v", err)
	}
	for key, value := range map[string]string{
		metaEmbedModel: "stub-embedder",
		metaEmbedDim:   strconv.Itoa(stubEmbedDim),
	} {
		if err := env.Store.MetaSet(ctx, key, value); err != nil {
			t.Fatalf("MetaSet %s: %v", key, err)
		}
	}

	stats, err := env.Store.Stats(ctx, statsJobSample)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.EmbedModel != "stub-embedder" || stats.EmbedDim != stubEmbedDim {
		t.Errorf("model/dim = %q/%d", stats.EmbedModel, stats.EmbedDim)
	}
	if stats.VectorTable != "vectors_test" || stats.Vectors != 2 {
		t.Errorf("vector table = %q with %d vectors", stats.VectorTable, stats.Vectors)
	}
	if stats.Migrating {
		t.Error("no migration is in flight")
	}

	if err := env.Store.MetaSet(ctx, metaMigration, "in-progress"); err != nil {
		t.Fatalf("MetaSet: %v", err)
	}
	stats, err = env.Store.Stats(ctx, statsJobSample)
	if err != nil {
		t.Fatalf("Stats during a migration: %v", err)
	}
	if !stats.Migrating {
		t.Error("an in-flight migration should be flagged")
	}
}

// The RISK T020 had to resolve: the schema records hit_count per row but never
// the lookups that missed, so the response carries counts and tier
// distributions — not a rate it cannot derive.
func TestStatsCacheTiersAndHitCounts(t *testing.T) {
	env := seededEnv(t)

	stats, err := env.Store.Stats(context.Background(), statsJobSample)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}

	search := stats.SearchCacheStats
	if search.Rows != 2 || search.Tiers[tierShort] != 1 || search.Tiers[tierLong] != 1 {
		t.Errorf("search cache stats = %+v", search)
	}
	if search.TotalHits != 7 || search.RowsWithHits != 1 {
		t.Errorf("search cache hits = %d over %d rows", search.TotalHits, search.RowsWithHits)
	}
	// A tier nothing occupies is still reported, at zero.
	if _, ok := search.Tiers[tierPermanent]; !ok {
		t.Error("every tier should be present in the distribution")
	}

	scrape := stats.ScrapeCacheStats
	if scrape.Rows != 3 || scrape.Tiers[tierShort] != 2 || scrape.Tiers[tierLong] != 1 {
		t.Errorf("scrape cache stats = %+v", scrape)
	}
	if scrape.TotalHits != 8 || scrape.RowsWithHits != 2 {
		t.Errorf("scrape cache hits = %d over %d rows", scrape.TotalHits, scrape.RowsWithHits)
	}
}

func TestStatsJobThroughput(t *testing.T) {
	env := seededEnv(t)

	stats, err := env.Store.Stats(context.Background(), statsJobSample)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	jobs := stats.Jobs
	for status, want := range map[string]int{jobPending: 1, jobRunning: 1, jobDone: 1, jobFailed: 1} {
		if jobs.ByStatus[status] != want {
			t.Errorf("by_status[%s] = %d, want %d", status, jobs.ByStatus[status], want)
		}
	}
	if jobs.ByType[jobTypeEmbed] != 2 || jobs.ByType[jobTypeDistill] != 1 {
		t.Errorf("by_type = %v", jobs.ByType)
	}
	if jobs.Retried != 1 || jobs.MaxAttempts != 3 {
		t.Errorf("retried = %d, max_attempts = %d", jobs.Retried, jobs.MaxAttempts)
	}
	if jobs.OldestPendingAt == "" {
		t.Error("a pending job should give the backlog an age")
	}
	// The one finished job was seeded 90s before it completed.
	if jobs.CompletedSampled != 1 || jobs.AvgCompletionMS < 80_000 {
		t.Errorf("completion timing = %d ms over %d jobs", jobs.AvgCompletionMS, jobs.CompletedSampled)
	}
}

func TestStatsJobTimingSampleCanBeDisabled(t *testing.T) {
	env := seededEnv(t)

	stats, err := env.Store.Stats(context.Background(), 0)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if stats.Jobs.CompletedSampled != 0 || stats.Jobs.AvgCompletionMS != 0 {
		t.Errorf("a zero sample should skip the timing entirely: %+v", stats.Jobs)
	}
	if stats.Jobs.ByStatus[jobDone] != 1 {
		t.Error("the counts should still be gathered")
	}
}

func TestStatsOnAnEmptyDatabase(t *testing.T) {
	_, env := newTestServer(t)

	stats, err := env.Store.Stats(context.Background(), statsJobSample)
	if err != nil {
		t.Fatalf("an empty database must not error: %v", err)
	}
	if stats.SearchCacheStats.Rows != 0 || stats.Jobs.ByStatus[jobPending] != 0 {
		t.Errorf("empty stats = %+v", stats)
	}
	if len(stats.ScrapeCacheStats.Tiers) == 0 {
		t.Error("the tier distribution should still have its shape")
	}
}

func TestStatsEndpointCarriesTheNewFields(t *testing.T) {
	env := seededEnv(t)
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/stats", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var raw map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	for _, key := range []string{
		"runs", "searches", "scrape_text_avg_chars", "vector_migration_in_progress",
		"search_cache_stats", "scrape_cache_stats", "jobs",
	} {
		if _, ok := raw[key]; !ok {
			t.Errorf("/api/stats is missing %q", key)
		}
	}
}
