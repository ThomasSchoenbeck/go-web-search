package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
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
	return newAPIServer(e.Cfg, newHarvester(e.Cfg, e.Store, e.Art.Log, nil))
}

// Seed writes a small, fully deterministic dataset: one finished run, one
// search with its SERP, two ranked URLs, a scraped page and a distilled fact.
// Enough for every read-only view to have something to render, written as plain
// SQL rather than through the pipeline so seeding needs neither Chrome nor a
// model endpoint.
func (e *testEnv) Seed(ctx context.Context) error { return seedTestData(ctx, e.Store) }

func seedTestData(ctx context.Context, store *Store) error {
	const (
		runID    = "00000000-0000-7000-8000-000000000001"
		searchID = "00000000-0000-7000-8000-000000000002"
		url1ID   = "00000000-0000-7000-8000-000000000003"
		url2ID   = "00000000-0000-7000-8000-000000000004"
	)
	now := time.Now().UTC().Format(time.RFC3339Nano)
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
		{`INSERT INTO urls (id, url, domain, first_seen_at, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{url1ID, "https://example.com/fixture-one", "example.com", now, now}},
		{`INSERT INTO urls (id, url, domain, first_seen_at, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{url2ID, "https://example.org/fixture-two", "example.org", now, now}},
		{`INSERT INTO search_urls (id, search_id, url_id, rank, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000006", searchID, url1ID, 1, now}},
		{`INSERT INTO search_urls (id, search_id, url_id, rank, created_at) VALUES (?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000007", searchID, url2ID, 2, now}},
		{`INSERT INTO scrape_cache (id, url, run_id, http_status, content_type, fetched_with, title, raw_html, clean_html, text_content, images, content_hash, tier, fetched_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000008", "https://example.com/fixture-one", runID, 200, "text/html",
				"http", "Fixture One", "<html><body><h1>Fixture One</h1></body></html>", "<h1>Fixture One</h1>",
				"Fixture One", "[]", "fixturehash", tierShort, now, now, now}},
		{`INSERT INTO memory_facts (id, text, source_url, volatility, tier, fetched_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-000000000009", "The fixture page says Fixture One.",
				"https://example.com/fixture-one", "stable", tierShort, now, now, now}},
		{`INSERT INTO search_cache (id, query_norm, query, results, tier, fetched_at, created_at, updated_at)
		  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-00000000000a", "fixture term", "fixture term",
				`["https://example.com/fixture-one"]`, tierShort, now, now, now}},
		{`INSERT INTO jobs (id, type, payload, status, run_after, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			[]any{"00000000-0000-7000-8000-00000000000b", jobTypeDistill, "{}", "pending", now, now, now}},
	}
	for _, s := range stmts {
		if _, err := store.db.ExecContext(ctx, s.query, s.args...); err != nil {
			return fmt.Errorf("seeding: %w", err)
		}
	}
	return nil
}

// testServeMode backs `-mode testserve`: the REST + MCP + SPA surface over
// whatever -data points at, with no Chrome, no job runner and no vector boot.
// The Playwright harness needs a real listener but only exercises read-only
// routes, and leaving the background systems out keeps startup fast and keeps
// e2e runs from depending on a reachable LLM endpoint.
func testServeMode(cfg Config, art *artifacts, store *Store, stop context.Context) error {
	// The Playwright harness sets this so the views have something to render.
	if os.Getenv("HARVESTER_TEST_SEED") == "1" {
		if err := seedTestData(context.Background(), store); err != nil {
			return err
		}
		art.Log.Printf("testserve seeded fixture data")
	}

	srv := newAPIServer(cfg, newHarvester(cfg, store, art.Log, nil))

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
