package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// newTestServer wires an apiServer over a throwaway main + log database and
// registers the teardown that removes both. Every test that needs a live
// handler goes through here, so no suite can touch ./data.
func newTestServer(t *testing.T) (*apiServer, *testEnv) {
	t.Helper()
	env, err := newTestEnv("")
	if err != nil {
		t.Fatalf("temp environment: %v", err)
	}
	dir := env.Dir
	t.Cleanup(func() {
		if err := env.Close(); err != nil {
			t.Errorf("tearing down temp environment: %v", err)
		}
		if _, err := os.Stat(dir); !os.IsNotExist(err) {
			t.Errorf("temp data dir %s survived teardown (err=%v)", dir, err)
		}
	})
	return env.Server(), env
}

func TestUIConfigServesNonSecretSettings(t *testing.T) {
	_, env := newTestServer(t)
	env.Cfg.Observability = ObservabilityConfig{
		PollInterval:        Duration{2500 * time.Millisecond},
		PollEnabled:         true,
		ProjectionSampleCap: 1234,
	}
	// Rebuild so the handler sees the edited config.
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ui-config", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got UIConfig
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if got.PollIntervalMS != 2500 {
		t.Errorf("poll_interval_ms = %d, want 2500", got.PollIntervalMS)
	}
	if !got.PollEnabled {
		t.Error("poll_enabled = false, want true")
	}
	if got.ProjectionSampleCap != 1234 {
		t.Errorf("projection_sample_cap = %d, want 1234", got.ProjectionSampleCap)
	}
}

func TestUIConfigLeaksNoSecrets(t *testing.T) {
	_, env := newTestServer(t)
	const secret = "super-secret-key-value"
	env.Cfg.Server.APIKey = secret
	srv := env.Server()

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ui-config", nil)
	req.Header.Set("Authorization", "Bearer "+secret)
	srv.http.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if strings.Contains(string(body), secret) {
		t.Fatalf("api_key leaked into /api/ui-config: %s", body)
	}
	// The response must carry exactly the three documented keys and nothing else.
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	want := map[string]bool{"poll_interval_ms": true, "poll_enabled": true, "projection_sample_cap": true}
	for k := range raw {
		if !want[k] {
			t.Errorf("unexpected field %q in /api/ui-config", k)
		}
	}
	if len(raw) != len(want) {
		t.Errorf("got %d fields, want %d", len(raw), len(want))
	}
}

// The assumed deployment leaves api_key unset and lets an edge enforce access:
// the SPA shell and every /api route must answer without a token.
func TestNoAPIKeyServesShellAndAPIOpenly(t *testing.T) {
	srv, env := newTestServer(t)
	if env.Cfg.Server.APIKey != "" {
		t.Fatal("test environment should default to no api_key")
	}

	for _, path := range []string{"/", "/api/ui-config", "/api/stats", "/healthz"} {
		rec := httptest.NewRecorder()
		srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s without a token = %d, want 200", path, rec.Code)
		}
	}
}

// Setting api_key must still gate /api/*, unchanged by this plan.
func TestAPIKeyStillGatesAPI(t *testing.T) {
	_, env := newTestServer(t)
	env.Cfg.Server.APIKey = "k"
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/ui-config", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("unauthenticated GET /api/ui-config = %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/ui-config", nil)
	req.Header.Set("Authorization", "Bearer k")
	srv.http.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("authenticated GET /api/ui-config = %d, want 200", rec.Code)
	}
}
