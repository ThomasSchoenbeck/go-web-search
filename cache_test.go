package main

import (
	"testing"
	"time"
)

// These cover the pure logic that does not need a live database or model. The
// database, embedding and gating paths need a running Turso and llama.cpp and
// are verified manually.

func TestNormalizeQuery(t *testing.T) {
	cases := map[string]string{
		"  Hello   World  ":        "hello world",
		"When Do the FEVERS play":  "when do the fevers play",
		"\tmixed\n whitespace here": "mixed whitespace here",
	}
	for in, want := range cases {
		if got := normalizeQuery(in); got != want {
			t.Errorf("normalizeQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestContentHash(t *testing.T) {
	if a, b := contentHash("hello", ""), contentHash("hello", "ignored"); a != b {
		t.Fatal("text should dominate the hash over clean_html")
	}
	if contentHash("", "") != "" {
		t.Fatal("empty content should hash to empty string")
	}
	if contentHash("x", "") == contentHash("y", "") {
		t.Fatal("different content should hash differently")
	}
}

func TestVectorLiteral(t *testing.T) {
	if got, want := vectorLiteral([]float32{1, 0.5, -2}), "[1,0.5,-2]"; got != want {
		t.Fatalf("vectorLiteral = %q, want %q", got, want)
	}
	if got := vectorLiteral(nil); got != "[]" {
		t.Fatalf("empty vector = %q, want []", got)
	}
}

func TestJSONArray(t *testing.T) {
	if got := jsonArray("prefix [{\"a\":1}] suffix"); got != "[{\"a\":1}]" {
		t.Fatalf("jsonArray span = %q", got)
	}
	if got := jsonArray("no array here"); got != "" {
		t.Fatalf("jsonArray no match = %q", got)
	}
}

func TestRememberTier(t *testing.T) {
	if store, _ := rememberTier("off", "short"); store {
		t.Fatal("off should not store")
	}
	if store, tier := rememberTier("long", "short"); !store || tier != tierLong {
		t.Fatalf("explicit tier: store=%v tier=%q", store, tier)
	}
	if store, tier := rememberTier("", "short"); !store || tier != tierShort {
		t.Fatalf("default: store=%v tier=%q", store, tier)
	}
}

func TestCombineSources(t *testing.T) {
	if got := combineSources(map[string]bool{}); got != "live" {
		t.Fatalf("empty = %q", got)
	}
	if got := combineSources(map[string]bool{"cache": true}); got != "cache" {
		t.Fatalf("single = %q", got)
	}
	if got := combineSources(map[string]bool{"cache": true, "live": true}); got != "mixed" {
		t.Fatalf("multi = %q", got)
	}
}

func TestLooksLikeURL(t *testing.T) {
	for _, u := range []string{"https://example.com/x", "http://a.b"} {
		if !looksLikeURL(u) {
			t.Fatalf("%q should look like a URL", u)
		}
	}
	for _, u := range []string{"just text", "ftp://x", "example.com"} {
		if looksLikeURL(u) {
			t.Fatalf("%q should not look like a URL", u)
		}
	}
}

func TestVolatilityMaxAge(t *testing.T) {
	if volatilityMaxAge("time-sensitive") != 24*time.Hour {
		t.Fatal("time-sensitive should cap age")
	}
	if volatilityMaxAge("stable") != 0 {
		t.Fatal("stable should be governed only by tier expiry")
	}
}

func TestFreshEnough(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-time.Hour).Format(time.RFC3339Nano)
	future := now.Add(time.Hour).Format(time.RFC3339Nano)

	if freshEnough(past, past, 0) {
		t.Fatal("an expired row is not fresh")
	}
	if !freshEnough(future, past, 0) {
		t.Fatal("an unexpired row with no max-age is fresh")
	}
	if freshEnough(future, past, 30*time.Minute) {
		t.Fatal("fetched older than max-age is not fresh")
	}
	if !freshEnough("", now.Format(time.RFC3339Nano), 0) {
		t.Fatal("no expiry + recent fetch is fresh")
	}
}

func TestBoolOr(t *testing.T) {
	tr, fa := true, false
	if !boolOr(nil, true) || boolOr(nil, false) {
		t.Fatal("nil should take the default")
	}
	if !boolOr(&tr, false) || boolOr(&fa, true) {
		t.Fatal("non-nil should take the pointed value")
	}
}
