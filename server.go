package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type apiServer struct {
	cfg  Config
	h    *harvester
	http *http.Server
}

// ---- request and response shapes, shared by REST and MCP ----

type SearchRequest struct {
	Terms  []string `json:"terms" jsonschema:"search terms to run"`
	Scrape bool     `json:"scrape,omitempty" jsonschema:"also scrape the URLs that were found"`
}

type SearchResponse struct {
	RunID    string          `json:"run_id"`
	Terms    int             `json:"terms"`
	URLs     []URLRow        `json:"urls"`
	Scrapes  []ScrapeOutcome `json:"scrapes,omitempty"`
	Complete bool            `json:"complete"`
	Note     string          `json:"note,omitempty"`
}

type ScrapeRequest struct {
	URLs  []string `json:"urls" jsonschema:"absolute URLs to fetch"`
	RunID string   `json:"run_id,omitempty" jsonschema:"instead of urls, scrape everything a previous run found"`
}

type ScrapeResponse struct {
	RunID    string          `json:"run_id,omitempty"`
	Results  []ScrapeOutcome `json:"results"`
	Snippets []Snippet       `json:"snippets,omitempty"`
}

// Snippet carries just enough text for a model to judge relevance. The full
// document stays behind GET /api/scrapes/{id}.
type Snippet struct {
	ScrapeID  string `json:"scrape_id"`
	URL       string `json:"url"`
	Title     string `json:"title,omitempty"`
	Text      string `json:"text"`
	Truncated bool   `json:"truncated"`
}

func newAPIServer(cfg Config, h *harvester) *apiServer {
	s := &apiServer{cfg: cfg, h: h}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/search", s.handleSearch)
	mux.HandleFunc("POST /api/scrape", s.handleScrape)
	mux.HandleFunc("GET /api/runs", s.handleListRuns)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("GET /api/runs/{id}/urls", s.handleRunURLs)
	mux.HandleFunc("GET /api/runs/{id}/searches", s.handleRunSearches)
	mux.HandleFunc("GET /api/runs/{id}/scrapes", s.handleRunScrapes)
	mux.HandleFunc("GET /api/searches/{id}/raw", s.handleSearchRaw)
	mux.HandleFunc("GET /api/scrapes/{id}", s.handleGetScrape)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// MCP shares the same listener; the SDK's handler is a plain http.Handler.
	mcpServer := s.buildMCPServer()
	mux.Handle("/mcp", mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return mcpServer }, nil))

	s.http = &http.Server{
		Addr:         cfg.Server.Addr,
		Handler:      s.withAuth(mux),
		ReadTimeout:  cfg.Server.ReadTimeout.Duration,
		WriteTimeout: cfg.Server.WriteTimeout.Duration,
	}
	return s
}

func (s *apiServer) ListenAndServe() error { return s.http.ListenAndServe() }

func (s *apiServer) Shutdown(ctx context.Context) error { return s.http.Shutdown(ctx) }

// withAuth applies the optional bearer token to everything except health.
func (s *apiServer) withAuth(next http.Handler) http.Handler {
	key := strings.TrimSpace(s.cfg.Server.APIKey)
	if key == "" {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+key {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---- core operations, called by both transports ----

func (s *apiServer) runSearch(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	if len(req.Terms) == 0 {
		return nil, errors.New("at least one search term is required")
	}

	runID, err := s.h.store.StartRun(ctx, "api-search", "")
	if err != nil {
		return nil, err
	}
	defer s.h.store.FinishRun(ctx, runID)

	never := context.Background() // no signal handling inside a request
	complete, err := s.h.SearchTerms(ctx, runID, req.Terms, never)
	if err != nil {
		return nil, err
	}

	urls, err := s.h.store.RunURLs(ctx, runID)
	if err != nil {
		return nil, err
	}

	resp := &SearchResponse{RunID: runID, Terms: len(req.Terms), URLs: urls, Complete: complete}

	if req.Scrape && s.cfg.Scrape.Enabled {
		outcomes, err := s.h.ScrapeRun(ctx, runID)
		if err != nil {
			return nil, err
		}
		resp.Scrapes = outcomes
		if len(urls) > s.cfg.Scrape.MaxURLs {
			resp.Note = fmt.Sprintf("scraped the first %d of %d URLs (scrape.max_urls); "+
				"fetch the rest with POST /api/scrape", s.cfg.Scrape.MaxURLs, len(urls))
		}
	}
	return resp, nil
}

func (s *apiServer) runScrape(ctx context.Context, req ScrapeRequest) (*ScrapeResponse, error) {
	var (
		outcomes []ScrapeOutcome
		err      error
	)
	switch {
	case req.RunID != "":
		outcomes, err = s.h.ScrapeRun(ctx, req.RunID)
		if err != nil {
			return nil, err
		}
	case len(req.URLs) > 0:
		outcomes = s.h.ScrapeURLs(ctx, "", req.URLs)
	default:
		return nil, errors.New("provide either urls or run_id")
	}
	return &ScrapeResponse{RunID: req.RunID, Results: outcomes}, nil
}

// snippetsFor loads capped text for each successful scrape, which is what makes
// the MCP tools directly useful to a model without a second round trip.
func (s *apiServer) snippetsFor(ctx context.Context, outcomes []ScrapeOutcome) []Snippet {
	limit := s.cfg.Scrape.SnippetChars
	if limit <= 0 {
		limit = 2000
	}
	var out []Snippet
	for _, o := range outcomes {
		if o.ScrapeID == "" || o.TextChars == 0 {
			continue
		}
		detail, err := s.h.store.GetScrape(ctx, o.ScrapeID, false)
		if err != nil {
			continue
		}
		text := detail.Text
		truncated := false
		if len(text) > limit {
			text = text[:limit]
			truncated = true
		}
		out = append(out, Snippet{
			ScrapeID: o.ScrapeID, URL: o.URL, Title: detail.Title,
			Text: text, Truncated: truncated,
		})
	}
	return out
}

// ---- REST handlers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func (s *apiServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.runSearch(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *apiServer) handleScrape(w http.ResponseWriter, r *http.Request) {
	var req ScrapeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.runScrape(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *apiServer) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := s.h.store.ListRuns(r.Context(), limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *apiServer) handleGetRun(w http.ResponseWriter, r *http.Request) {
	run, err := s.h.store.GetRun(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

func (s *apiServer) handleRunURLs(w http.ResponseWriter, r *http.Request) {
	urls, err := s.h.store.RunURLs(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, urls)
}

func (s *apiServer) handleRunSearches(w http.ResponseWriter, r *http.Request) {
	searches, err := s.h.store.ListSearches(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, searches)
}

func (s *apiServer) handleRunScrapes(w http.ResponseWriter, r *http.Request) {
	ids, err := s.h.store.RunScrapeIDs(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scrape_ids": ids})
}

func (s *apiServer) handleSearchRaw(w http.ResponseWriter, r *http.Request) {
	body, err := s.h.store.SearchRawHTML(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(body))
}

func (s *apiServer) handleGetScrape(w http.ResponseWriter, r *http.Request) {
	includeRaw := r.URL.Query().Get("raw") == "1"
	detail, err := s.h.store.GetScrape(r.Context(), r.PathValue("id"), includeRaw)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, detail)
}

// ---- MCP tools ----

func (s *apiServer) buildMCPServer() *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "serp-harvester", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "web_search",
		Description: "Search Google, Bing and DuckDuckGo for one or more terms and return the " +
			"destination URLs found. Set scrape=true to also fetch the page contents. " +
			"Returns a run_id; full details stay retrievable by id.",
	}, s.mcpSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name: "web_scrape",
		Description: "Fetch and clean specific URLs, or every URL from a previous run_id. " +
			"Returns text snippets plus scrape ids for the full documents.",
	}, s.mcpScrape)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_scrape",
		Description: "Return the full cleaned text, title and images of a single scrape by id.",
	}, s.mcpGetScrape)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_run",
		Description: "Summarise a run by id: how many searches, URLs and scrapes it produced.",
	}, s.mcpGetRun)

	return server
}

func (s *apiServer) mcpSearch(ctx context.Context, _ *mcp.CallToolRequest, in SearchRequest) (
	*mcp.CallToolResult, SearchResponse, error) {

	resp, err := s.runSearch(ctx, in)
	if err != nil {
		return nil, SearchResponse{}, err
	}
	return nil, *resp, nil
}

func (s *apiServer) mcpScrape(ctx context.Context, _ *mcp.CallToolRequest, in ScrapeRequest) (
	*mcp.CallToolResult, ScrapeResponse, error) {

	resp, err := s.runScrape(ctx, in)
	if err != nil {
		return nil, ScrapeResponse{}, err
	}
	resp.Snippets = s.snippetsFor(ctx, resp.Results)
	return nil, *resp, nil
}

type getScrapeInput struct {
	ScrapeID   string `json:"scrape_id" jsonschema:"id returned by web_scrape"`
	IncludeRaw bool   `json:"include_raw,omitempty" jsonschema:"include the unprocessed HTML as well"`
}

func (s *apiServer) mcpGetScrape(ctx context.Context, _ *mcp.CallToolRequest, in getScrapeInput) (
	*mcp.CallToolResult, ScrapeDetail, error) {

	detail, err := s.h.store.GetScrape(ctx, in.ScrapeID, in.IncludeRaw)
	if err != nil {
		return nil, ScrapeDetail{}, err
	}
	// clean_html is large and rarely what a model wants; text is the useful field.
	detail.CleanHTML = ""
	return nil, *detail, nil
}

type getRunInput struct {
	RunID string `json:"run_id" jsonschema:"id returned by web_search"`
}

func (s *apiServer) mcpGetRun(ctx context.Context, _ *mcp.CallToolRequest, in getRunInput) (
	*mcp.CallToolResult, RunSummary, error) {

	run, err := s.h.store.GetRun(ctx, in.RunID)
	if err != nil {
		return nil, RunSummary{}, err
	}
	return nil, *run, nil
}

// serveMode runs the HTTP server until a signal arrives.
func serveMode(cfg Config, art *artifacts, h *harvester, stop context.Context) error {
	srv := newAPIServer(cfg, h)

	errCh := make(chan error, 1)
	go func() {
		art.Log.Printf("listening on http://%s", cfg.Server.Addr)
		art.Log.Printf("  REST : POST /api/search, POST /api/scrape, GET /api/runs/{id}...")
		art.Log.Printf("  MCP  : %s/mcp (streamable HTTP)", cfg.Server.Addr)
		if cfg.Server.APIKey == "" {
			art.Log.Printf("  NOTE : no api_key set, every caller that can reach the port is trusted")
		}
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-stop.Done():
		art.Log.Printf("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
