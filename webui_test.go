package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The SPA fallback must never answer for a backend path: an unregistered
// /api/... has to stay a 404 rather than becoming a 200 page of HTML, which is
// the failure mode that makes a typo'd endpoint look like a working one.
func TestSPAFallbackDoesNotShadowBackendRoutes(t *testing.T) {
	srv, _ := newTestServer(t)

	for _, path := range []string{"/api/bogus", "/api/runs/x/nope", "/mcp/anything"} {
		rec := httptest.NewRecorder()
		srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d, want 404", path, rec.Code)
		}
	}
}

func TestBackendPath(t *testing.T) {
	backend := []string{"/api/", "/api/stats", "/mcp", "/mcp/x", "/healthz"}
	for _, p := range backend {
		if !backendPath(p) {
			t.Errorf("backendPath(%q) = false, want true", p)
		}
	}
	spa := []string{"/", "/runs", "/runs/123", "/logs", "/assets/index.js", "/apiary", "/healthzz"}
	for _, p := range spa {
		if backendPath(p) {
			t.Errorf("backendPath(%q) = true, want false", p)
		}
	}
}

// Client-side routes must resolve to the app rather than 404, and only GET/HEAD
// are meaningful for a static shell.
func TestSPAServesClientRoutes(t *testing.T) {
	srv, _ := newTestServer(t)

	for _, path := range []string{"/", "/runs", "/runs/123", "/logs"} {
		rec := httptest.NewRecorder()
		srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		// 200 with a real build embedded, 503 with only the placeholder. Either
		// way the SPA handler owns the path — what must not happen is a 404.
		if rec.Code != http.StatusOK && rec.Code != http.StatusServiceUnavailable {
			t.Errorf("GET %s = %d, want 200 or 503", path, rec.Code)
		}
	}

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/runs", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT /runs = %d, want 405", rec.Code)
	}
}
