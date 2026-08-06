package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The seed holds three cached pages: the fixture page (short, 200, full
// content), a failed fetch (short, 404, no content) and an archive page (long,
// 200, no run).
func TestListScrapeCacheReturnsSizesNotBodies(t *testing.T) {
	env := seededEnv(t)

	entries, err := env.Store.ListScrapeCache(context.Background(), "", "", 0, 0)
	if err != nil {
		t.Fatalf("ListScrapeCache: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}

	byURL := map[string]ScrapeCacheSummary{}
	for _, e := range entries {
		byURL[e.URL] = e
	}
	page, ok := byURL[fixtureURL]
	if !ok {
		t.Fatal("the fixture page is missing")
	}
	if page.HTTPStatus != 200 || page.Tier != tierShort || page.HitCount != 3 {
		t.Errorf("fixture page = %+v", page)
	}
	if page.TextChars == 0 || page.CleanChars == 0 || page.RawChars == 0 {
		t.Errorf("content sizes missing: %+v", page)
	}
	if page.ContentHash != "fixturehash" || page.Title != "Fixture One" {
		t.Errorf("identity fields = %+v", page)
	}

	failed := byURL["https://example.org/fixture-two"]
	if failed.Error != "not found" || failed.HTTPStatus != 404 {
		t.Errorf("failed fetch = %+v", failed)
	}
	if failed.RobotsAllowed {
		t.Error("robots_allowed should be false for the seeded disallowed row")
	}

	// The listing must not carry the bodies themselves; those stay behind
	// /api/scrapes/{id}.
	blob, err := json.Marshal(entries)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	if strings.Contains(string(blob), "<html>") {
		t.Error("the listing leaked stored HTML")
	}
}

func TestListScrapeCacheFiltersByTierAndURL(t *testing.T) {
	env := seededEnv(t)
	ctx := context.Background()

	long, err := env.Store.ListScrapeCache(ctx, tierLong, "", 0, 0)
	if err != nil {
		t.Fatalf("ListScrapeCache: %v", err)
	}
	if len(long) != 1 || long[0].Tier != tierLong {
		t.Errorf("tier filter returned %+v", long)
	}

	// The URL filter is a substring, so a bare domain narrows to that host.
	domain, err := env.Store.ListScrapeCache(ctx, "", "example.org", 0, 0)
	if err != nil {
		t.Fatalf("ListScrapeCache: %v", err)
	}
	if len(domain) != 1 || domain[0].URL != "https://example.org/fixture-two" {
		t.Errorf("domain filter returned %+v", domain)
	}

	none, err := env.Store.ListScrapeCache(ctx, "", "no-such-host", 0, 0)
	if err != nil || len(none) != 0 {
		t.Errorf("unmatched filter: %d rows, err %v", len(none), err)
	}
}

func TestListScrapeCachePaginates(t *testing.T) {
	env := seededEnv(t)
	ctx := context.Background()

	first, err := env.Store.ListScrapeCache(ctx, "", "", 2, 0)
	if err != nil {
		t.Fatalf("ListScrapeCache: %v", err)
	}
	second, err := env.Store.ListScrapeCache(ctx, "", "", 2, 2)
	if err != nil {
		t.Fatalf("ListScrapeCache: %v", err)
	}
	if len(first) != 2 || len(second) != 1 {
		t.Fatalf("pages = %d and %d, want 2 then 1", len(first), len(second))
	}
	if first[0].ID == second[0].ID {
		t.Error("the two pages returned the same row")
	}
}

func TestListScrapeCacheEmpty(t *testing.T) {
	_, env := newTestServer(t)

	entries, err := env.Store.ListScrapeCache(context.Background(), "", "", 0, 0)
	if err != nil {
		t.Fatalf("an empty cache must not error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("entries = %d, want none", len(entries))
	}
}

func TestScrapeCacheEndpoint(t *testing.T) {
	env := seededEnv(t)
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cache/scrapes?q=example.net", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Count   int                  `json:"count"`
		Entries []ScrapeCacheSummary `json:"entries"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if body.Count != 1 || len(body.Entries) != 1 {
		t.Fatalf("body = %+v, want the one archive entry", body)
	}
	if body.Entries[0].Tier != tierLong {
		t.Errorf("tier = %q, want long", body.Entries[0].Tier)
	}
}
