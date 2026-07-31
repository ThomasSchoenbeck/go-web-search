package main

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// harvester is the shared engine behind both the CLI and the HTTP server, so
// the two modes cannot drift apart in behaviour.
type harvester struct {
	cfg     Config
	store   *Store
	log     *log.Logger
	session *session
	scraper *scraper

	// searchMu serialises whole searches. Engines within a term run in
	// parallel, but two concurrent API requests each firing three tabs at the
	// same engines would recreate exactly the burst pattern the stagger exists
	// to avoid.
	searchMu sync.Mutex
}

func newHarvester(cfg Config, store *Store, logger *log.Logger, sess *session) *harvester {
	return &harvester{
		cfg:     cfg,
		store:   store,
		log:     logger,
		session: sess,
		scraper: newScraper(cfg.Scrape, cfg.Cache, store, logger, sess),
	}
}

type engineOutcome struct {
	engine string
	rec    SearchRecord
	err    error
}

// SearchTerms runs every term across every configured engine and stores the
// results. Returns false if it stopped early.
func (h *harvester) SearchTerms(ctx context.Context, runID string, terms []string, stop context.Context) (bool, error) {
	h.searchMu.Lock()
	defer h.searchMu.Unlock()

	engines, err := enginesByNames(strings.Join(h.cfg.Search.Engines, ","))
	if err != nil {
		return false, err
	}
	typed := parseEngineSet(strings.Join(h.cfg.Search.Typed, ","))
	counts := h.cfg.Search.Results

	completed := true
	for i, term := range terms {
		if stop.Err() != nil || ctx.Err() != nil {
			completed = false
			break
		}
		h.log.Printf("[%d/%d] term %q across %d engines", i+1, len(terms), term, len(engines))

		outcomes := h.searchOneTerm(ctx, runID, term, engines, typed, counts, stop)

		// Persisting after the fan-in keeps writes ordered and off the hot path
		// of the goroutines doing network work.
		for _, o := range outcomes {
			if o.err != nil {
				h.log.Printf("     ERROR  %s: %v", o.engine, o.err)
			}
			searchID, saveErr := h.store.SaveSearch(ctx, o.rec)
			if saveErr != nil {
				return false, saveErr
			}
			h.log.Printf("     %-11s %d anchors -> %d destinations (search %s)",
				o.engine, o.rec.AnchorCount, len(o.rec.URLs), searchID)
		}

		if i < len(terms)-1 {
			if !waitOrStop(stop, h.cfg.Search.MinDelay.Duration, h.cfg.Search.MaxDelay.Duration) {
				completed = false
				break
			}
		}
	}
	return completed, nil
}

// searchOneTerm fans the engines out into their own tabs with staggered,
// jittered starts.
func (h *harvester) searchOneTerm(ctx context.Context, runID, term string,
	engines []engineDef, typed map[string]bool, counts map[string]int,
	stop context.Context) []engineOutcome {

	outcomes := make([]engineOutcome, len(engines))
	var wg sync.WaitGroup

	for i, e := range engines {
		wg.Add(1)
		go func(idx int, e engineDef) {
			defer wg.Done()

			delay := time.Duration(idx) * h.cfg.Search.EngineStagger.Duration
			if j := h.cfg.Search.EngineJitter.Duration; j > 0 {
				delay += time.Duration(rand.Int63n(int64(j)))
			}
			if !sleepCtx(ctx, delay) {
				outcomes[idx] = engineOutcome{engine: e.name, err: ctx.Err()}
				return
			}
			if stop.Err() != nil {
				outcomes[idx] = engineOutcome{engine: e.name, err: stop.Err()}
				return
			}

			outcomes[idx] = h.queryEngine(ctx, runID, term, e, typed[e.name], counts[e.name])
		}(i, e)
	}

	wg.Wait()
	return outcomes
}

func (h *harvester) queryEngine(ctx context.Context, runID, term string, e engineDef,
	useTyped bool, count int) engineOutcome {

	out := engineOutcome{engine: e.name}
	rec := SearchRecord{RunID: runID, Term: term, Engine: e.name, SearchMode: "url"}
	started := time.Now()

	tabCtx, cancel, err := h.session.newTab()
	if err != nil {
		rec.Err = err.Error()
		rec.Duration = time.Since(started)
		out.rec, out.err = rec, err
		return out
	}
	defer cancel()

	var res *searchResult
	var searchErr error
	if useTyped {
		rec.SearchMode = "typed"
		rec.RequestedURL = e.home
		res, searchErr = h.session.searchByBox(tabCtx, e, term, h.cfg.Search.QueryTimeout.Duration)
	} else {
		rec.RequestedURL = e.searchURL(term, count)
		res, searchErr = h.session.searchByURL(tabCtx, e, term, count, h.cfg.Search.QueryTimeout.Duration)
	}
	rec.Duration = time.Since(started)

	if searchErr != nil {
		rec.Err = searchErr.Error()
		out.rec, out.err = rec, searchErr
		return out
	}

	rec.LandedURL = res.landedURL
	rec.HTTPStatus = res.httpStatus
	rec.AnchorCount = len(res.links)
	rec.RawHTML = res.html
	rec.Blocked = looksBlocked(res.landedURL, res.html)
	// Merge the engine's built-in skip hosts with the operator's global
	// exclude_hosts list, without mutating the shared engine definition.
	skipHosts := append(append([]string{}, e.skipHosts...), h.cfg.Search.ExcludeHosts...)
	rec.URLs = destinations(res.links, e.skipLabels, skipHosts)

	if rec.SearchMode == "typed" {
		if got := landedQuery(res.landedURL); got != "" && !sameQuery(got, term) {
			h.log.Printf("     WARNING: %s searched for %q, not %q - autocomplete probably won", e.name, got, term)
		}
	}
	if rec.Blocked {
		h.log.Printf("     WARNING: %s served a challenge page, results are probably empty", e.name)
	}

	out.rec = rec
	return out
}

// ScrapeRun scrapes every URL a run collected.
func (h *harvester) ScrapeRun(ctx context.Context, runID string) ([]ScrapeOutcome, error) {
	rows, err := h.store.RunURLs(ctx, runID)
	if err != nil {
		return nil, err
	}
	urls := make([]string, 0, len(rows))
	for _, r := range rows {
		urls = append(urls, r.URL)
	}
	return h.scraper.Scrape(ctx, runID, urls, true, 0), nil
}

// ScrapeURLs scrapes an explicit list.
func (h *harvester) ScrapeURLs(ctx context.Context, runID string, urls []string) []ScrapeOutcome {
	return h.scraper.Scrape(ctx, runID, urls, true, 0)
}
