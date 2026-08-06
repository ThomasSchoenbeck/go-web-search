package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Provenance answers "what caused what" over data already stored: which searches
// found a URL, what was scraped from it, and which facts were distilled out of
// that scrape. Two shapes, both read-only:
//
//   - URLProvenance pivots on one URL (T009).
//   - CausalityGraph assembles a whole run (T025).
//
// Vectors live in a generation table whose name comes from system_meta, and that
// table is unavailable while a re-embed migration runs. Every query here treats
// that as "vector presence unknown" rather than an error, mirroring Stats.

// FoundBySearch is one search that turned up a URL, and where it ranked.
type FoundBySearch struct {
	SearchID   string `json:"search_id"`
	RunID      string `json:"run_id"`
	Term       string `json:"term"`
	Engine     string `json:"engine"`
	SearchMode string `json:"search_mode"`
	Rank       int    `json:"rank"`
	CreatedAt  string `json:"created_at"`
}

// FactProvenance is a distilled fact plus whether it currently has an embedding.
type FactProvenance struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	SourceURL  string `json:"source_url,omitempty"`
	Volatility string `json:"volatility,omitempty"`
	Tier       string `json:"tier,omitempty"`
	HasVector  bool   `json:"has_vector"`
	CreatedAt  string `json:"created_at"`
}

// URLProvenance is the chain around a single URL, in both directions.
type URLProvenance struct {
	URL   string `json:"url"`
	URLID string `json:"url_id,omitempty"`
	Known bool   `json:"known"`

	// Backward: what led here.
	FoundBy []FoundBySearch `json:"found_by"`

	// Forward: what came from here.
	Scrape *ScrapeSizes     `json:"scrape,omitempty"`
	Facts  []FactProvenance `json:"facts"`

	// VectorsAvailable is false when no active vector table exists or a re-embed
	// migration is in flight; HasVector on each fact is then meaningless.
	VectorsAvailable bool   `json:"vectors_available"`
	Note             string `json:"note,omitempty"`
}

// SearchesFindingURL lists the searches that returned a URL, best rank first.
func (s *Store) SearchesFindingURL(ctx context.Context, rawURL string) ([]FoundBySearch, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT se.id, se.run_id, se.term, se.engine, se.search_mode, su.rank, se.created_at
		   FROM urls u
		   JOIN search_urls su ON su.url_id = u.id
		   JOIN searches se ON se.id = su.search_id
		  WHERE u.url = ?
		  ORDER BY su.rank, se.created_at`, rawURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FoundBySearch
	for rows.Next() {
		var f FoundBySearch
		if err := rows.Scan(&f.SearchID, &f.RunID, &f.Term, &f.Engine, &f.SearchMode, &f.Rank, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// FactsBySourceURL lists the memory facts distilled from a URL.
func (s *Store) FactsBySourceURL(ctx context.Context, rawURL string) ([]FactProvenance, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, text, COALESCE(source_url, ''), COALESCE(volatility, ''), tier, created_at
		   FROM memory_facts WHERE source_url = ? ORDER BY created_at`, rawURL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FactProvenance
	for rows.Next() {
		var f FactProvenance
		if err := rows.Scan(&f.ID, &f.Text, &f.SourceURL, &f.Volatility, &f.Tier, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// urlID resolves a URL to its registry id, if the URL was ever seen.
func (s *Store) urlID(ctx context.Context, rawURL string) (string, bool, error) {
	var id string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM urls WHERE url = ?`, rawURL).Scan(&id)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return id, err == nil, err
}

// markVectors sets HasVector on facts that have an embedding in the active
// table. Returns false when vector state cannot be determined — no active
// table, or a re-embed in flight — which callers surface rather than fail on.
func (s *Store) markVectors(ctx context.Context, facts []FactProvenance) (bool, error) {
	if len(facts) == 0 {
		table, ready, err := s.activeVectorTable(ctx)
		return table != "" && ready, err
	}

	table, ready, err := s.activeVectorTable(ctx)
	if err != nil {
		return false, err
	}
	if table == "" || !ready {
		return false, nil
	}

	ids := make([]any, len(facts))
	for i, f := range facts {
		ids[i] = f.ID
	}
	// The table name comes from system_meta and is generated internally, so it is
	// safe to interpolate; the ids stay bound.
	query := fmt.Sprintf(
		`SELECT id FROM %s WHERE owner_kind = 'memory' AND id IN (%s)`,
		table, placeholders(len(ids)))
	rows, err := s.db.QueryContext(ctx, query, ids...)
	if err != nil {
		if isNoSuchTable(err) {
			return false, nil
		}
		return false, err
	}
	defer rows.Close()

	present := make(map[string]bool, len(ids))
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return false, err
		}
		present[id] = true
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	for i := range facts {
		facts[i].HasVector = present[facts[i].ID]
	}
	return true, nil
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}

const vectorsUnavailableNote = "vector store unavailable (no active table or re-embed in progress); vector presence not reported"

// URLProvenance assembles the backward and forward chain around one URL.
func (s *Store) URLProvenance(ctx context.Context, rawURL string) (*URLProvenance, error) {
	p := &URLProvenance{URL: rawURL, FoundBy: []FoundBySearch{}, Facts: []FactProvenance{}}

	id, known, err := s.urlID(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	p.URLID, p.Known = id, known

	found, err := s.SearchesFindingURL(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if found != nil {
		p.FoundBy = found
	}

	scrape, ok, err := s.ScrapeSizesByURL(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if ok {
		p.Scrape = scrape
	}

	facts, err := s.FactsBySourceURL(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	if facts != nil {
		p.Facts = facts
	}

	available, err := s.markVectors(ctx, p.Facts)
	if err != nil {
		return nil, err
	}
	p.VectorsAvailable = available
	if !available {
		p.Note = vectorsUnavailableNote
	}
	// A URL can be scraped without ever having been a search result, so "known"
	// means the registry knows it, not that anything points at it.
	if !p.Known && p.Scrape != nil {
		p.Known = true
	}
	return p, nil
}

// ---- whole-run causality graph (T025) ----

// CausalityNode is one element of the run graph. ID is namespaced by kind so
// node ids stay unique across kinds; RefID is the underlying row id the UI links
// to.
type CausalityNode struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"` // search | url | scrape | fact
	RefID  string `json:"ref_id"`
	Label  string `json:"label"`
	Detail string `json:"detail,omitempty"`
	// URL nodes carry their URL so the view can link to provenance directly.
	URL       string `json:"url,omitempty"`
	HasVector bool   `json:"has_vector,omitempty"`
}

// CausalityEdge connects two nodes. Rank is set on search→url edges.
type CausalityEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Rank int    `json:"rank,omitempty"`
}

type CausalityGraph struct {
	RunID string          `json:"run_id"`
	Nodes []CausalityNode `json:"nodes"`
	Edges []CausalityEdge `json:"edges"`
	// Truncated reports that the run has more URLs than Limit allowed, so the
	// graph is a prefix rather than the whole run.
	Truncated        bool   `json:"truncated"`
	Limit            int    `json:"limit"`
	VectorsAvailable bool   `json:"vectors_available"`
	Note             string `json:"note,omitempty"`
}

// RunCausality assembles searches → urls (with rank) → scrapes → facts for one
// run. URLs shared by several searches appear once, with an edge per finding
// search. maxURLs bounds the response; a run with more URLs is truncated rather
// than streamed whole.
func (s *Store) RunCausality(ctx context.Context, runID string, maxURLs int) (*CausalityGraph, error) {
	if maxURLs <= 0 {
		maxURLs = 200
	}
	g := &CausalityGraph{RunID: runID, Nodes: []CausalityNode{}, Edges: []CausalityEdge{}, Limit: maxURLs}

	searches, err := s.ListSearches(ctx, runID)
	if err != nil {
		return nil, err
	}
	for _, se := range searches {
		g.Nodes = append(g.Nodes, CausalityNode{
			ID:     "search:" + se.ID,
			Kind:   "search",
			RefID:  se.ID,
			Label:  se.Engine + " · " + se.Term,
			Detail: se.SearchMode,
		})
	}

	// One row per (search, url) so a shared URL keeps an edge from each search.
	// Fetch one past the cap to detect truncation without a second count query.
	rows, err := s.db.QueryContext(ctx,
		`SELECT su.search_id, u.id, u.url, su.rank
		   FROM search_urls su
		   JOIN searches se ON se.id = su.search_id
		   JOIN urls u ON u.id = su.url_id
		  WHERE se.run_id = ?
		  ORDER BY su.rank, u.url
		  LIMIT ?`, runID, maxURLs+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type link struct {
		searchID, urlID, url string
		rank                 int
	}
	var links []link
	for rows.Next() {
		var l link
		if err := rows.Scan(&l.searchID, &l.urlID, &l.url, &l.rank); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(links) > maxURLs {
		links = links[:maxURLs]
		g.Truncated = true
	}

	seenURL := map[string]bool{}
	urlList := make([]string, 0, len(links))
	for _, l := range links {
		if !seenURL[l.urlID] {
			seenURL[l.urlID] = true
			urlList = append(urlList, l.url)
			g.Nodes = append(g.Nodes, CausalityNode{
				ID:    "url:" + l.urlID,
				Kind:  "url",
				RefID: l.urlID,
				Label: l.url,
				URL:   l.url,
			})
		}
		g.Edges = append(g.Edges, CausalityEdge{From: "search:" + l.searchID, To: "url:" + l.urlID, Rank: l.rank})
	}

	if len(urlList) == 0 {
		available, err := s.markVectors(ctx, nil)
		if err != nil {
			return nil, err
		}
		g.VectorsAvailable = available
		if !available {
			g.Note = vectorsUnavailableNote
		}
		return g, nil
	}

	urlByID := map[string]string{} // url -> node id
	for _, l := range links {
		urlByID[l.url] = "url:" + l.urlID
	}

	if err := s.attachScrapes(ctx, g, urlList, urlByID); err != nil {
		return nil, err
	}
	facts, err := s.attachFacts(ctx, g, urlList, urlByID)
	if err != nil {
		return nil, err
	}

	available, err := s.markVectors(ctx, facts)
	if err != nil {
		return nil, err
	}
	g.VectorsAvailable = available
	if !available {
		g.Note = vectorsUnavailableNote
	}
	// markVectors mutates its own slice, so copy the flags onto the nodes.
	if available {
		byID := map[string]bool{}
		for _, f := range facts {
			byID[f.ID] = f.HasVector
		}
		for i := range g.Nodes {
			if g.Nodes[i].Kind == "fact" {
				g.Nodes[i].HasVector = byID[g.Nodes[i].RefID]
			}
		}
	}
	return g, nil
}

// attachScrapes adds a scrape node per scraped URL, edged from its URL node.
func (s *Store) attachScrapes(ctx context.Context, g *CausalityGraph, urls []string, urlNode map[string]string) error {
	args := make([]any, len(urls))
	for i, u := range urls {
		args[i] = u
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, COALESCE(title, ''), COALESCE(http_status, 0)
		   FROM scrape_cache WHERE url IN (`+placeholders(len(args))+`)`, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id, url, title string
		var status int
		if err := rows.Scan(&id, &url, &title, &status); err != nil {
			return err
		}
		label := title
		if label == "" {
			label = url
		}
		g.Nodes = append(g.Nodes, CausalityNode{
			ID:     "scrape:" + id,
			Kind:   "scrape",
			RefID:  id,
			Label:  label,
			Detail: fmt.Sprintf("HTTP %d", status),
			URL:    url,
		})
		g.Edges = append(g.Edges, CausalityEdge{From: urlNode[url], To: "scrape:" + id})
	}
	return rows.Err()
}

// attachFacts adds a fact node per distilled fact, edged from the URL it came
// from. Returns the facts so vector presence can be resolved in one query.
func (s *Store) attachFacts(ctx context.Context, g *CausalityGraph, urls []string, urlNode map[string]string) ([]FactProvenance, error) {
	args := make([]any, len(urls))
	for i, u := range urls {
		args[i] = u
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, text, COALESCE(source_url, ''), COALESCE(volatility, ''), tier, created_at
		   FROM memory_facts WHERE source_url IN (`+placeholders(len(args))+`) ORDER BY created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var facts []FactProvenance
	for rows.Next() {
		var f FactProvenance
		if err := rows.Scan(&f.ID, &f.Text, &f.SourceURL, &f.Volatility, &f.Tier, &f.CreatedAt); err != nil {
			return nil, err
		}
		facts = append(facts, f)
		g.Nodes = append(g.Nodes, CausalityNode{
			ID:     "fact:" + f.ID,
			Kind:   "fact",
			RefID:  f.ID,
			Label:  f.Text,
			Detail: f.Volatility,
			URL:    f.SourceURL,
		})
		g.Edges = append(g.Edges, CausalityEdge{From: urlNode[f.SourceURL], To: "fact:" + f.ID})
	}
	return facts, rows.Err()
}
