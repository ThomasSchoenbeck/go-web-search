package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The seed holds two cached queries: "fixture term" (short tier, 1 result) and
// "Fixture Archive" (long tier, 2 results).
func TestListSearchCacheReturnsMetadataNotTheBlob(t *testing.T) {
	env := seededEnv(t)

	entries, err := env.Store.ListSearchCache(context.Background(), "", "", 0, 0)
	if err != nil {
		t.Fatalf("ListSearchCache: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}

	byQuery := map[string]SearchCacheSummary{}
	for _, e := range entries {
		byQuery[e.Query] = e
	}
	archive, ok := byQuery["Fixture Archive"]
	if !ok {
		t.Fatal("the long-tier entry is missing")
	}
	if archive.Tier != tierLong || archive.HitCount != 7 {
		t.Errorf("archive entry = %+v", archive)
	}
	if archive.ResultCount != 2 {
		t.Errorf("result_count = %d, want 2 — the summary, not the blob", archive.ResultCount)
	}
	if archive.ResultsChars == 0 {
		t.Error("results_chars should report the stored blob's size")
	}
	if archive.ExpiresAt == "" || archive.FetchedAt == "" {
		t.Errorf("expiry metadata missing: %+v", archive)
	}
}

func TestListSearchCacheFiltersByTierAndQuery(t *testing.T) {
	env := seededEnv(t)
	ctx := context.Background()

	long, err := env.Store.ListSearchCache(ctx, tierLong, "", 0, 0)
	if err != nil {
		t.Fatalf("ListSearchCache: %v", err)
	}
	if len(long) != 1 || long[0].Tier != tierLong {
		t.Errorf("tier filter returned %+v", long)
	}

	// The text filter runs against query_norm, so it is case-insensitive the
	// same way the cache key is.
	matched, err := env.Store.ListSearchCache(ctx, "", "ARCHIVE", 0, 0)
	if err != nil {
		t.Fatalf("ListSearchCache: %v", err)
	}
	if len(matched) != 1 || matched[0].Query != "Fixture Archive" {
		t.Errorf("text filter returned %+v", matched)
	}

	none, err := env.Store.ListSearchCache(ctx, "", "no-such-query", 0, 0)
	if err != nil || len(none) != 0 {
		t.Errorf("unmatched filter: %d rows, err %v", len(none), err)
	}
}

func TestListSearchCachePaginates(t *testing.T) {
	env := seededEnv(t)
	ctx := context.Background()

	first, err := env.Store.ListSearchCache(ctx, "", "", 1, 0)
	if err != nil {
		t.Fatalf("ListSearchCache: %v", err)
	}
	second, err := env.Store.ListSearchCache(ctx, "", "", 1, 1)
	if err != nil {
		t.Fatalf("ListSearchCache: %v", err)
	}
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("pages = %d and %d, want 1 each", len(first), len(second))
	}
	if first[0].ID == second[0].ID {
		t.Error("the two pages returned the same row")
	}
}

func TestListSearchCacheEmpty(t *testing.T) {
	_, env := newTestServer(t)

	entries, err := env.Store.ListSearchCache(context.Background(), "", "", 0, 0)
	if err != nil {
		t.Fatalf("an empty cache must not error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %d, want none", len(entries))
	}
}

func TestSearchCacheEndpoint(t *testing.T) {
	env := seededEnv(t)
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cache/searches?tier=long", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Count   int                  `json:"count"`
		Entries []SearchCacheSummary `json:"entries"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if body.Count != 1 || len(body.Entries) != 1 {
		t.Fatalf("body = %+v, want the one long-tier entry", body)
	}
	if body.Entries[0].ResultCount != 2 {
		t.Errorf("result_count = %d, want 2", body.Entries[0].ResultCount)
	}
}
