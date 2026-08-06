package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const (
	fixtureURL    = "https://example.com/fixture-one"
	fixtureRunID  = "00000000-0000-7000-8000-000000000001"
	fixtureFactID = "00000000-0000-7000-8000-000000000009"
)

// seededEnv gives a throwaway environment already carrying the fixture dataset.
func seededEnv(t *testing.T) *testEnv {
	t.Helper()
	_, env := newTestServer(t)
	if err := env.Seed(context.Background()); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	return env
}

func TestSearchesFindingURL(t *testing.T) {
	env := seededEnv(t)

	found, err := env.Store.SearchesFindingURL(context.Background(), fixtureURL)
	if err != nil {
		t.Fatalf("SearchesFindingURL: %v", err)
	}
	if len(found) != 1 {
		t.Fatalf("got %d finding searches, want 1", len(found))
	}
	if found[0].Engine != "google" || found[0].Rank != 1 || found[0].RunID != fixtureRunID {
		t.Errorf("unexpected search: %+v", found[0])
	}
}

func TestSearchesFindingURLUnknown(t *testing.T) {
	env := seededEnv(t)

	found, err := env.Store.SearchesFindingURL(context.Background(), "https://nobody.example/nope")
	if err != nil {
		t.Fatalf("unknown URL should not error: %v", err)
	}
	if len(found) != 0 {
		t.Errorf("got %d results for an unknown URL, want 0", len(found))
	}
}

func TestFactsBySourceURL(t *testing.T) {
	env := seededEnv(t)

	facts, err := env.Store.FactsBySourceURL(context.Background(), fixtureURL)
	if err != nil {
		t.Fatalf("FactsBySourceURL: %v", err)
	}
	if len(facts) != 1 || facts[0].ID != fixtureFactID {
		t.Fatalf("got %+v, want the seeded fact", facts)
	}
	if facts[0].Volatility != "stable" {
		t.Errorf("volatility = %q, want stable", facts[0].Volatility)
	}
}

// With no vector table the chain must still assemble, flagged rather than failed.
func TestURLProvenanceDegradesWithoutVectors(t *testing.T) {
	env := seededEnv(t)

	chain, err := env.Store.URLProvenance(context.Background(), fixtureURL)
	if err != nil {
		t.Fatalf("URLProvenance: %v", err)
	}
	if !chain.Known {
		t.Error("the seeded URL should be known")
	}
	if len(chain.FoundBy) != 1 {
		t.Errorf("found_by = %d, want 1", len(chain.FoundBy))
	}
	if chain.Scrape == nil {
		t.Fatal("the seeded URL has a scrape")
	}
	if len(chain.Facts) != 1 {
		t.Fatalf("facts = %d, want 1", len(chain.Facts))
	}
	if chain.VectorsAvailable {
		t.Error("no vector table was created, so vectors must report unavailable")
	}
	if chain.Note == "" {
		t.Error("an unavailable vector store should be explained in the note")
	}
	if chain.Facts[0].HasVector {
		t.Error("has_vector must not be claimed when vector state is unknown")
	}
}

// withVectors creates an active vector table and embeds the seeded fact.
func withVectors(t *testing.T, env *testEnv) {
	t.Helper()
	ctx := context.Background()
	const table, dim = "vectors_test", 4
	if err := env.Store.ensureVectorTable(ctx, table, dim); err != nil {
		t.Fatalf("ensureVectorTable: %v", err)
	}
	if err := env.Store.MetaSet(ctx, metaVectorTable, table); err != nil {
		t.Fatalf("MetaSet: %v", err)
	}
	if err := env.Store.UpsertVector(ctx, table, "memory", fixtureFactID, []float32{0.1, 0.2, 0.3, 0.4}, "test-model", dim); err != nil {
		t.Fatalf("UpsertVector: %v", err)
	}
}

func TestURLProvenanceReportsVectorPresence(t *testing.T) {
	env := seededEnv(t)
	withVectors(t, env)

	chain, err := env.Store.URLProvenance(context.Background(), fixtureURL)
	if err != nil {
		t.Fatalf("URLProvenance: %v", err)
	}
	if !chain.VectorsAvailable {
		t.Fatal("vectors should be available once a table is active")
	}
	if !chain.Facts[0].HasVector {
		t.Error("the embedded fact should report has_vector")
	}
	if chain.Note != "" {
		t.Errorf("no note expected when vectors are available, got %q", chain.Note)
	}
}

// A re-embed in flight must read as "unknown", not as "no vector".
func TestURLProvenanceDegradesDuringMigration(t *testing.T) {
	env := seededEnv(t)
	withVectors(t, env)
	if err := env.Store.MetaSet(context.Background(), metaMigration, "in-progress"); err != nil {
		t.Fatalf("MetaSet: %v", err)
	}

	chain, err := env.Store.URLProvenance(context.Background(), fixtureURL)
	if err != nil {
		t.Fatalf("URLProvenance: %v", err)
	}
	if chain.VectorsAvailable {
		t.Error("a migration in flight must report vectors unavailable")
	}
	if len(chain.Facts) != 1 {
		t.Error("facts must still be returned during a migration")
	}
}

func TestURLProvenanceUnknownURLIsEmptyNotAnError(t *testing.T) {
	env := seededEnv(t)

	chain, err := env.Store.URLProvenance(context.Background(), "https://nobody.example/nope")
	if err != nil {
		t.Fatalf("unknown URL should not error: %v", err)
	}
	if chain.Known {
		t.Error("an unseen URL should not be reported as known")
	}
	if len(chain.FoundBy) != 0 || len(chain.Facts) != 0 || chain.Scrape != nil {
		t.Errorf("expected an empty chain, got %+v", chain)
	}
}

func TestRunCausalityAssemblesTheChain(t *testing.T) {
	env := seededEnv(t)
	withVectors(t, env)

	graph, err := env.Store.RunCausality(context.Background(), fixtureRunID, 200)
	if err != nil {
		t.Fatalf("RunCausality: %v", err)
	}

	counts := map[string]int{}
	for _, n := range graph.Nodes {
		counts[n.Kind]++
	}
	if counts["search"] != 2 {
		t.Errorf("search nodes = %d, want 2", counts["search"])
	}
	if counts["url"] != 2 {
		t.Errorf("url nodes = %d, want 2", counts["url"])
	}
	if counts["scrape"] != 2 {
		t.Errorf("scrape nodes = %d, want 2", counts["scrape"])
	}
	if counts["fact"] != 1 {
		t.Errorf("fact nodes = %d, want 1", counts["fact"])
	}
	if graph.Truncated {
		t.Error("a two-URL run should not be truncated at a cap of 200")
	}
	if !graph.VectorsAvailable {
		t.Error("vectors should be available")
	}

	var factNode *CausalityNode
	for i := range graph.Nodes {
		if graph.Nodes[i].Kind == "fact" {
			factNode = &graph.Nodes[i]
		}
	}
	if factNode == nil || !factNode.HasVector {
		t.Error("the fact node should carry its vector presence")
	}

	// Every edge must point at nodes that exist, or the view cannot render it.
	ids := map[string]bool{}
	for _, n := range graph.Nodes {
		ids[n.ID] = true
	}
	for _, e := range graph.Edges {
		if !ids[e.From] || !ids[e.To] {
			t.Errorf("dangling edge %s → %s", e.From, e.To)
		}
	}
}

// A URL found by two searches is one node with two incoming edges.
func TestRunCausalityDeduplicatesSharedURLs(t *testing.T) {
	env := seededEnv(t)
	ctx := context.Background()

	// The bing search also found the URL google found, at a worse rank.
	if _, err := env.Store.db.ExecContext(ctx,
		`INSERT INTO search_urls (id, search_id, url_id, rank, created_at)
		 VALUES ('shared-link', '00000000-0000-7000-8000-00000000000c',
		         '00000000-0000-7000-8000-000000000003', 5, '2026-08-05T10:00:00Z')`); err != nil {
		t.Fatalf("linking a shared URL: %v", err)
	}

	graph, err := env.Store.RunCausality(ctx, fixtureRunID, 200)
	if err != nil {
		t.Fatalf("RunCausality: %v", err)
	}

	urlNodes := 0
	for _, n := range graph.Nodes {
		if n.Kind == "url" && n.URL == fixtureURL {
			urlNodes++
		}
	}
	if urlNodes != 1 {
		t.Errorf("shared URL produced %d nodes, want 1", urlNodes)
	}

	incoming := 0
	for _, e := range graph.Edges {
		if e.To == "url:00000000-0000-7000-8000-000000000003" {
			incoming++
		}
	}
	if incoming != 2 {
		t.Errorf("shared URL has %d incoming edges, want 2", incoming)
	}
}

func TestRunCausalityRespectsTheCap(t *testing.T) {
	env := seededEnv(t)

	graph, err := env.Store.RunCausality(context.Background(), fixtureRunID, 1)
	if err != nil {
		t.Fatalf("RunCausality: %v", err)
	}
	if !graph.Truncated {
		t.Error("a cap of 1 against a two-URL run should truncate")
	}
	if graph.Limit != 1 {
		t.Errorf("limit = %d, want 1", graph.Limit)
	}
	urls := 0
	for _, n := range graph.Nodes {
		if n.Kind == "url" {
			urls++
		}
	}
	if urls != 1 {
		t.Errorf("url nodes = %d, want 1", urls)
	}
}

func TestRunCausalityUnknownRunIsEmptyNotAnError(t *testing.T) {
	env := seededEnv(t)

	graph, err := env.Store.RunCausality(context.Background(), "no-such-run", 200)
	if err != nil {
		t.Fatalf("unknown run should not error: %v", err)
	}
	if len(graph.Nodes) != 0 || len(graph.Edges) != 0 {
		t.Errorf("expected an empty graph, got %d nodes / %d edges", len(graph.Nodes), len(graph.Edges))
	}
}

// ---- endpoint level ----

func TestProvenanceEndpoint(t *testing.T) {
	_, env := newTestServer(t)
	if err := env.Seed(context.Background()); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/provenance?url="+fixtureURL, nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var chain URLProvenance
	if err := json.NewDecoder(rec.Body).Decode(&chain); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(chain.FoundBy) != 1 || chain.Scrape == nil || len(chain.Facts) != 1 {
		t.Errorf("incomplete chain: %+v", chain)
	}
}

func TestProvenanceEndpointRequiresURL(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/provenance", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestRunCausalityEndpoint(t *testing.T) {
	_, env := newTestServer(t)
	if err := env.Seed(context.Background()); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/runs/"+fixtureRunID+"/causality", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var graph CausalityGraph
	if err := json.NewDecoder(rec.Body).Decode(&graph); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(graph.Nodes) == 0 || len(graph.Edges) == 0 {
		t.Error("expected a populated graph")
	}
	if graph.Limit != 200 {
		t.Errorf("limit = %d, want the configured 200", graph.Limit)
	}
}
