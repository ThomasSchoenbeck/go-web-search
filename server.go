package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
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
	// embed is the semantic explorer's embedding source. It is the LLM client in
	// every real mode; the test harness swaps in a deterministic stub so e2e runs
	// need no model endpoint.
	embed queryEmbedder
	// logs is the separate log database, threaded in from whoever opened it, so
	// the logs endpoint can read what the batching writer has written. It is not
	// part of the main Store.
	logs *LogStore
}

// ---- request and response shapes, shared by REST and MCP ----

type SearchRequest struct {
	Terms     []string `json:"terms" jsonschema:"search terms to run"`
	Scrape    bool     `json:"scrape,omitempty" jsonschema:"also fetch page contents for the URLs found; set true ONLY when the user explicitly asked to scrape"`
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
	URLs          []string `json:"urls" jsonschema:"absolute URLs to fetch"`
	RunID         string   `json:"run_id,omitempty" jsonschema:"when passed with urls, links scrapes to the search that found them; when passed without urls, scrapes everything that run found (use only when full coverage is needed)"`
	UseCache      *bool    `json:"use_cache,omitempty" jsonschema:"reuse the scrape cache (default true); set false to force a fresh fetch"`
	MaxAge        int      `json:"max_age_seconds,omitempty" jsonschema:"reject cached content older than this many seconds"`
	Remember      string   `json:"remember,omitempty" jsonschema:"distil scraped pages into memory at this durability: short, long, permanent, or off (default short)"`
	DistillDetail string   `json:"distill_detail,omitempty" jsonschema:"how many facts to distil per page: brief, normal (default), or thorough"`
	DistillFocus  string   `json:"distill_focus,omitempty" jsonschema:"optional free-text guidance on what to prioritise when distilling, e.g. 'upcoming game dates and opponents only'"`
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

func newAPIServer(cfg Config, h *harvester, logs *LogStore) *apiServer {
	llm := newLLMClient(cfg.LLM, cfg.LLM.Timeout.Duration, h.log)
	s := &apiServer{cfg: cfg, h: h, llm: llm, resolver: newResolver(cfg, h.store, llm, h), embed: llm, logs: logs}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/search", s.handleSearch)
	mux.HandleFunc("POST /api/scrape", s.handleScrape)
	mux.HandleFunc("POST /api/memory/query", s.handleMemoryQuery)
	mux.HandleFunc("POST /api/memory/store", s.handleMemoryStore)
	mux.HandleFunc("GET /api/memory/facts", s.handleListFacts)
	mux.HandleFunc("GET /api/memory/facts/{id}", s.handleGetFact)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/ui-config", s.handleUIConfig)
	mux.HandleFunc("POST /api/distill/preview", s.handleDistillPreview)
	mux.HandleFunc("POST /api/vacuum", s.handleVacuum)
	mux.HandleFunc("GET /api/runs", s.handleListRuns)
	mux.HandleFunc("GET /api/runs/{id}", s.handleGetRun)
	mux.HandleFunc("GET /api/runs/{id}/urls", s.handleRunURLs)
	mux.HandleFunc("GET /api/runs/{id}/searches", s.handleRunSearches)
	mux.HandleFunc("GET /api/runs/{id}/scrapes", s.handleRunScrapes)
	mux.HandleFunc("GET /api/runs/{id}/causality", s.handleRunCausality)
	mux.HandleFunc("GET /api/jobs", s.handleListJobs)
	mux.HandleFunc("GET /api/cache/searches", s.handleListSearchCache)
	mux.HandleFunc("GET /api/cache/scrapes", s.handleListScrapeCache)
	mux.HandleFunc("GET /api/logs", s.handleListLogs)
	mux.HandleFunc("GET /api/provenance", s.handleProvenance)
	mux.HandleFunc("GET /api/explore", s.handleExplore)
	mux.HandleFunc("GET /api/projection", s.handleProjection)
	mux.HandleFunc("GET /api/searches/{id}/raw", s.handleSearchRaw)
	mux.HandleFunc("GET /api/scrapes/{id}", s.handleGetScrape)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// The embedded observability SPA at the root. Registered without a method:
	// "GET /" would be ambiguous against the method-less "/mcp" below, which
	// ServeMux rejects at registration. Every pattern here has a more specific
	// path, so the API, MCP and health routes stay ahead of the fallback.
	mux.HandleFunc("/", s.handleSPA)

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
		s.rememberScrapes(ctx, req.Remember, "", "", resp.Scrapes)
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
	s.rememberScrapes(ctx, req.Remember, req.DistillDetail, req.DistillFocus, outcomes)
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
func (s *apiServer) rememberScrapes(ctx context.Context, remember, detail, focus string, outcomes []ScrapeOutcome) {
	store, tier := rememberTier(remember, s.cfg.Memory.RememberDefault)
	if !store {
		return
	}
	for _, o := range outcomes {
		if o.ScrapeID != "" && o.TextChars > 0 {
			if err := enqueueDistill(ctx, s.h.store, o.ScrapeID, tier, detail, focus); err != nil {
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

// handleListFacts browses distilled memory facts. Query params: limit, offset, q
// (case-insensitive text substring).
func (s *apiServer) handleListFacts(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	facts, err := s.h.store.ListFacts(r.Context(), r.URL.Query().Get("q"), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(facts), "facts": facts})
}

// FactDetail is a single fact plus the basis it was distilled from: the source
// scrape's sizes (the bloat signal) and a link to read the raw material.
type FactDetail struct {
	Fact    FactSummary  `json:"fact"`
	Source  *ScrapeSizes `json:"source,omitempty"`
	ReadRaw string       `json:"read_raw,omitempty"`
	Note    string       `json:"note,omitempty"`
}

func (s *apiServer) handleGetFact(w http.ResponseWriter, r *http.Request) {
	f, ok, err := s.h.store.scanFact(r.Context(), r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeErr(w, http.StatusNotFound, errors.New("fact not found"))
		return
	}
	detail := FactDetail{Fact: FactSummary{
		ID: f.ID, Text: f.Text, TextChars: len(f.Text), SourceURL: f.SourceURL,
		Volatility: f.Volatility, Tier: f.Tier, HitCount: f.HitCount, ExpiresAt: f.ExpiresAt,
	}}
	if f.SourceURL != "" {
		if sz, found, serr := s.h.store.ScrapeSizesByURL(r.Context(), f.SourceURL); serr == nil {
			if found {
				detail.Source = sz
				detail.ReadRaw = fmt.Sprintf("/api/scrapes/%s?raw=1", sz.ScrapeID)
			} else {
				detail.Note = "source page is no longer cached, so the raw material can't be retrieved"
			}
		}
	}
	writeJSON(w, http.StatusOK, detail)
}

// UIConfig is the non-secret slice of config.toml the observability SPA reads
// at startup. It is built field by field rather than by marshalling
// ObservabilityConfig, so adding a field to config can never leak one here by
// accident. Milliseconds because the consumer is JavaScript.
type UIConfig struct {
	PollIntervalMS      int64 `json:"poll_interval_ms"`
	PollEnabled         bool  `json:"poll_enabled"`
	ProjectionSampleCap int   `json:"projection_sample_cap"`
}

func (s *apiServer) handleUIConfig(w http.ResponseWriter, r *http.Request) {
	o := s.cfg.Observability
	writeJSON(w, http.StatusOK, UIConfig{
		PollIntervalMS:      o.PollInterval.Duration.Milliseconds(),
		PollEnabled:         o.PollEnabled,
		ProjectionSampleCap: o.ProjectionSampleCap,
	})
}

func (s *apiServer) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.h.store.Stats(r.Context(), s.cfg.Observability.JobTimingSample)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

// DistillPreviewRequest runs the distiller against one scrape synchronously with
// overridable settings, storing nothing — a bench for tuning prompt/settings.
type DistillPreviewRequest struct {
	ScrapeID      string `json:"scrape_id"`
	Detail        string `json:"detail,omitempty"`          // brief|normal|thorough
	Focus         string `json:"focus,omitempty"`           // free-text guidance
	SystemPrompt  string `json:"system_prompt,omitempty"`   // full override; ignores detail/focus when set
	MaxInputChars int    `json:"max_input_chars,omitempty"` // 0 = distillMaxChars
	MaxTokens     *int   `json:"max_tokens,omitempty"`
	NoThink       *bool  `json:"no_think,omitempty"`
}

type DistillPreviewResponse struct {
	SystemPrompt     string          `json:"system_prompt"`
	InputChars       int             `json:"input_chars"`
	FactCount        int             `json:"fact_count"`
	Facts            []extractedFact `json:"facts"`
	PromptTokens     int             `json:"prompt_tokens"`
	CompletionTokens int             `json:"completion_tokens"`
	TokPerSec        float64         `json:"tok_per_sec"`
	DurationMS       int64           `json:"duration_ms"`
}

func (s *apiServer) handleDistillPreview(w http.ResponseWriter, r *http.Request) {
	var req DistillPreviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if strings.TrimSpace(req.ScrapeID) == "" {
		writeErr(w, http.StatusBadRequest, errors.New("scrape_id is required"))
		return
	}
	scrape, err := s.h.store.GetScrape(r.Context(), req.ScrapeID, false)
	if err != nil {
		writeErr(w, http.StatusNotFound, err)
		return
	}
	text := scrape.Text
	limit := req.MaxInputChars
	if limit <= 0 {
		limit = distillMaxChars
	}
	if len(text) > limit {
		text = text[:limit]
	}
	sys := req.SystemPrompt
	if strings.TrimSpace(sys) == "" {
		sys = distillSystemPrompt(req.Detail, req.Focus)
	}
	facts, usage, err := extractFacts(r.Context(), s.llm, sys, text, chatParams{maxTokens: req.MaxTokens, noThink: req.NoThink})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	resp := DistillPreviewResponse{
		SystemPrompt:     sys,
		InputChars:       len(text),
		FactCount:        len(facts),
		Facts:            facts,
		PromptTokens:     usage.PromptTokens,
		CompletionTokens: usage.CompletionTokens,
		DurationMS:       usage.Duration.Milliseconds(),
	}
	if usage.CompletionTokens > 0 && usage.Duration > 0 {
		resp.TokPerSec = float64(usage.CompletionTokens) / usage.Duration.Seconds()
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

// handleProvenance pivots on a URL. An unknown URL is a normal, empty answer —
// not a 404 — because "nothing points at this yet" is a real observation.
func (s *apiServer) handleProvenance(w http.ResponseWriter, r *http.Request) {
	rawURL := strings.TrimSpace(r.URL.Query().Get("url"))
	if rawURL == "" {
		writeErr(w, http.StatusBadRequest, errors.New("url query parameter is required"))
		return
	}
	chain, err := s.h.store.URLProvenance(r.Context(), rawURL)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, chain)
}

// handleListJobs browses the background queue. Query params: status, type,
// limit, offset. Read-only: nothing here claims, retries or deletes a job.
func (s *apiServer) handleListJobs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	jobs, err := s.h.store.ListJobs(r.Context(), q.Get("status"), q.Get("type"), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	counts, err := s.h.store.JobStatusCounts(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, JobsPage{Jobs: jobs, Counts: counts})
}

// handleListSearchCache browses the search cache. Query params: tier, q (query
// substring), limit, offset. The stored results blob is summarised, not served.
func (s *apiServer) handleListSearchCache(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	rows, err := s.h.store.ListSearchCache(r.Context(), q.Get("tier"), q.Get("q"), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(rows), "entries": rows})
}

// handleListScrapeCache browses the scrape cache. Query params: tier, q (URL
// substring), limit, offset. Content sizes only — the bodies stay behind
// /api/scrapes/{id}.
func (s *apiServer) handleListScrapeCache(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	rows, err := s.h.store.ListScrapeCache(r.Context(), q.Get("tier"), q.Get("q"), limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(rows), "entries": rows})
}

// handleListLogs queries the separate log database. Query params: run_id,
// level, source, limit, offset. Newest-first, so the viewer reads as a tail.
func (s *apiServer) handleListLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))
	entries, err := s.logs.Query(r.Context(), LogQuery{
		RunID:  q.Get("run_id"),
		Level:  q.Get("level"),
		Source: q.Get("source"),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"count": len(entries), "entries": entries})
}

// handleExplore is the raw nearest-neighbour probe. Unlike /api/memory/query it
// gates nothing and synthesizes nothing — it reports what is near, and how near.
func (s *apiServer) handleExplore(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeErr(w, http.StatusBadRequest, errors.New("q query parameter is required"))
		return
	}
	k, _ := strconv.Atoi(r.URL.Query().Get("k"))
	result, err := s.h.store.Explore(r.Context(), s.embed, query, k)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// handleProjection dumps raw embeddings for the browser to lay out. The sample
// cap is config-driven (observability.projection_sample_cap) because this reads
// whole vectors, not distances — an uncapped dump is a very large response.
func (s *apiServer) handleProjection(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	dump, err := s.h.store.VectorProjection(r.Context(), s.cfg.Observability.ProjectionSampleCap, limit, offset)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, dump)
}

// handleRunCausality returns the whole-run graph. An unknown run id yields an
// empty graph rather than an error, matching the URL pivot.
func (s *apiServer) handleRunCausality(w http.ResponseWriter, r *http.Request) {
	graph, err := s.h.store.RunCausality(r.Context(), r.PathValue("id"), s.cfg.Observability.CausalityMaxURLs)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, graph)
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
		Description: "Search the web for terms and return destination URLs, tagged by source " +
			"(memory|cache|live|mixed); memory and cache are checked before the engines. Set " +
			"scrape=true ONLY if the user explicitly asked to fetch page contents directly. Don't re-run the " +
			"same query to retry — repeats trigger the engines' bot detection.",
	}, s.mcpSearch)

	mcp.AddTool(server, &mcp.Tool{
		Name: "web_scrape",
		Description: "Fetch and clean page contents for the given URLs; returns text snippets plus " +
			"scrape ids. Pick broadly — about 1-2 URL per domain, skip pure navigation links. When the " +
			"user says \"scrape all\", don't cherry-pick: fetch every not-yet-scraped URL from this " +
			"conversation's searches that belong to the current topic (pass the full url list, or a run_id with no urls); if you'd skip " +
			"any, ask first.",
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
		Description: "Ask what is already known. " +
			"Returns found=true with a synthesized answer only when memory clears the confidence " +
			"gates; otherwise found=false and you should web_search instead.",
	}, s.mcpMemoryQuery)

	mcp.AddTool(server, &mcp.Tool{
		Name: "memory_store",
		Description: "Remember a single self-contained fact directly. " +
			"remember sets durability (short|long|permanent).",
	}, s.mcpMemoryStore)

	mcp.AddTool(server, &mcp.Tool{
		Name: "current_time",
		Description: "ALWAYS ask me first in a conversation before executing search. " +
			"Return the current date and time of the machine running this server.",
	}, s.mcpCurrentTime)

	mcp.AddTool(server, &mcp.Tool{
		Name: "approximate_location",
		Description: "Return the APPROXIMATE geographic location of the machine running this " +
			"server, derived from its public IP via ipinfo.io. Only approximate: it reflects the " +
			"ISP/network location, not a precise position, and can be off by a city or more. " +
			"Returns city, region, country, lat/long, postal code and timezone.",
	}, s.mcpApproxLocation)

	return server
}

type currentTimeInput struct{}

type currentTimeOutput struct {
	RFC3339  string `json:"rfc3339"`
	Unix     int64  `json:"unix"`
	Timezone string `json:"timezone"`
	Weekday  string `json:"weekday"`
	Human    string `json:"human"`
}

func (s *apiServer) mcpCurrentTime(_ context.Context, _ *mcp.CallToolRequest, _ currentTimeInput) (
	*mcp.CallToolResult, currentTimeOutput, error) {

	now := time.Now()
	zone, _ := now.Zone()
	return nil, currentTimeOutput{
		RFC3339:  now.Format(time.RFC3339),
		Unix:     now.Unix(),
		Timezone: zone,
		Weekday:  now.Weekday().String(),
		Human:    now.Format("Monday, 2 January 2006, 15:04:05 MST"),
	}, nil
}

type approxLocationInput struct{}

// approxLocationOutput mirrors the ipinfo.io fields we keep. Decoding the full
// response into just these fields silently drops ip, hostname, org and readme.
type approxLocationOutput struct {
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"` // "lat,long"
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
}

func (s *apiServer) mcpApproxLocation(ctx context.Context, _ *mcp.CallToolRequest, _ approxLocationInput) (
	*mcp.CallToolResult, approxLocationOutput, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://ipinfo.io/json", nil)
	if err != nil {
		return nil, approxLocationOutput{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, approxLocationOutput{}, fmt.Errorf("ipinfo.io: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return nil, approxLocationOutput{}, fmt.Errorf("ipinfo.io: status %d", resp.StatusCode)
	}

	var out approxLocationOutput
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, approxLocationOutput{}, fmt.Errorf("ipinfo.io: decoding response: %w", err)
	}
	return nil, out, nil
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

// serveMode runs the HTTP server until a signal arrives. logs is the log
// database opened in main, threaded through so the logs endpoint can read it.
func serveMode(cfg Config, art *artifacts, h *harvester, logs *LogStore, stop context.Context) error {
	srv := newAPIServer(cfg, h, logs)

	// Background job system: worker pool + poller + reaper. Handlers (embed,
	// distill, cleanup, re-embed) are registered on the runner as those
	// subsystems land. Cancelling jobCtx on shutdown stops every goroutine.
	jobCtx, jobCancel := context.WithCancel(context.Background())
	defer jobCancel()
	llm := newLLMClient(cfg.LLM, cfg.LLM.Timeout.Duration, art.Log)
	runner := newJobRunner(h.store, art.Log, cfg.Database.MaxOpenConns, time.Second, cfg.Jobs.StaleAfter.Duration)
	registerJobs(runner, cfg, h, llm)

	// Startup maintenance, before any worker holds a connection: trim processed
	// blobs, then VACUUM to hand the freed space back (VACUUM needs exclusive
	// access, which is why it runs here rather than mid-serve).
	if cfg.Retention.TrimRaw {
		if err := h.store.TrimRawContent(jobCtx, cfg.Retention.RawMaxAge.Duration, cfg.Retention.RawKeepLast); err != nil {
			art.Log.Printf("WARNING: startup trim failed: %v", err)
		}
	}
	if cfg.Retention.VacuumOnStartup {
		art.Log.Printf("vacuuming database on startup...")
		vstart := time.Now()
		if err := h.store.Vacuum(jobCtx); err != nil {
			art.Log.Printf("WARNING: startup vacuum failed: %v", err)
		} else {
			art.Log.Printf("vacuum done in %s", time.Since(vstart).Round(time.Millisecond))
		}
	}
	if cfg.Retention.VacuumAt != "" {
		go scheduledVacuum(jobCtx, cfg.Retention.VacuumAt, h.store, art.Log)
	}

	runner.Start(jobCtx)
	if err := bootVectors(jobCtx, h.store, llm, art.Log); err != nil {
		// A failure here leaves no active vector table, so every embed job will
		// fail with "no active vector table yet" until it is fixed. Make it loud
		// rather than a soft warning that scrolls past unnoticed.
		art.Log.Printf("ERROR: vector store did NOT initialise; embeddings and semantic memory are disabled until fixed: %v", err)
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

// scheduledVacuum runs VACUUM once a day at the given "HH:MM" local time until
// ctx is cancelled. VACUUM stalls all queries while it runs, hence off-peak.
func scheduledVacuum(ctx context.Context, at string, store *Store, logger *log.Logger) {
	hm, err := time.Parse("15:04", at)
	if err != nil {
		logger.Printf("WARNING: bad vacuum_at %q (want HH:MM): %v", at, err)
		return
	}
	for {
		now := time.Now()
		next := time.Date(now.Year(), now.Month(), now.Day(), hm.Hour(), hm.Minute(), 0, 0, now.Location())
		if !next.After(now) {
			next = next.Add(24 * time.Hour)
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			logger.Printf("scheduled vacuum starting")
			start := time.Now()
			if err := store.Vacuum(ctx); err != nil {
				logger.Printf("WARNING: scheduled vacuum failed: %v", err)
			} else {
				logger.Printf("scheduled vacuum done in %s", time.Since(start).Round(time.Millisecond))
			}
		}
	}
}

// VacuumResponse reports the outcome of an on-demand VACUUM, including how much
// the file shrank (best-effort file-size read).
type VacuumResponse struct {
	DurationMS     int64 `json:"duration_ms"`
	BeforeBytes    int64 `json:"before_bytes,omitempty"`
	AfterBytes     int64 `json:"after_bytes,omitempty"`
	ReclaimedBytes int64 `json:"reclaimed_bytes,omitempty"`
}

func (s *apiServer) handleVacuum(w http.ResponseWriter, r *http.Request) {
	dbPath := filepath.Join(s.cfg.Database.DataDir, s.cfg.Database.MainDB)
	before := fileSize(dbPath)
	start := time.Now()
	if err := s.h.store.Vacuum(r.Context()); err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	after := fileSize(dbPath)
	resp := VacuumResponse{DurationMS: time.Since(start).Milliseconds(), BeforeBytes: before, AfterBytes: after}
	if before > 0 && after > 0 {
		resp.ReclaimedBytes = before - after
	}
	writeJSON(w, http.StatusOK, resp)
}

func fileSize(path string) int64 {
	if fi, err := os.Stat(path); err == nil {
		return fi.Size()
	}
	return 0
}
