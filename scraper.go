package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// ScrapeOutcome is what the caller (CLI or API) gets back per URL.
type ScrapeOutcome struct {
	ScrapeID    string `json:"scrape_id,omitempty"`
	URL         string `json:"url"`
	Status      int    `json:"http_status,omitempty"`
	FetchedWith string `json:"fetched_with,omitempty"`
	Title       string `json:"title,omitempty"`
	TextChars   int    `json:"text_chars"`
	Images      int    `json:"images"`
	Skipped     string `json:"skipped,omitempty"`
	Error       string `json:"error,omitempty"`
}

type scraper struct {
	cfg     ScrapeConfig
	store   *Store
	log     *log.Logger
	client  *http.Client
	robots  *robotsCache
	session *session // optional; without it there is no browser fallback

	// browserSlots bounds concurrent fallback tabs. Each tab is a Chrome
	// renderer process, so this is a memory ceiling rather than a politeness
	// one - unbounded would let a page of JS-heavy results open a tab each.
	browserSlots chan struct{}
}

func newScraper(cfg ScrapeConfig, store *Store, logger *log.Logger, sess *session) *scraper {
	client := &http.Client{
		Timeout: cfg.HTTPTimeout.Duration,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	slots := cfg.MaxBrowserTabs
	if slots <= 0 {
		slots = 1
	}
	return &scraper{
		cfg:          cfg,
		store:        store,
		log:          logger,
		client:       client,
		robots:       newRobotsCache(cfg.RobotsUserAgent, client),
		session:      sess,
		browserSlots: make(chan struct{}, slots),
	}
}

// Scrape fetches every URL, grouping by host: hosts run in parallel, URLs
// within a host run one after another with a delay. That is the shape that
// keeps a crawl fast without hammering any single server, which is the whole
// point of grouping by domain rather than just firing N workers at a queue.
func (s *scraper) Scrape(ctx context.Context, runID string, urls []string) []ScrapeOutcome {
	if len(urls) > s.cfg.MaxURLs && s.cfg.MaxURLs > 0 {
		s.log.Printf("scrape: capping %d urls at max_urls=%d", len(urls), s.cfg.MaxURLs)
		urls = urls[:s.cfg.MaxURLs]
	}
	if len(urls) == 0 {
		return nil
	}

	if s.cfg.TotalTimeout.Duration > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.cfg.TotalTimeout.Duration)
		defer cancel()
	}

	groups := groupByHost(urls)
	s.log.Printf("scrape: %d urls across %d domains", len(urls), len(groups))

	maxDomains := s.cfg.MaxDomains
	if maxDomains <= 0 {
		maxDomains = 4
	}
	sem := make(chan struct{}, maxDomains)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []ScrapeOutcome
	)

	for host, hostURLs := range groups {
		wg.Add(1)
		go func(host string, list []string) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}

			for i, raw := range list {
				if ctx.Err() != nil {
					return
				}
				if i > 0 {
					if !sleepCtx(ctx, s.cfg.PerDomainDelay.Duration) {
						return
					}
				}
				outcome := s.one(ctx, runID, raw)
				mu.Lock()
				results = append(results, outcome)
				mu.Unlock()
			}
		}(host, hostURLs)
	}

	wg.Wait()
	sort.Slice(results, func(i, j int) bool { return results[i].URL < results[j].URL })
	return results
}

func (s *scraper) one(ctx context.Context, runID, raw string) ScrapeOutcome {
	out := ScrapeOutcome{URL: raw}
	started := time.Now()

	target, err := url.Parse(raw)
	if err != nil {
		out.Error = "unparseable url"
		return out
	}

	allowed, crawlDelay := s.robots.Allowed(ctx, target)
	if !allowed {
		out.Skipped = "disallowed by robots.txt"
		s.log.Printf("scrape: %s skipped, robots.txt disallows it", raw)
		id, err := s.store.SaveScrape(ctx, ScrapeRecord{
			URL: raw, RunID: runID, RobotsAllowed: false,
			Duration: time.Since(started),
		})
		if err != nil {
			out.Error = err.Error()
		}
		out.ScrapeID = id
		return out
	}
	if s.cfg.RespectCrawlDelay && crawlDelay > 0 {
		if !sleepCtx(ctx, crawlDelay) {
			out.Error = "cancelled while honouring crawl-delay"
			return out
		}
	}

	body, status, contentType, err := s.fetchHTTP(ctx, raw)
	fetchedWith := "http"

	if err == nil && s.shouldRetryInBrowser(body, contentType) {
		if browserBody, berr := s.fetchBrowser(ctx, raw); berr == nil {
			body, fetchedWith = browserBody, "browser"
			s.log.Printf("scrape: %s re-fetched in browser (thin HTTP response)", raw)
		} else {
			s.log.Printf("scrape: %s browser fallback failed (%v), keeping HTTP body", raw, berr)
		}
	}

	rec := ScrapeRecord{
		URL:           raw,
		RunID:         runID,
		HTTPStatus:    status,
		ContentType:   contentType,
		FetchedWith:   fetchedWith,
		RobotsAllowed: true,
		RawHTML:       body,
		Duration:      time.Since(started),
	}
	if err != nil {
		rec.Err = err.Error()
		out.Error = err.Error()
	} else if isHTML(contentType) {
		base := target
		if doc, cerr := cleanHTML(body, func(ref string) string { return resolveRef(base, ref) }); cerr == nil {
			rec.Title = doc.Title
			rec.CleanHTML = doc.CleanHTML
			rec.Text = doc.Text
			rec.Images = doc.Images
			out.Title = doc.Title
			out.TextChars = len(doc.Text)
			out.Images = len(doc.Images)
		} else {
			rec.Err = "clean: " + cerr.Error()
			out.Error = rec.Err
		}
	} else {
		out.Skipped = "not html (" + contentType + ")"
	}

	id, saveErr := s.store.SaveScrape(ctx, rec)
	if saveErr != nil && out.Error == "" {
		out.Error = saveErr.Error()
	}
	out.ScrapeID = id
	out.Status = status
	out.FetchedWith = fetchedWith
	return out
}

func (s *scraper) fetchHTTP(ctx context.Context, raw string) (body string, status int, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return "", 0, "", err
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", 0, "", err
	}
	defer resp.Body.Close()

	limit := s.cfg.MaxBodyBytes
	if limit <= 0 {
		limit = 5 << 20
	}
	raw3, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return "", resp.StatusCode, resp.Header.Get("Content-Type"), err
	}
	return string(raw3), resp.StatusCode, resp.Header.Get("Content-Type"), nil
}

// shouldRetryInBrowser is the JS-rendered heuristic: plain HTTP got HTML back,
// but almost no text came out of it. That is what a client-rendered app looks
// like from the outside - an empty mount point and a script tag.
func (s *scraper) shouldRetryInBrowser(body, contentType string) bool {
	if !s.cfg.BrowserFallback || s.session == nil || !isHTML(contentType) {
		return false
	}
	doc, err := cleanHTML(body, func(string) string { return "" })
	if err != nil {
		return true
	}
	if len(doc.Text) < s.cfg.MinTextChars {
		return true
	}
	// An empty framework mount point is conclusive even when there is some
	// surrounding boilerplate text.
	for _, marker := range []string{`id="root"></div>`, `id="__next"></div>`, `id="app"></div>`} {
		if strings.Contains(body, marker) {
			return true
		}
	}
	return false
}

func (s *scraper) fetchBrowser(ctx context.Context, raw string) (string, error) {
	select {
	case s.browserSlots <- struct{}{}:
		defer func() { <-s.browserSlots }()
	case <-ctx.Done():
		return "", ctx.Err()
	}

	tabCtx, cancel, err := s.session.newTab()
	if err != nil {
		return "", err
	}
	defer cancel()

	runCtx, runCancel := context.WithTimeout(tabCtx, s.cfg.HTTPTimeout.Duration+15*time.Second)
	defer runCancel()

	var body string
	err = chromedp.Run(runCtx,
		chromedp.Navigate(raw),
		chromedp.WaitReady("body", chromedp.ByQuery),
		pause(600*time.Millisecond, 1400*time.Millisecond),
		chromedp.OuterHTML("html", &body, chromedp.ByQuery),
	)
	if err != nil {
		return "", err
	}
	return body, nil
}

func groupByHost(urls []string) map[string][]string {
	groups := make(map[string][]string)
	for _, raw := range urls {
		u, err := url.Parse(raw)
		if err != nil {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host == "" {
			continue
		}
		groups[host] = append(groups[host], raw)
	}
	return groups
}

func resolveRef(base *url.URL, ref string) string {
	parsed, err := url.Parse(strings.TrimSpace(ref))
	if err != nil {
		return ""
	}
	resolved := base.ResolveReference(parsed)
	if resolved.Scheme != "http" && resolved.Scheme != "https" {
		return ""
	}
	return resolved.String()
}

func isHTML(contentType string) bool {
	ct := strings.ToLower(contentType)
	return ct == "" || strings.Contains(ct, "text/html") || strings.Contains(ct, "xhtml")
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
