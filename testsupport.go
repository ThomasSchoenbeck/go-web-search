package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Shared test scaffolding. It lives in a normal (non-_test) file because the
// browserless -mode testserve entry point in main.go uses it to back the
// Playwright end-to-end harness, which runs the real binary rather than `go
// test`. Nothing here is reachable from browse or serve mode.
//
// The hard rule this exists to enforce: every test — Go or Playwright — runs
// against a throwaway main DB and log DB under a temp directory that is deleted
// afterwards. No suite ever opens the developer's ./data.

// testEnv is a disposable harvester environment: its own data directory, main
// database and log database, all removed by Close.
type testEnv struct {
	Dir   string
	Cfg   Config
	Store *Store
	Logs  *LogStore
	Art   *artifacts

	ownsDir bool
}

// newTestEnv builds a throwaway environment under dir. An empty dir gets a
// fresh temp directory that Close deletes; an explicit dir is left in place for
// the caller (the Playwright harness) to remove, so teardown stays owned by
// whoever created it.
func newTestEnv(dir string) (*testEnv, error) {
	ownsDir := dir == ""
	if ownsDir {
		d, err := os.MkdirTemp("", "harvester-test-")
		if err != nil {
			return nil, fmt.Errorf("creating temp data dir: %w", err)
		}
		dir = d
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	env := &testEnv{Dir: dir, ownsDir: ownsDir}
	cfg := defaultConfig()
	cfg.Database.DataDir = dir
	// A test must never inherit the developer's endpoints or listener.
	cfg.Server.Addr = "127.0.0.1:0"
	cfg.Server.APIKey = ""
	// Recurring cleanup would fire mid-test for no benefit.
	cfg.Retention.VacuumOnStartup = false
	cfg.Retention.VacuumAt = ""
	env.Cfg = cfg

	store, err := openStore(cfg.Database.Driver, filepath.Join(dir, cfg.Database.MainDB), cfg.Database.MaxOpenConns, cfg.Database.AutoVacuum)
	if err != nil {
		env.Close()
		return nil, err
	}
	env.Store = store

	logs, err := openLogStore(cfg.Database.Driver, filepath.Join(dir, cfg.Database.LogDB))
	if err != nil {
		env.Close()
		return nil, err
	}
	env.Logs = logs

	// Test logs go nowhere by default; a failing test prints what it asserts on.
	env.Art = &artifacts{Log: log.New(io.Discard, "", 0)}
	return env, nil
}

// Server builds an apiServer over the throwaway stores, with no browser session
// — every observability route is read-only, so nothing here needs Chrome.
func (e *testEnv) Server() *apiServer {
	return newAPIServer(e.Cfg, newHarvester(e.Cfg, e.Store, e.Art.Log, nil), e.Logs)
}

// Seed writes a small, fully deterministic dataset: one finished run, one
// search with its SERP, two ranked URLs, a scraped page and a distilled fact.
// Enough for every read-only view to have something to render, written as plain
// SQL rather than through the pipeline so seeding needs neither Chrome nor a
// model endpoint.
func (e *testEnv) Seed(ctx context.Context) error { return seedTestData(ctx, e.Store) }

// SeedLogs fills the log database, which seedTestData does not touch: the two
// are separate files, and the logs viewer reads only the second one.
func (e *testEnv) SeedLogs(ctx context.Context) error { return seedTestLogs(ctx, e.Logs) }

func seedTestData(ctx context.Context, store *Store) error {
	const (
		runID     = "00000000-0000-7000-8000-000000000001"
		searchID  = "00000000-0000-7000-8000-000000000002"
		url1ID    = "00000000-0000-7000-8000-000000000003"
		url2ID    = "00000000-0000-7000-8000-000000000004"
		search2ID = "00000000-0000-7000-8000-00000000000c"
	)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	// A fixed distance into the past, so rows that need an age (a finished job,
	// a cache entry fetched before this moment) have a deterministic one.
	earlier := time.Now().UTC().Add(-90 * time.Second).Format(time.RFC3339Nano)
	stmts := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO runs (id, mode, started_at, finished_at, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{runID, "testserve", now, now, now}},
		{`INSERT INTO searches (id, run_id, term, engine, search_mode, landed_url, http_status, anchor_count, duration_ms, created_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{searchID, runID, "fixture term", "google", "typed", "https://www.google.com/search?q=fixture", 200, 2, 42, now}},
		{`INSERT INTO search_raw (id, search_id, html, byte_size, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000005", searchID, "<html><body>fixture SERP</body></html>", 38, now}},
		// A second search that was blocked and stored no SERP, so the views have
		// both the error state and the "no raw HTML" 404 to render.
		{`INSERT INTO searches (id, run_id, term, engine, search_mode, http_status, blocked, anchor_count, error, duration_ms, created_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{search2ID, runID, "fixture term", "bing", "direct", 429, 1, 0, "challenged by engine", 91, now}},
		{`INSERT INTO urls (id, url, domain, first_seen_at, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{url1ID, "https://example.com/fixture-one", "example.com", now, now}},
		{`INSERT INTO urls (id, url, domain, first_seen_at, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{url2ID, "https://example.org/fixture-two", "example.org", now, now}},
		{`INSERT INTO search_urls (id, search_id, url_id, rank, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000006", searchID, url1ID, 1, now}},
		{`INSERT INTO search_urls (id, search_id, url_id, rank, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000007", searchID, url2ID, 2, now}},
		{`INSERT INTO scrape_cache (id, url, run_id, http_status, content_type, fetched_with, title, raw_html, clean_html, text_content, images, content_hash, etag, last_modified, tier, hit_count, expires_at, fetched_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000008", "https://example.com/fixture-one", runID, 200, "text/html",
				"http", "Fixture One", "<html><body><h1>Fixture One</h1></body></html>", "<h1>Fixture One</h1>",
				"Fixture One", `[{"url":"https://example.com/one.png","alt":"Fixture image","width":320,"height":200}]`,
				"fixturehash", `W/"fixture-etag"`, "Wed, 05 Aug 2026 10:00:00 GMT", tierShort, 3, now, now, now, now}},
		// A failed scrape with no images and no content, for the graceful-empty
		// and error states.
		{`INSERT INTO scrape_cache (id, url, run_id, http_status, fetched_with, robots_allowed, error, tier, hit_count, fetched_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-00000000000d", "https://example.org/fixture-two", runID, 404,
				"http", 0, "not found", tierShort, 0, now, now, now}},
		{`INSERT INTO memory_facts (id, text, source_url, volatility, tier, fetched_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000009", "The fixture page says Fixture One.",
				"https://example.com/fixture-one", "stable", tierShort, now, now, now}},
		{`INSERT INTO search_cache (id, query_norm, query, results, tier, fetched_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-00000000000a", "fixture term", "fixture term",
				`["https://example.com/fixture-one"]`, tierShort, now, now, now}},
		// A second cached query at a promoted tier, so the cache browser's tier
		// filter has something to separate.
		{`INSERT INTO search_cache (id, query_norm, query, results, tier, hit_count, expires_at, fetched_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000011", "fixture archive", "Fixture Archive",
				`["https://example.org/fixture-two","https://example.net/fixture-archive"]`, tierLong, 7, now, earlier, earlier, now}},
		// A cached page from no particular run, at a promoted tier: the scrape
		// cache outlives the run that filled it, and the browser must show that.
		{`INSERT INTO scrape_cache (id, url, http_status, content_type, fetched_with, robots_allowed, title, clean_html, text_content, content_hash, tier, hit_count, expires_at, fetched_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000012", "https://example.net/fixture-archive", 200, "text/html",
				"http", 1, "Fixture Archive", "<h1>Fixture Archive</h1>", "Fixture Archive",
				"archivehash", tierLong, 5, now, earlier, earlier, now}},
		{`INSERT INTO jobs (id, type, payload, status, run_after, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-00000000000b", jobTypeDistill, "{}", "pending", now, now, now}},
		// One job per remaining status, so the monitor's breakdown and its
		// status/type filters have every case to render.
		{`INSERT INTO jobs (id, type, payload, status, attempts, run_after, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-00000000000e", jobTypeEmbed, `{"owner_kind":"memory"}`, jobFailed, 3, now, earlier, now}},
		{`INSERT INTO jobs (id, type, payload, status, run_after, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-00000000000f", jobTypeEmbed, `{"owner_kind":"search"}`, jobDone, earlier, earlier, now}},
		{`INSERT INTO jobs (id, type, payload, status, attempts, run_after, locked_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000010", jobTypeCleanup, "", jobRunning, 1, now, now, earlier, now}},
	}
	for _, s := range stmts {
		if _, err := store.db.ExecContext(ctx, s.query, s.args...); err != nil {
			return fmt.Errorf("seeding: %w", err)
		}
	}
	return nil
}

// seedTestLogs writes fixture lines straight into the log database rather than
// through LogStore.Write: that path is an asynchronous batching goroutine, and
// a test that has to wait out a flush interval before it can assert is a flaky
// test. Levels, sources and the presence of a run id all vary, so every filter
// the logs endpoint offers has something to select on.
func seedTestLogs(ctx context.Context, logs *LogStore) error {
	const runID = "00000000-0000-7000-8000-000000000001"
	base := time.Now().UTC().Add(-time.Minute)
	lines := []struct {
		id, runID, level, source, message string
	}{
		{"00000000-0000-7000-8000-000000000013", runID, "info", "harvester", "fixture run started"},
		{"00000000-0000-7000-8000-000000000014", runID, "notice", "harvester", "NOTE: fixture profile is new"},
		{"00000000-0000-7000-8000-000000000015", runID, "warn", "scraper", "WARNING: fixture page was slow"},
		// No run id: lines written outside a run must still be listable.
		{"00000000-0000-7000-8000-000000000016", "", "error", "scraper", "ERROR: fixture fetch failed"},
	}
	for i, l := range lines {
		at := base.Add(time.Duration(i) * time.Second).Format(time.RFC3339Nano)
		if _, err := logs.db.ExecContext(ctx,
			`INSERT INTO logs (id, run_id, level, source, message, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			l.id, nullable(l.runID), l.level, l.source, l.message, at); err != nil {
			return fmt.Errorf("seeding logs: %w", err)
		}
	}
	return nil
}

// stubEmbedder produces a deterministic vector from the text itself, so the
// semantic explorer can be exercised end to end with no model endpoint running.
// Nearness is meaningless — it is a hash, not a language model — but identical
// text always embeds identically, which is what makes the seeded fixtures and
// the query line up.
type stubEmbedder struct{}

const stubEmbedDim = 8

func (stubEmbedder) Embed(_ context.Context, texts []string, _ bool, _ string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i, text := range texts {
		out[i] = stubVector(text)
	}
	return out, nil
}

// stubVector spreads a checksum of the text across the dimensions and
// normalises, so cosine distance stays in its usual range.
func stubVector(text string) []float32 {
	vec := make([]float32, stubEmbedDim)
	for i, r := range text {
		vec[i%stubEmbedDim] += float32((int(r)%17)+1) / 16
	}
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm == 0 {
		vec[0] = 1
		return vec
	}
	norm = math.Sqrt(norm)
	for i := range vec {
		vec[i] = float32(float64(vec[i]) / norm)
	}
	return vec
}

// seedVectors gives the seeded fact and cached search an embedding under an
// active vector table, so the explorer has something to find.
func seedVectors(ctx context.Context, store *Store) error {
	const table = "vectors_test"
	if err := store.ensureVectorTable(ctx, table, stubEmbedDim); err != nil {
		return fmt.Errorf("seeding vector table: %w", err)
	}
	if err := store.MetaSet(ctx, metaVectorTable, table); err != nil {
		return err
	}
	seeds := []struct {
		kind, id, text string
	}{
		{ownerMemory, "00000000-0000-7000-8000-000000000009", "The fixture page says Fixture One."},
		{ownerSearch, "00000000-0000-7000-8000-00000000000a", "fixture term"},
	}
	for _, s := range seeds {
		if err := store.UpsertVector(ctx, table, s.kind, s.id, stubVector(s.text), "stub-embedder", stubEmbedDim); err != nil {
			return fmt.Errorf("seeding %s vector: %w", s.kind, err)
		}
	}
	return nil
}

// testServeMode backs `-mode testserve`: the REST + MCP + SPA surface over
// whatever -data points at, with no Chrome, no job runner and no vector boot.
// The Playwright harness needs a real listener but only exercises read-only
// routes, and leaving the background systems out keeps startup fast and keeps
// e2e runs from depending on a reachable LLM endpoint.
func testServeMode(cfg Config, art *artifacts, store *Store, logs *LogStore, stop context.Context) error {
	// The Playwright harness sets this so the views have something to render.
	if os.Getenv("HARVESTER_TEST_SEED") == "1" {
		ctx := context.Background()
		if err := seedTestData(ctx, store); err != nil {
			return err
		}
		if err := seedVectors(ctx, store); err != nil {
			return err
		}
		if err := seedTestLogs(ctx, logs); err != nil {
			return err
		}
		art.Log.Printf("testserve seeded fixture data")
	}

	srv := newAPIServer(cfg, newHarvester(cfg, store, art.Log, nil), logs)
	// No model endpoint is reachable in a test run, so the explorer embeds with
	// the deterministic stub instead.
	srv.embed = stubEmbedder{}

	errCh := make(chan error, 1)
	go func() {
		art.Log.Printf("testserve listening on http://%s", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-stop.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// Close releases both databases and removes the temp directory it created.
// Safe to call on a partially built env, and safe to call twice.
func (e *testEnv) Close() error {
	var firstErr error
	if e.Logs != nil {
		if _, err := e.Logs.Close(); err != nil {
			firstErr = err
		}
		e.Logs = nil
	}
	if e.Store != nil {
		if err := e.Store.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		e.Store = nil
	}
	if e.ownsDir && e.Dir != "" {
		if err := os.RemoveAll(e.Dir); err != nil && firstErr == nil {
			firstErr = err
		}
		e.Dir = ""
	}
	return firstErr
}
