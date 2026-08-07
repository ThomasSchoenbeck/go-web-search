package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const projectionCap = 100

// seedVectors gives the seeded fact and cached search one embedding each, so the
// dump has one point per owner kind.
func TestVectorProjectionReturnsBothOwnerKinds(t *testing.T) {
	env := seededVectorEnv(t)

	dump, err := env.Store.VectorProjection(context.Background(), projectionCap, 0, 0)
	if err != nil {
		t.Fatalf("VectorProjection: %v", err)
	}
	if !dump.Available {
		t.Fatalf("expected the store to be available, note = %q", dump.Note)
	}
	if len(dump.Points) != 2 {
		t.Fatalf("points = %d, want 2 (one per owner kind)", len(dump.Points))
	}

	byKind := map[string]ProjectionPoint{}
	for _, p := range dump.Points {
		byKind[p.OwnerKind] = p
	}
	mem, ok := byKind[ownerMemory]
	if !ok {
		t.Fatal("no memory point returned")
	}
	if mem.Label != fixtureFactText {
		t.Errorf("memory label = %q, want the fact text", mem.Label)
	}
	if mem.SourceURL != fixtureURL {
		t.Errorf("memory source_url = %q", mem.SourceURL)
	}
	search, ok := byKind[ownerSearch]
	if !ok {
		t.Fatal("no search point returned")
	}
	if search.Label != fixtureCacheText {
		t.Errorf("search label = %q, want the cached query", search.Label)
	}
	if dump.Dim != stubEmbedDim || dump.Model == "" {
		t.Errorf("dim = %d, model = %q", dump.Dim, dump.Model)
	}
	if dump.Total[ownerMemory] != 1 || dump.Total[ownerSearch] != 1 {
		t.Errorf("total = %v, want one of each", dump.Total)
	}
	if dump.Truncated {
		t.Error("nothing was left out, so truncated should be false")
	}
}

// The whole point of the dump: the raw vector survives the round trip through
// the F32_BLOB column intact.
func TestVectorProjectionReturnsTheStoredVector(t *testing.T) {
	env := seededVectorEnv(t)

	dump, err := env.Store.VectorProjection(context.Background(), projectionCap, 0, 0)
	if err != nil {
		t.Fatalf("VectorProjection: %v", err)
	}
	for _, p := range dump.Points {
		if len(p.Vector) != stubEmbedDim {
			t.Fatalf("%s vector has %d dimensions, want %d", p.OwnerKind, len(p.Vector), stubEmbedDim)
		}
	}

	want := stubVector(fixtureFactText)
	for _, p := range dump.Points {
		if p.OwnerKind != ownerMemory {
			continue
		}
		for i := range want {
			// F32_BLOB storage is float32, so compare at float32 precision.
			if diff := p.Vector[i] - want[i]; diff > 1e-6 || diff < -1e-6 {
				t.Errorf("component %d = %v, want %v", i, p.Vector[i], want[i])
			}
		}
	}
}

func TestVectorProjectionRespectsTheCapAndPages(t *testing.T) {
	env := seededVectorEnv(t)
	ctx := context.Background()

	// A limit above the configured cap is clamped down to it.
	capped, err := env.Store.VectorProjection(ctx, 1, 50, 0)
	if err != nil {
		t.Fatalf("VectorProjection: %v", err)
	}
	if capped.Limit != 1 || len(capped.Points) != 1 {
		t.Errorf("limit = %d with %d points, want the cap of 1", capped.Limit, len(capped.Points))
	}
	if !capped.Truncated {
		t.Error("a capped dump that left a point behind must say so")
	}

	second, err := env.Store.VectorProjection(ctx, projectionCap, 1, 1)
	if err != nil {
		t.Fatalf("VectorProjection: %v", err)
	}
	if len(second.Points) != 1 {
		t.Fatalf("second page = %d points, want 1", len(second.Points))
	}
	if second.Points[0].ID == capped.Points[0].ID {
		t.Error("the offset did not advance the page")
	}
	if second.Truncated {
		t.Error("the second page reaches the end, so truncated should be false")
	}

	past, err := env.Store.VectorProjection(ctx, projectionCap, 0, 500)
	if err != nil {
		t.Fatalf("an offset past the end must not error: %v", err)
	}
	if len(past.Points) != 0 {
		t.Errorf("offset past the end returned %d points", len(past.Points))
	}
}

// A vector whose owning row was deleted is skipped, not plotted as an anonymous
// dot — the same choice the explorer makes.
func TestVectorProjectionSkipsOrphanedVectors(t *testing.T) {
	env := seededVectorEnv(t)
	ctx := context.Background()

	if _, err := env.Store.db.ExecContext(ctx, `DELETE FROM memory_facts WHERE id = ?`, fixtureFactID); err != nil {
		t.Fatalf("deleting the fact: %v", err)
	}

	dump, err := env.Store.VectorProjection(ctx, projectionCap, 0, 0)
	if err != nil {
		t.Fatalf("VectorProjection: %v", err)
	}
	for _, p := range dump.Points {
		if p.OwnerKind == ownerMemory {
			t.Errorf("orphaned memory vector was returned: %+v", p)
		}
	}
	if dump.Total[ownerMemory] != 1 {
		t.Errorf("total still counts the stored vector: %v", dump.Total)
	}
}

func TestVectorProjectionDegradesWithNoVectorTable(t *testing.T) {
	env := seededEnv(t) // no vectors seeded

	dump, err := env.Store.VectorProjection(context.Background(), projectionCap, 0, 0)
	if err != nil {
		t.Fatalf("an unavailable store must not error: %v", err)
	}
	if dump.Available {
		t.Error("available should be false with no vector table")
	}
	if dump.Note == "" {
		t.Error("the degraded state must explain itself")
	}
	if len(dump.Points) != 0 {
		t.Error("no points should be returned")
	}
}

func TestVectorProjectionDegradesDuringMigration(t *testing.T) {
	env := seededVectorEnv(t)
	ctx := context.Background()
	if err := env.Store.MetaSet(ctx, metaMigration, "in-progress"); err != nil {
		t.Fatalf("MetaSet: %v", err)
	}

	dump, err := env.Store.VectorProjection(ctx, projectionCap, 0, 0)
	if err != nil {
		t.Fatalf("a migration must not error: %v", err)
	}
	if dump.Available {
		t.Error("available should be false during a migration")
	}
	if dump.Note != migratingNote {
		t.Errorf("note = %q, want the migration note", dump.Note)
	}
}

func TestParseVectorLiteralRoundTrips(t *testing.T) {
	want := []float32{1, 0.5, -2}
	got, err := parseVectorLiteral(vectorLiteral(want))
	if err != nil {
		t.Fatalf("parseVectorLiteral: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("length = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("component %d = %v, want %v", i, got[i], want[i])
		}
	}

	if empty, err := parseVectorLiteral("[]"); err != nil || len(empty) != 0 {
		t.Errorf("empty literal = %v, err %v", empty, err)
	}
	if _, err := parseVectorLiteral("[1,not-a-number]"); err == nil {
		t.Error("a malformed component should be an error, not a silent zero")
	}
}

// ---- endpoint level ----

func TestProjectionEndpoint(t *testing.T) {
	env := seededVectorEnv(t)
	env.Cfg.Observability.ProjectionSampleCap = 1
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/projection?limit=50", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var dump ProjectionDump
	if err := json.NewDecoder(rec.Body).Decode(&dump); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	// The configured cap wins over the requested limit.
	if dump.Limit != 1 || len(dump.Points) != 1 {
		t.Errorf("limit = %d with %d points, want the configured cap of 1", dump.Limit, len(dump.Points))
	}
	if len(dump.Points[0].Vector) != stubEmbedDim {
		t.Errorf("the point carries no usable vector: %+v", dump.Points[0])
	}
}

// An unavailable vector store is a 200 with a note, never a 500.
func TestProjectionEndpointDegradesWithoutError(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/projection", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var dump ProjectionDump
	if err := json.NewDecoder(rec.Body).Decode(&dump); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if dump.Available || dump.Note == "" {
		t.Errorf("expected an explained degraded result, got %+v", dump)
	}
}
