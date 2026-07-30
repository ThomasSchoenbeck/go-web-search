package main

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// resolveOpts controls a resolve. Cache and memory default on; a caller disables
// them per request. remember selects the tier for anything stored as a result.
type resolveOpts struct {
	useMemory bool
	useCache  bool
	maxAge    time.Duration
	remember  string
}

// SearchOutcome is the tagged result of a search resolve. Source is one of
// "memory", "cache" or "live", so the caller always knows where it came from.
type SearchOutcome struct {
	Source string       `json:"source"`
	Answer string       `json:"answer,omitempty"`
	Facts  []MemoryFact `json:"-"`
	URLs   []CachedURL  `json:"urls,omitempty"`
	RunID  string       `json:"run_id,omitempty"`
}

// resolver ties memory, the caches and the live engines into one chain, keeping
// the ordering (memory -> cache -> engines) in code rather than in a tool
// description.
type resolver struct {
	cfg   Config
	store *Store
	llm   *LLMClient
	h     *harvester
}

func newResolver(cfg Config, store *Store, llm *LLMClient, h *harvester) *resolver {
	return &resolver{cfg: cfg, store: store, llm: llm, h: h}
}

// looksLikeURL is the dispatch test: an absolute http(s) URL goes to the scrape
// path, anything else to the search path.
func looksLikeURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// Search runs the chain for a text query: memory first (may answer and skip the
// web), then the search cache (exact then semantic), then the live engines,
// caching the fresh result on the way out.
func (r *resolver) Search(ctx context.Context, query string, o resolveOpts) (*SearchOutcome, error) {
	if o.useMemory {
		if ans, facts, ok, err := MemoryAnswer(ctx, r.store, r.llm, r.cfg.Cache, r.cfg.Memory, query); err == nil && ok {
			return &SearchOutcome{Source: "memory", Answer: ans, Facts: facts}, nil
		}
	}

	if o.useCache {
		if e, ok, err := r.store.LookupSearchCacheExact(ctx, r.cfg.Cache, query, o.maxAge); err == nil && ok {
			return &SearchOutcome{Source: "cache", URLs: e.Results}, nil
		}
		if qv, err := r.llm.Embed(ctx, []string{query}, true); err == nil && len(qv) == 1 {
			if e, ok, err := r.store.LookupSearchCacheSemantic(ctx, r.cfg.Cache, r.cfg.Memory, qv[0], o.maxAge); err == nil && ok {
				return &SearchOutcome{Source: "cache", URLs: e.Results}, nil
			}
		}
	}

	// Live: run the engines for this one query.
	runID, err := r.store.StartRun(ctx, "resolve-search", "")
	if err != nil {
		return nil, err
	}
	defer r.store.FinishRun(ctx, runID)

	never := context.Background()
	if _, err := r.h.SearchTerms(ctx, runID, []string{query}, never); err != nil {
		return nil, err
	}
	rows, err := r.store.RunURLs(ctx, runID)
	if err != nil {
		return nil, err
	}
	urls := make([]CachedURL, len(rows))
	for i, u := range rows {
		urls[i] = CachedURL{URL: u.URL, Domain: u.Domain, Rank: u.Rank}
	}

	if o.useCache {
		tier := normalizeTier(o.remember, r.cfg.Memory.RememberDefault)
		if _, err := r.store.StoreSearchCache(ctx, r.cfg.Cache, query, urls, tier); err != nil {
			r.h.log.Printf("resolve: caching search %q: %v", query, err)
		}
	}
	return &SearchOutcome{Source: "live", URLs: urls, RunID: runID}, nil
}
