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
	cfg      Config
	h        *harvester
	llm      *LLMClient
	resolver *resolver
	http     *http.Server
}

// ---- request and response shapes, shared by REST and MCP ----

type SearchRequest struct {
	Terms     []string `json:"terms" jsonschema:"search terms to run"`
	Scrape    bool     `json:"scrape,omitempty" jsonschema:"also scrape the URLs that were found"`
	UseCache  *bool    `json:"use_cache,omitempty" jsonschema:"reuse the search cache (default true); set false to force a live search"`
	UseMemory *bool    `json:"use_memory,omitempty" jsonschema:"consult memory first and skip the web when it can answer (default true)"`
	MaxAge    int      `json:"max_age_seconds,omitempty" jsonschema:"reject cached or remembered results older than this many seconds"`
	Remember  string   `json:"remember,omitempty" jsonschema:"how durably to keep what is learned: short, long, permanent, or off (default short)"`
}

type SearchResponse struct {
	RunID    string            `json:"run_id,omitempty"`
	Terms    int               `json:"terms"`
	Source   string            `json:"source,omitempty"`
	Answers  map[string]string `json:"answers,omitempty"`
	URLs     []CachedURL       `json:"urls"`
	Scrapes  []ScrapeOutcome   `json:"scrapes,omitempty"`
	Complete bool              `json:"complete"`
	Note     string            `json:"note,omitempty"`
}

type ScrapeRequest struct {
	URLs     []string `json:"urls" jsonschema:"absolute URLs to fetch"`
	RunID    string   `json:"run_id,omitempty" jsonschema:"when passed with urls, links scrapes to the search that found them; when passed without urls, scrapes everything that run found (use only when full coverage is needed)"`
	UseCache *bool    `json:"use_cache,omitempty" jsonschema:"reuse the scrape cache (default true); set false to force a fresh fetch"`
	MaxAge   int      `json:"max_age_seconds,omitempty" jsonschema:"reject cached content older than this many seconds"`
	Remember string   `json:"remember,omitempty" jsonschema:"distil scraped pages into memory at this durability: short, long, permanent, or off (default short)"`
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
	llm := newLLMClient(cfg.LLM, 60*time.Second)
	s := &apiServer{cfg: cfg, h: h, llm: llm, resolver: newResolver(cfg, h.store, llm, h)}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/search", s.handleSearch)
	mux.HandleFunc("POST /api/scrape", s.handleScrape)
	mux.HandleFunc("POST /api/memory/query", s.handleMemoryQuery)
	mux.HandleFunc("POST /api/memory/store", s.handleMemoryStore)
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
		Handler:      corsMiddleware(s.withAuth(mux)),
		ReadTimeout:  cfg.Server.ReadTimeout.Duration,
		WriteTimeout: cfg.Server.WriteTimeout.Duration,
	}
	return s
}

// corsMiddleware adds Access-Control headers so browser-based MCP clients on
// other ports can reach the server via the streamable HTTP transport.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers",
				"Authorization, Content-Type, Accept, MCP-Protocol-Version, Mcp-Session-Id")
			w.Header().Set("Access-Control-Expose-Headers",
				"Content-Type, MCP-Session-Id, Trailer")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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
	o := resolveOpts{
		useMemory: boolOr(req.UseMemory, true),
		useCache:  boolOr(req.UseCache, true),
		maxAge:    time.Duration(req.MaxAge) * time.Second,
		remember:  req.Remember,
	}

	resp := &SearchResponse{Terms: len(req.Terms), Answers: map[string]string{}, Complete: true}
	sources := map[string]bool{}
	seen := map[string]bool{}

	// The memory -> cache -> engines chain runs per term in the resolver.
	for _, term := range req.Terms {
		out, err := s.resolver.Search(ctx, term, o)
		if err != nil {
			return nil, err
		}
		sources[out.Source] = true
		if out.Source == "memory" {
			resp.Answers[term] = out.Answer
			continue
		}
		if out.RunID != "" {
			resp.RunID = out.RunID
		}
		for _, u := range out.URLs {
			if !seen[u.URL] {
				seen[u.URL] = true
				resp.URLs = append(resp.URLs, u)
			}
		}
	}
	resp.Source = combineSources(sources)
	if len(resp.Answers) == 0 {
		resp.Answers = nil
	}

	if req.Scrape && s.cfg.Scrape.Enabled && len(resp.URLs) > 0 {
		runID := resp.RunID
		if runID == "" {
			id, err := s.h.store.StartRun(ctx, "api-scrape", "")
			if err != nil {
				return nil, err
			}
			defer s.h.store.FinishRun(ctx, id)
			runID = id
		}
		urls := make([]string, len(resp.URLs))
		for i, u := range resp.URLs {
			urls[i] = u.URL
		}
		resp.Scrapes = s.h.scraper.Scrape(ctx, runID, urls, o.useCache, o.maxAge)
		s.rememberScrapes(ctx, req.Remember, resp.Scrapes)
		if len(resp.URLs) > s.cfg.Scrape.MaxURLs {
			resp.Note = fmt.Sprintf("scraped the first %d of %d URLs (scrape.max_urls); "+
				"fetch the rest with POST /api/scrape", s.cfg.Scrape.MaxURLs, len(resp.URLs))
		}
	}
	return resp, nil
}

func (s *apiServer) runScrape(ctx context.Context, req ScrapeRequest) (*ScrapeResponse, error) {
	useCache := boolOr(req.UseCache, true)
	maxAge := time.Duration(req.MaxAge) * time.Second

	var (
		outcomes []ScrapeOutcome
		runID    string
	)
	switch {
	case req.RunID != "" && len(req.URLs) == 0:
		// Batch scrape: fetch every URL a previous run found.
		rows, err := s.h.store.RunURLs(ctx, req.RunID)
		if err != nil {
			return nil, err
		}
		urls := make([]string, len(rows))
		for i, r := range rows {
			urls[i] = r.URL
		}
		outcomes = s.h.scraper.Scrape(ctx, req.RunID, urls, useCache, maxAge)
		runID = req.RunID
	case len(req.URLs) > 0:
		if req.RunID != "" {
			// Associated with an existing search run for tracing.
			outcomes = s.h.scraper.Scrape(ctx, req.RunID, req.URLs, useCache, maxAge)
			runID = req.RunID
		} else {
			// Direct scrape: no parent search, create its own run.
			id, err := s.h.store.StartRun(ctx, "api-scrape", "")
			if err != nil {
				return nil, err
			}
			defer s.h.store.FinishRun(ctx, id)
			outcomes = s.h.scraper.Scrape(ctx, id, req.URLs, useCache, maxAge)
			runID = id
		}
	default:
		return nil, errors.New("provide either urls or run_id")
	}
	s.rememberScrapes(ctx, req.Remember, outcomes)
	return &ScrapeResponse{RunID: runID, Results: outcomes}, nil
}

// boolOr resolves an optional bool flag to its default when unset.
func boolOr(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

// combineSources labels a multi-term search by where its results came from.
func combineSources(m map[string]bool) string {
	switch len(m) {
	case 0:
		return "live"
	case 1:
		for k := range m {
			return k
		}
	}
	return "mixed"
}

// rememberTier interprets the remember flag: off disables storage, an explicit
// tier is used as-is, anything else defaults to on at the configured tier.
func rememberTier(remember, def string) (store bool, tier string) {
	switch strings.ToLower(strings.TrimSpace(remember)) {
	case "off", "none", "no", "false":
		return false, ""
	case tierShort, tierLong, tierPermanent:
		return true, remember
	default:
		return true, def
	}
}

// rememberScrapes enqueues distillation of successful scrapes into memory unless
// the caller turned remembering off.
func (s *apiServer) rememberScrapes(ctx context.Context, remember string, outcomes []ScrapeOutcome) {
	store, tier := rememberTier(remember, s.cfg.Memory.RememberDefault)
	if !store {
		return
	}
	for _, o := range outcomes {
		if o.ScrapeID != "" && o.TextChars > 0 {
			if err := enqueueDistill(ctx, s.h.store, o.ScrapeID, tier); err != nil {
				s.h.log.Printf("remember: enqueue distill for %s: %v", o.ScrapeID, err)
			}
		}
	}
}

// ---- memory operations, shared by REST and MCP ----

type MemoryQueryRequest struct {
	Question string `json:"question" jsonschema:"the question to try to answer from memory"`
}

type MemoryFactView struct {
	Text       string  `json:"text"`
	SourceURL  string  `json:"source_url,omitempty"`
	Volatility string  `json:"volatility,omitempty"`
	Similarity float64 `json:"similarity,omitempty"`
}

type MemoryQueryResponse struct {
	Found  bool             `json:"found"`
	Answer string           `json:"answer,omitempty"`
	Facts  []MemoryFactView `json:"facts,omitempty"`
}

type MemoryStoreRequest struct {
	Text       string `json:"text" jsonschema:"the fact to remember, stated so it stands alone"`
	SourceURL  string `json:"source_url,omitempty" jsonschema:"where the fact came from"`
	Volatility string `json:"volatility,omitempty" jsonschema:"stable or time-sensitive"`
	Remember   string `json:"remember,omitempty" jsonschema:"durability: short, long or permanent (default short)"`
}

type MemoryStoreResponse struct {
	ID string `json:"id"`
}

func (s *apiServer) runMemoryQuery(ctx context.Context, req MemoryQueryRequest) (*MemoryQueryResponse, error) {
	if strings.TrimSpace(req.Question) == "" {
		return nil, errors.New("question is required")
	}
	ans, facts, ok, err := MemoryAnswer(ctx, s.h.store, s.llm, s.cfg.Cache, s.cfg.Memory, req.Question)
	if err != nil {
		return nil, err
	}
	resp := &MemoryQueryResponse{Found: ok, Answer: ans}
	for _, f := range facts {
		resp.Facts = append(resp.Facts, MemoryFactView{
			Text: f.Text, SourceURL: f.SourceURL, Volatility: f.Volatility, Similarity: f.Similarity,
		})
	}
	return resp, nil
}

func (s *apiServer) runMemoryStore(ctx context.Context, req MemoryStoreRequest) (*MemoryStoreResponse, error) {
	if strings.TrimSpace(req.Text) == "" {
		return nil, errors.New("text is required")
	}
	store, tier := rememberTier(req.Remember, s.cfg.Memory.RememberDefault)
	if !store {
		return nil, errors.New("remember is off; nothing stored")
	}
	id, err := s.h.store.StoreFact(ctx, s.cfg.Cache, s.cfg.Memory, s.llm, req.Text, req.SourceURL, req.Volatility, tier)
	if err != nil {
		return nil, err
	}
	return &MemoryStoreResponse{ID: id}, nil
}

func (s *apiServer) handleMemoryQuery(w http.ResponseWriter, r *http.Request) {
	var req MemoryQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.runMemoryQuery(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *apiServer) handleMemoryStore(w http.ResponseWriter, r *http.Request) {
	var req MemoryStoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	resp, err := s.runMemoryStore(r.Context(), req)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *apiServer) mcpMemoryQuery(ctx context.Context, _ *mcp.CallToolRequest, in MemoryQueryRequest) (
	*mcp.CallToolResult, MemoryQueryResponse, error) {
	resp, err := s.runMemoryQuery(ctx, in)
	if err != nil {
		return nil, MemoryQueryResponse{}, err
	}
	return nil, *resp, nil
}

func (s *apiServer) mcpMemoryStore(ctx context.Context, _ *mcp.CallToolRequest, in MemoryStoreRequest) (
	*mcp.CallToolResult, MemoryStoreResponse, error) {
	resp, err := s.runMemoryStore(ctx, in)
	if err != nil {
		return nil, MemoryStoreResponse{}, err
	}
	return nil, *resp, nil
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
	server := mcp.NewServer(&mcp.Implementation{Name: "go-web-search", Version: "0.1.0"}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name: "web_search",
		Description: "Search the web for one or more terms and return the destination URLs. " +
			"Memory is consulted first and, when it can answer confidently, the response carries an " +
			"'answers' entry with source='memory' instead of hitting the web; otherwise a cached or " +
			"live result is returned, tagged by 'source' (memory|cache|live|mixed). use_cache and " +
			"use_memory default true; set either false to bypass. remember (short|long|permanent|off) " +
			"sets how durably anything scraped is kept. Set scrape=true to also fetch page contents. " +
			"Full details stay retrievable by id.",
	}, s.mcpSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name: "web_scrape",
		Description: "Fetch and clean page contents. When selecting urls from a web_search result, " +
			"pick broadly: aim for at least one URL per domain, skip obvious navigation links " +
			"like /roster, /stats, /news, /videos, /about, and don't cluster on a single site. " +
			"Pass the run_id from web_search to link scrapes back to that search for tracing. " +
			"For direct scrapes (no prior search), just pass urls — a run is created automatically. " +
			"use_cache defaults true (a fresh cached copy is returned without refetching; provenance " +
			"is reported in fetched_with as cache/cache-revalidated/http/browser). remember " +
			"(short|long|permanent|off) distils the pages into memory for later. Returns text " +
			"snippets plus scrape ids for the full documents.",
	}, s.mcpScrape)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_scrape",
		Description: "Return the full cleaned text, title and images of a single scrape by id.",
	}, s.mcpGetScrape)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_run",
		Description: "Summarise a run by id: how many searches, urls and scrapes it produced.",
	}, s.mcpGetRun)

	mcp.AddTool(server, &mcp.Tool{
		Name: "memory_query",
		Description: "Ask what is already known, from facts distilled out of earlier scrapes. " +
			"Returns found=true with a synthesized answer only when memory clears the confidence " +
			"gates; otherwise found=false and you should web_search instead.",
	}, s.mcpMemoryQuery)

	mcp.AddTool(server, &mcp.Tool{
		Name: "memory_store",
		Description: "Remember a single self-contained fact directly, without scraping. " +
			"remember sets durability (short|long|permanent).",
	}, s.mcpMemoryStore)

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

	// Background job system: worker pool + poller + reaper. Handlers (embed,
	// distill, cleanup, re-embed) are registered on the runner as those
	// subsystems land. Cancelling jobCtx on shutdown stops every goroutine.
	jobCtx, jobCancel := context.WithCancel(context.Background())
	defer jobCancel()
	llm := newLLMClient(cfg.LLM, 60*time.Second)
	runner := newJobRunner(h.store, art.Log, cfg.Database.MaxOpenConns, time.Second)
	registerJobs(runner, cfg, h, llm)
	runner.Start(jobCtx)
	if err := bootVectors(jobCtx, h.store, llm, art.Log); err != nil {
		art.Log.Printf("WARNING: vector boot/migration check failed: %v", err)
	}

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
