package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// seededLogEnv gives the log database its fixture lines. seedTestData writes
// only to the main database — the two are separate files.
func seededLogEnv(t *testing.T) *testEnv {
	t.Helper()
	_, env := newTestServer(t)
	if err := env.SeedLogs(context.Background()); err != nil {
		t.Fatalf("seeding logs: %v", err)
	}
	return env
}

func TestLogQueryReturnsNewestFirst(t *testing.T) {
	env := seededLogEnv(t)

	entries, err := env.Logs.Query(context.Background(), LogQuery{})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 4 {
		t.Fatalf("entries = %d, want 4", len(entries))
	}
	if entries[0].Level != "error" {
		t.Errorf("first entry = %+v, want the newest (the error line)", entries[0])
	}
	for i := 1; i < len(entries); i++ {
		if entries[i-1].CreatedAt < entries[i].CreatedAt {
			t.Errorf("entries out of order at %d: %q before %q", i, entries[i-1].CreatedAt, entries[i].CreatedAt)
		}
	}
	if entries[0].Message == "" || entries[0].Source == "" {
		t.Errorf("every field should be populated: %+v", entries[0])
	}
	// The newest line was written outside a run, so its run id is empty rather
	// than the string "NULL".
	if entries[0].RunID != "" {
		t.Errorf("run_id = %q, want empty for a line written outside a run", entries[0].RunID)
	}
}

func TestLogQueryFilters(t *testing.T) {
	env := seededLogEnv(t)
	ctx := context.Background()

	byRun, err := env.Logs.Query(ctx, LogQuery{RunID: fixtureRunID})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(byRun) != 3 {
		t.Errorf("run filter = %d rows, want 3", len(byRun))
	}

	byLevel, err := env.Logs.Query(ctx, LogQuery{Level: "warn"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(byLevel) != 1 || byLevel[0].Level != "warn" {
		t.Errorf("level filter = %+v", byLevel)
	}

	bySource, err := env.Logs.Query(ctx, LogQuery{Source: "scraper"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(bySource) != 2 {
		t.Errorf("source filter = %d rows, want 2", len(bySource))
	}

	combined, err := env.Logs.Query(ctx, LogQuery{RunID: fixtureRunID, Source: "scraper"})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(combined) != 1 || combined[0].Level != "warn" {
		t.Errorf("combined filter = %+v", combined)
	}

	none, err := env.Logs.Query(ctx, LogQuery{Level: "no-such-level"})
	if err != nil {
		t.Fatalf("an unmatched filter must not error: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("unmatched filter returned %d rows", len(none))
	}
}

func TestLogQueryPaginates(t *testing.T) {
	env := seededLogEnv(t)
	ctx := context.Background()

	first, err := env.Logs.Query(ctx, LogQuery{Limit: 2})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	second, err := env.Logs.Query(ctx, LogQuery{Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("pages = %d and %d, want 2 each", len(first), len(second))
	}
	if first[1].ID == second[0].ID {
		t.Error("the offset did not advance the page")
	}
}

func TestLogQueryEmptyDatabase(t *testing.T) {
	_, env := newTestServer(t) // no log fixtures

	entries, err := env.Logs.Query(context.Background(), LogQuery{})
	if err != nil {
		t.Fatalf("an empty log database must not error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %d, want none", len(entries))
	}
}

// The written lines must be readable back through the same store, which is what
// the endpoint serves in a real deployment.
func TestLogWriteIsReadableBack(t *testing.T) {
	_, env := newTestServer(t)

	env.Logs.Write(fixtureRunID, "info", "test", "a line written through the batching writer")

	// The writer flushes on a timer, so poll rather than assume it has landed.
	deadline := time.Now().Add(10 * time.Second)
	for {
		entries, err := env.Logs.Query(context.Background(), LogQuery{Source: "test"})
		if err != nil {
			t.Fatalf("Query: %v", err)
		}
		if len(entries) == 1 {
			if entries[0].RunID != fixtureRunID {
				t.Errorf("run_id = %q, want the writer's current run", entries[0].RunID)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the written line never became readable")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// ---- endpoint level ----

func TestLogsEndpoint(t *testing.T) {
	env := seededLogEnv(t)
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/logs?level=error", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Count   int        `json:"count"`
		Entries []LogEntry `json:"entries"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if body.Count != 1 || len(body.Entries) != 1 {
		t.Fatalf("body = %+v, want the one error line", body)
	}
	if body.Entries[0].Level != "error" {
		t.Errorf("level = %q, want error", body.Entries[0].Level)
	}
}

// The log database is a separate file the server did not hold until this task
// wired it in; an empty one is a normal 200.
func TestLogsEndpointEmpty(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/logs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if body.Count != 0 {
		t.Errorf("count = %d, want 0", body.Count)
	}
}
