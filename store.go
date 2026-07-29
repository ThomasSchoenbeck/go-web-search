package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	// Registers the Turso driver with database/sql. The bindings moved from
	// github.com/tursodatabase/turso-go to this path; the driver name it
	// registers is set in config so a rename does not require a rebuild.
	_ "turso.tech/database/tursogo"
)

//go:embed schema.sql
var mainSchema string

// Store is the main database. Writes are serialised: the Turso Go bindings are
// beta and this is a local file database, so a single connection removes a
// whole class of concurrency bugs that would otherwise surface once the
// scraper starts writing from many goroutines. Revisit if it ever becomes the
// bottleneck, which for a tool bounded by network latency is unlikely.
type Store struct {
	db *sql.DB
}

func openStore(driver, path string, maxConns int) (*Store, error) {
	db, err := sql.Open(driver, path)
	if err != nil {
		return nil, fmt.Errorf("opening %s with driver %q: %w", path, driver, err)
	}
	if maxConns <= 0 {
		maxConns = 1
	}
	db.SetMaxOpenConns(maxConns)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to %s: %w", path, err)
	}
	if _, err := db.Exec(mainSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying schema to %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// newID returns a time-ordered UUIDv7, falling back to v4 only if the system
// clock source fails.
func newID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	return uuid.NewString()
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// StartRun records a run and returns its id.
func (s *Store) StartRun(ctx context.Context, mode, artifactDir string) (string, error) {
	id := newID()
	now := nowRFC3339()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO runs (id, mode, artifact_dir, started_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, mode, artifactDir, now, now)
	if err != nil {
		return "", fmt.Errorf("starting run: %w", err)
	}
	return id, nil
}

func (s *Store) FinishRun(ctx context.Context, runID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE runs SET finished_at = ? WHERE id = ?`, nowRFC3339(), runID)
	return err
}

// SearchRecord is everything known about one engine query.
type SearchRecord struct {
	RunID        string
	Term         string
	Engine       string
	SearchMode   string
	RequestedURL string
	LandedURL    string
	HTTPStatus   int
	Blocked      bool
	AnchorCount  int
	Err          string
	Duration     time.Duration
	RawHTML      string
	URLs         []string
}

// SaveSearch writes one query, its raw SERP, and its extracted URLs in a single
// transaction, so a crash mid-write can never leave URLs attached to a search
// that does not exist.
func (s *Store) SaveSearch(ctx context.Context, rec SearchRecord) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	now := nowRFC3339()
	searchID := newID()

	blocked := 0
	if rec.Blocked {
		blocked = 1
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO searches (id, run_id, term, engine, search_mode, requested_url,
		    landed_url, http_status, blocked, anchor_count, error, duration_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		searchID, rec.RunID, rec.Term, rec.Engine, rec.SearchMode, rec.RequestedURL,
		rec.LandedURL, rec.HTTPStatus, blocked, rec.AnchorCount, rec.Err,
		rec.Duration.Milliseconds(), now)
	if err != nil {
		return "", fmt.Errorf("saving search: %w", err)
	}

	if rec.RawHTML != "" {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO search_raw (id, search_id, html, byte_size, created_at) VALUES (?, ?, ?, ?, ?)`,
			newID(), searchID, rec.RawHTML, len(rec.RawHTML), now); err != nil {
			return "", fmt.Errorf("saving raw html: %w", err)
		}
	}

	for rank, rawURL := range rec.URLs {
		urlID, err := upsertURL(ctx, tx, rawURL, now)
		if err != nil {
			return "", err
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO search_urls (id, search_id, url_id, rank, created_at)
			 VALUES (?, ?, ?, ?, ?)`,
			newID(), searchID, urlID, rank+1, now); err != nil {
			return "", fmt.Errorf("saving search url: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return searchID, nil
}

// upsertURL returns the id of a URL, inserting it the first time it is seen.
func upsertURL(ctx context.Context, tx *sql.Tx, rawURL, now string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `SELECT id FROM urls WHERE url = ?`, rawURL).Scan(&id)
	switch {
	case err == nil:
		return id, nil
	case err != sql.ErrNoRows:
		return "", fmt.Errorf("looking up url: %w", err)
	}

	id = newID()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO urls (id, url, domain, first_seen_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, rawURL, domainOf(rawURL), now, now); err != nil {
		return "", fmt.Errorf("inserting url: %w", err)
	}
	return id, nil
}

// DistinctURLs returns every unique URL found by a run, which is the handover
// list the scraper will consume.
func (s *Store) DistinctURLs(ctx context.Context, runID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT u.url
		   FROM urls u
		   JOIN search_urls su ON su.url_id = u.id
		   JOIN searches s ON s.id = su.search_id
		  WHERE s.run_id = ?
		  ORDER BY u.url`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func domainOf(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}

// ScrapeRecord is one fetch attempt.
type ScrapeRecord struct {
	URL           string
	RunID         string
	HTTPStatus    int
	ContentType   string
	FetchedWith   string
	RobotsAllowed bool
	Title         string
	RawHTML       string
	CleanHTML     string
	Text          string
	Err           string
	Duration      time.Duration
	Images        []imageRef
}

// SaveScrape writes a scrape and its images in one transaction, registering the
// URL if it was never seen by a search.
func (s *Store) SaveScrape(ctx context.Context, rec ScrapeRecord) (string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	now := nowRFC3339()
	urlID, err := upsertURL(ctx, tx, rec.URL, now)
	if err != nil {
		return "", err
	}

	allowed := 0
	if rec.RobotsAllowed {
		allowed = 1
	}
	scrapeID := newID()
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO scrapes (id, url_id, run_id, http_status, content_type, fetched_with,
		     robots_allowed, title, raw_html, clean_html, text_content, error, duration_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		scrapeID, urlID, nullable(rec.RunID), rec.HTTPStatus, rec.ContentType, rec.FetchedWith,
		allowed, rec.Title, rec.RawHTML, rec.CleanHTML, rec.Text, rec.Err,
		rec.Duration.Milliseconds(), now); err != nil {
		return "", fmt.Errorf("saving scrape: %w", err)
	}

	for _, img := range rec.Images {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO scrape_images (id, scrape_id, url, alt, width, height, created_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			newID(), scrapeID, img.URL, img.Alt, img.Width, img.Height, now); err != nil {
			return "", fmt.Errorf("saving image: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}
	return scrapeID, nil
}

// ---- read side, backing the by-id endpoints ----

type RunSummary struct {
	ID          string `json:"id"`
	Mode        string `json:"mode"`
	ArtifactDir string `json:"artifact_dir,omitempty"`
	StartedAt   string `json:"started_at"`
	FinishedAt  string `json:"finished_at,omitempty"`
	Searches    int    `json:"searches"`
	URLs        int    `json:"urls"`
	Scrapes     int    `json:"scrapes"`
}

func (s *Store) GetRun(ctx context.Context, runID string) (*RunSummary, error) {
	var r RunSummary
	var artifact, finished sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, mode, artifact_dir, started_at, finished_at FROM runs WHERE id = ?`, runID).
		Scan(&r.ID, &r.Mode, &artifact, &r.StartedAt, &finished)
	if err != nil {
		return nil, err
	}
	r.ArtifactDir = artifact.String
	r.FinishedAt = finished.String

	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM searches WHERE run_id = ?`, runID).Scan(&r.Searches)
	s.db.QueryRowContext(ctx,
		`SELECT COUNT(DISTINCT su.url_id) FROM search_urls su
		   JOIN searches se ON se.id = su.search_id WHERE se.run_id = ?`, runID).Scan(&r.URLs)
	s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM scrapes WHERE run_id = ?`, runID).Scan(&r.Scrapes)
	return &r, nil
}

func (s *Store) ListRuns(ctx context.Context, limit int) ([]RunSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT r.id, r.mode, r.started_at, COALESCE(r.finished_at, ''),
		        (SELECT COUNT(*) FROM searches WHERE run_id = r.id),
		        (SELECT COUNT(DISTINCT su.url_id) FROM search_urls su
		           JOIN searches se ON se.id = su.search_id WHERE se.run_id = r.id),
		        (SELECT COUNT(*) FROM scrapes WHERE run_id = r.id)
		  FROM runs r
		  ORDER BY r.started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RunSummary
	for rows.Next() {
		var r RunSummary
		if err := rows.Scan(&r.ID, &r.Mode, &r.StartedAt, &r.FinishedAt,
			&r.Searches, &r.URLs, &r.Scrapes); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

type SearchSummary struct {
	ID          string `json:"id"`
	RunID       string `json:"run_id"`
	Term        string `json:"term"`
	Engine      string `json:"engine"`
	SearchMode  string `json:"search_mode"`
	LandedURL   string `json:"landed_url,omitempty"`
	HTTPStatus  int    `json:"http_status,omitempty"`
	Blocked     bool   `json:"blocked"`
	AnchorCount int    `json:"anchor_count"`
	Error       string `json:"error,omitempty"`
	DurationMS  int64  `json:"duration_ms"`
	CreatedAt   string `json:"created_at"`
}

func (s *Store) ListSearches(ctx context.Context, runID string) ([]SearchSummary, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, run_id, term, engine, search_mode, COALESCE(landed_url,''),
		        COALESCE(http_status,0), blocked, anchor_count, COALESCE(error,''),
		        COALESCE(duration_ms,0), created_at
		   FROM searches WHERE run_id = ? ORDER BY created_at`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SearchSummary
	for rows.Next() {
		var v SearchSummary
		var blocked int
		if err := rows.Scan(&v.ID, &v.RunID, &v.Term, &v.Engine, &v.SearchMode, &v.LandedURL,
			&v.HTTPStatus, &blocked, &v.AnchorCount, &v.Error, &v.DurationMS, &v.CreatedAt); err != nil {
			return nil, err
		}
		v.Blocked = blocked == 1
		out = append(out, v)
	}
	return out, rows.Err()
}

// SearchRawHTML returns the stored SERP for a search.
func (s *Store) SearchRawHTML(ctx context.Context, searchID string) (string, error) {
	var body string
	err := s.db.QueryRowContext(ctx, `SELECT html FROM search_raw WHERE search_id = ?`, searchID).Scan(&body)
	return body, err
}

type URLRow struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Domain string `json:"domain"`
	Rank   int    `json:"rank,omitempty"`
	Engine string `json:"engine,omitempty"`
}

// RunURLs returns every distinct URL a run found, best (lowest) rank first.
func (s *Store) RunURLs(ctx context.Context, runID string) ([]URLRow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT u.id, u.url, u.domain, MIN(su.rank) AS best_rank
		   FROM urls u
		   JOIN search_urls su ON su.url_id = u.id
		   JOIN searches se ON se.id = su.search_id
		  WHERE se.run_id = ?
		  GROUP BY u.id, u.url, u.domain
		  ORDER BY best_rank, u.url`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []URLRow
	for rows.Next() {
		var v URLRow
		if err := rows.Scan(&v.ID, &v.URL, &v.Domain, &v.Rank); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type ScrapeDetail struct {
	ID            string     `json:"id"`
	URL           string     `json:"url"`
	RunID         string     `json:"run_id,omitempty"`
	HTTPStatus    int        `json:"http_status,omitempty"`
	ContentType   string     `json:"content_type,omitempty"`
	FetchedWith   string     `json:"fetched_with,omitempty"`
	RobotsAllowed bool       `json:"robots_allowed"`
	Title         string     `json:"title,omitempty"`
	CleanHTML     string     `json:"clean_html,omitempty"`
	Text          string     `json:"text,omitempty"`
	RawHTML       string     `json:"raw_html,omitempty"`
	Error         string     `json:"error,omitempty"`
	DurationMS    int64      `json:"duration_ms"`
	CreatedAt     string     `json:"created_at"`
	Images        []ImageRow `json:"images,omitempty"`
}

type ImageRow struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Alt    string `json:"alt,omitempty"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// GetScrape returns one scrape. includeRaw is opt-in because the raw HTML is
// usually far larger than everything else combined.
func (s *Store) GetScrape(ctx context.Context, scrapeID string, includeRaw bool) (*ScrapeDetail, error) {
	var d ScrapeDetail
	var robots int
	var runID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT s.id, u.url, s.run_id, COALESCE(s.http_status,0), COALESCE(s.content_type,''),
		        COALESCE(s.fetched_with,''), s.robots_allowed, COALESCE(s.title,''),
		        COALESCE(s.clean_html,''), COALESCE(s.text_content,''), COALESCE(s.raw_html,''),
		        COALESCE(s.error,''), COALESCE(s.duration_ms,0), s.created_at
		   FROM scrapes s JOIN urls u ON u.id = s.url_id
		  WHERE s.id = ?`, scrapeID).
		Scan(&d.ID, &d.URL, &runID, &d.HTTPStatus, &d.ContentType, &d.FetchedWith, &robots,
			&d.Title, &d.CleanHTML, &d.Text, &d.RawHTML, &d.Error, &d.DurationMS, &d.CreatedAt)
	if err != nil {
		return nil, err
	}
	d.RunID = runID.String
	d.RobotsAllowed = robots == 1
	if !includeRaw {
		d.RawHTML = ""
	}

	rows, err := s.db.QueryContext(ctx,
		`SELECT id, url, COALESCE(alt,''), COALESCE(width,0), COALESCE(height,0)
		   FROM scrape_images WHERE scrape_id = ? ORDER BY created_at`, scrapeID)
	if err != nil {
		return &d, nil
	}
	defer rows.Close()
	for rows.Next() {
		var img ImageRow
		if err := rows.Scan(&img.ID, &img.URL, &img.Alt, &img.Width, &img.Height); err != nil {
			break
		}
		d.Images = append(d.Images, img)
	}
	return &d, nil
}

// RunScrapeIDs lists the scrapes produced by a run.
func (s *Store) RunScrapeIDs(ctx context.Context, runID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id FROM scrapes WHERE run_id = ? ORDER BY created_at`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}
