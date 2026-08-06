package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const (
	fixtureFactText  = "The fixture page says Fixture One."
	fixtureCacheID   = "00000000-0000-7000-8000-00000000000a"
	fixtureCacheText = "fixture term"
)

// failingEmbedder proves the explorer never pays for an embedding when the
// vector store cannot answer anyway.
type failingEmbedder struct{ called bool }

func (f *failingEmbedder) Embed(context.Context, []string, bool, string) ([][]float32, error) {
	f.called = true
	return nil, errors.New("the model endpoint should not have been called")
}

func seededVectorEnv(t *testing.T) *testEnv {
	t.Helper()
	env := seededEnv(t)
	if err := seedVectors(context.Background(), env.Store); err != nil {
		t.Fatalf("seeding vectors: %v", err)
	}
	return env
}

func TestExploreReturnsNeighborsFromBothOwnerKinds(t *testing.T) {
	env := seededVectorEnv(t)

	result, err := env.Store.Explore(context.Background(), stubEmbedder{}, fixtureFactText, 10)
	if err != nil {
		t.Fatalf("Explore: %v", err)
	}
	if !result.Available {
		t.Fatalf("expected the store to be available, note = %q", result.Note)
	}
	if len(result.Neighbors) != 2 {
		t.Fatalf("neighbors = %d, want 2 (one per owner kind)", len(result.Neighbors))
	}

	kinds := map[string]Neighbor{}
	for _, n := range result.Neighbors {
		kinds[n.OwnerKind] = n
	}
	mem, ok := kinds[ownerMemory]
	if !ok {
		t.Fatal("no memory neighbour returned")
	}
	if mem.Text != fixtureFactText {
		t.Errorf("memory neighbour text = %q, want the fact text", mem.Text)
	}
	if mem.SourceURL != fixtureURL {
		t.Errorf("memory neighbour source_url = %q", mem.SourceURL)
	}

	search, ok := kinds[ownerSearch]
	if !ok {
		t.Fatal("no search neighbour returned")
	}
	if search.Text != fixtureCacheText {
		t.Errorf("search neighbour text = %q, want the cached query", search.Text)
	}
	if search.ResultCount != 1 {
		t.Errorf("search neighbour result_count = %d, want 1", search.ResultCount)
	}
}

// Querying the exact text of a stored item must put that item first.
func TestExploreSortsNearestFirst(t *testing.T) {
	env := seededVectorEnv(t)

	result, err := env.Store.Explore(context.Background(), stubEmbedder{}, fixtureFactText, 10)
	if err != nil {
		t.Fatalf("Explore: %v", err)
	}
	if result.Neighbors[0].OwnerKind != ownerMemory {
		t.Errorf("nearest neighbour = %s, want the exactly-matching memory fact", result.Neighbors[0].OwnerKind)
	}
	for i := 1; i < len(result.Neighbors); i++ {
		if result.Neighbors[i-1].Distance > result.Neighbors[i].Distance {
			t.Errorf("neighbours out of order at %d: %v > %v", i,
				result.Neighbors[i-1].Distance, result.Neighbors[i].Distance)
		}
	}
}

func TestExploreReportsSimilarityAsOneMinusDistance(t *testing.T) {
	env := seededVectorEnv(t)

	result, err := env.Store.Explore(context.Background(), stubEmbedder{}, fixtureCacheText, 10)
	if err != nil {
		t.Fatalf("Explore: %v", err)
	}
	for _, n := range result.Neighbors {
		if diff := (1 - n.Distance) - n.Similarity; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("similarity %v is not 1 − distance %v", n.Similarity, n.Distance)
		}
	}
}

func TestExploreHonorsK(t *testing.T) {
	env := seededVectorEnv(t)

	result, err := env.Store.Explore(context.Background(), stubEmbedder{}, fixtureFactText, 1)
	if err != nil {
		t.Fatalf("Explore: %v", err)
	}
	if len(result.Neighbors) != 1 {
		t.Errorf("neighbors = %d, want 1", len(result.Neighbors))
	}
	if result.K != 1 {
		t.Errorf("k = %d, want 1", result.K)
	}
}

func TestExploreBoundsK(t *testing.T) {
	env := seededVectorEnv(t)

	// VectorSearch is a linear scan, so an unbounded k is a real cost.
	result, err := env.Store.Explore(context.Background(), stubEmbedder{}, fixtureFactText, 10_000)
	if err != nil {
		t.Fatalf("Explore: %v", err)
	}
	if result.K != exploreMaxK {
		t.Errorf("k = %d, want it clamped to %d", result.K, exploreMaxK)
	}

	result, err = env.Store.Explore(context.Background(), stubEmbedder{}, fixtureFactText, 0)
	if err != nil {
		t.Fatalf("Explore: %v", err)
	}
	if result.K != exploreDefaultK {
		t.Errorf("k = %d, want the default %d", result.K, exploreDefaultK)
	}
}

func TestExploreDegradesWithNoVectorTable(t *testing.T) {
	env := seededEnv(t) // no vectors seeded
	emb := &failingEmbedder{}

	result, err := env.Store.Explore(context.Background(), emb, "anything", 10)
	if err != nil {
		t.Fatalf("an unavailable store must not be an error: %v", err)
	}
	if result.Available {
		t.Error("available should be false with no vector table")
	}
	if result.Note == "" {
		t.Error("the degraded state must explain itself")
	}
	if len(result.Neighbors) != 0 {
		t.Error("no neighbours should be returned")
	}
	if emb.called {
		t.Error("the embedder must not be called when the store cannot answer")
	}
}

func TestExploreDegradesDuringMigration(t *testing.T) {
	env := seededVectorEnv(t)
	if err := env.Store.MetaSet(context.Background(), metaMigration, "in-progress"); err != nil {
		t.Fatalf("MetaSet: %v", err)
	}
	emb := &failingEmbedder{}

	result, err := env.Store.Explore(context.Background(), emb, "anything", 10)
	if err != nil {
		t.Fatalf("a migration must not be an error: %v", err)
	}
	if result.Available {
		t.Error("available should be false during a migration")
	}
	if result.Note != migratingNote {
		t.Errorf("note = %q, want the migration note", result.Note)
	}
	if emb.called {
		t.Error("the embedder must not be called during a migration")
	}
}

// A vector whose owning row was deleted must be skipped, not rendered blank.
func TestExploreSkipsOrphanedVectors(t *testing.T) {
	env := seededVectorEnv(t)
	ctx := context.Background()

	if _, err := env.Store.db.ExecContext(ctx, `DELETE FROM memory_facts WHERE id = ?`, fixtureFactID); err != nil {
		t.Fatalf("deleting the fact: %v", err)
	}

	result, err := env.Store.Explore(ctx, stubEmbedder{}, fixtureFactText, 10)
	if err != nil {
		t.Fatalf("Explore: %v", err)
	}
	for _, n := range result.Neighbors {
		if n.OwnerKind == ownerMemory {
			t.Errorf("orphaned memory vector was returned: %+v", n)
		}
	}
	if result.MemoryHits != 1 {
		t.Errorf("memory_hits = %d, want the raw hit still counted", result.MemoryHits)
	}
}

// ---- endpoint level ----

func TestExploreEndpoint(t *testing.T) {
	env := seededVectorEnv(t)
	srv := env.Server()
	srv.embed = stubEmbedder{}

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet,
		"/api/explore?q="+url.QueryEscape(fixtureCacheText)+"&k=5", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result ExploreResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if !result.Available || len(result.Neighbors) == 0 {
		t.Errorf("expected neighbours, got %+v", result)
	}
	if result.K != 5 {
		t.Errorf("k = %d, want 5", result.K)
	}
}

func TestExploreEndpointRequiresQuery(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/explore", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

// An unavailable vector store is a 200 with a note, never a 500.
func TestExploreEndpointDegradesWithoutError(t *testing.T) {
	_, env := newTestServer(t)
	srv := env.Server()
	srv.embed = &failingEmbedder{}

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/explore?q=anything", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var result ExploreResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if result.Available || result.Note == "" {
		t.Errorf("expected an explained degraded result, got %+v", result)
	}
}
