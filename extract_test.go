package main

import (
	"encoding/base64"
	"net/url"
	"testing"
)

func TestDestinations(t *testing.T) {
	bingWrapped := "https://www.bing.com/ck/a?!&&p=1&u=a1" +
		base64.RawURLEncoding.EncodeToString([]byte("https://example.org/bing-result"))

	raw := []string{
		"https://www.google.com/search?q=test",                         // engine infrastructure
		"https://policies.google.com/privacy",                          // engine infrastructure
		"https://duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fa", // ddg wrapper
		bingWrapped,                        // bing wrapper
		"https://example.net/page#section", // fragment stripped
		"https://example.net/page",         // duplicate of the previous one
		"javascript:void(0)",               // not a destination
		"https://gstatic.com/asset.js",     // shared infrastructure
	}

	got := destinations(raw, []string{"google", "duckduckgo", "bing", "gstatic"}, nil)
	want := []string{
		"https://example.com/a",
		"https://example.org/bing-result",
		"https://example.net/page",
	}

	if len(got) != len(want) {
		t.Fatalf("got %d urls %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSearchURLOnlyAddsCountWhenAsked(t *testing.T) {
	var google, ddg engineDef
	for _, e := range allEngines() {
		switch e.name {
		case "google":
			google = e
		case "duckduckgo":
			ddg = e
		}
	}

	if got, want := google.searchURL("go test", 0), "https://www.google.com/search?q=go+test"; got != want {
		t.Errorf("count 0: got %q, want %q", got, want)
	}
	if got, want := google.searchURL("go test", 20), "https://www.google.com/search?q=go+test&num=20"; got != want {
		t.Errorf("count 20: got %q, want %q", got, want)
	}
	// DuckDuckGo has no count parameter, so a count must not invent one.
	if got, want := ddg.searchURL("go test", 20), "https://duckduckgo.com/?q=go+test"; got != want {
		t.Errorf("ddg: got %q, want %q", got, want)
	}
}

func TestParseResultCounts(t *testing.T) {
	counts, err := parseResultCounts("google=20, ddg=5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if counts["google"] != 20 {
		t.Errorf("google: got %d, want 20", counts["google"])
	}
	if counts["duckduckgo"] != 5 {
		t.Errorf("ddg alias should normalise to duckduckgo, got %d", counts["duckduckgo"])
	}
	if _, err := parseResultCounts("google"); err == nil {
		t.Error("expected an error for a missing =count")
	}
}

func TestParseEngineSet(t *testing.T) {
	if set := parseEngineSet("none"); len(set) != 0 {
		t.Errorf("none should be empty, got %v", set)
	}
	if set := parseEngineSet("google,ddg"); !set["google"] || !set["duckduckgo"] {
		t.Errorf("got %v", set)
	}
}

func TestLandedQueryMatching(t *testing.T) {
	landed := "https://www.google.com/search?q=golang+web+scraping&sca_esv=abc123&source=hp"
	if got := landedQuery(landed); got != "golang web scraping" {
		t.Errorf("got %q", got)
	}
	if !sameQuery("Golang  Web Scraping", "golang web scraping") {
		t.Error("normalised comparison should match")
	}
	if sameQuery("golang web scraping", "golang web scraper") {
		t.Error("different queries must not match")
	}
}

func TestGroupByHost(t *testing.T) {
	groups := groupByHost([]string{
		"https://a.example/1",
		"https://a.example/2",
		"https://b.example/x",
		"not a url at all",
	})
	if len(groups["a.example"]) != 2 {
		t.Errorf("a.example: got %v", groups["a.example"])
	}
	if len(groups["b.example"]) != 1 {
		t.Errorf("b.example: got %v", groups["b.example"])
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 hosts, got %d: %v", len(groups), groups)
	}
}

func TestResolveRef(t *testing.T) {
	base, err := url.Parse("https://example.com/blog/post.html")
	if err != nil {
		t.Fatal(err)
	}
	cases := map[string]string{
		"/img/a.png":                  "https://example.com/img/a.png",
		"b.png":                       "https://example.com/blog/b.png",
		"//cdn.example.net/c.png":     "https://cdn.example.net/c.png",
		"https://other.example/d.png": "https://other.example/d.png",
		"data:image/png;base64,AAAA":  "", // non-http schemes are dropped
	}
	for ref, want := range cases {
		if got := resolveRef(base, ref); got != want {
			t.Errorf("%q: got %q, want %q", ref, got, want)
		}
	}
}

func TestCollapseWhitespace(t *testing.T) {
	in := "\n\n  Hello   world  \n\n\n\n   Next    line \n\n"
	if got, want := collapseWhitespace(in), "Hello world\n\nNext line"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestIsHTML(t *testing.T) {
	for _, ct := range []string{"", "text/html", "text/html; charset=utf-8", "application/xhtml+xml"} {
		if !isHTML(ct) {
			t.Errorf("%q should be html", ct)
		}
	}
	for _, ct := range []string{"application/pdf", "image/png", "application/json"} {
		if isHTML(ct) {
			t.Errorf("%q should not be html", ct)
		}
	}
}
