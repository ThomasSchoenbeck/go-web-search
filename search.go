package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/chromedp/chromedp/kb"
)

type searchResult struct {
	landedURL  string
	html       string
	links      []string
	httpStatus int
}

// searchByBox loads the engine's homepage, puts the term into its search field
// the way a paste would, and presses Enter. The engine then builds its own
// result URL, which carries whatever session and tracking parameters it expects
// to see - none of which we could have guessed.
func (s *session) searchByBox(parent context.Context, e engineDef, term string, timeout time.Duration) (*searchResult, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	if _, err := s.awaitNavigation(ctx, chromedp.Navigate(e.home)); err != nil {
		return nil, fmt.Errorf("loading %s: %w", e.home, err)
	}
	if err := chromedp.Run(ctx,
		chromedp.WaitReady("body", chromedp.ByQuery),
		pause(700*time.Millisecond, 1800*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error { return s.dismissConsent(ctx, e) }),
	); err != nil {
		return nil, fmt.Errorf("preparing %s: %w", e.home, err)
	}

	node, selector, err := findSearchBox(ctx, e)
	if err != nil {
		return nil, err
	}
	s.log.Printf("     typing into %s", selector)

	if err := chromedp.Run(ctx,
		chromedp.MouseClickNode(node),
		pause(200*time.Millisecond, 600*time.Millisecond),
		// Type character by character so the engine's box fires the
		// keydown/keypress/input events its JavaScript listens for. A single
		// Input.insertText (paste-style) leaves that state un-updated on modern
		// JS-controlled boxes, so the later Enter submits nothing. This is what a
		// human does by hand in browse mode, where Enter works.
		chromedp.KeyEvent(term),
		// Let the suggestion dropdown settle before Enter, otherwise it can
		// swallow the key and submit a suggestion instead of the term.
		pause(500*time.Millisecond, 1200*time.Millisecond),
	); err != nil {
		return nil, fmt.Errorf("entering search term: %w", err)
	}

	// Enter triggers a navigation. Waiting for that navigation to finish before
	// polling is the whole point: polling immediately binds to the homepage's
	// JavaScript execution context, which is destroyed the moment the results
	// page loads, producing "Cannot find context with specified id".
	status, err := s.awaitNavigation(ctx, chromedp.KeyEvent(kb.Enter))
	if err != nil {
		s.log.Printf("     WARNING: no navigation seen after Enter (%v), falling back to a plain settle", err)
		if err := chromedp.Run(ctx, chromedp.WaitReady("body", chromedp.ByQuery)); err != nil {
			return nil, fmt.Errorf("submitting search box: %w", err)
		}
	}

	if err := waitForResults(ctx, e); err != nil {
		s.log.Printf("     WARNING: result container never appeared (%v), collecting whatever rendered", err)
	}
	res, err := s.collect(ctx, e)
	if err != nil {
		return nil, err
	}
	res.httpStatus = status
	return res, nil
}

// searchByURL navigates straight to the results URL.
func (s *session) searchByURL(parent context.Context, e engineDef, term string, results int, timeout time.Duration) (*searchResult, error) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	target := e.searchURL(term, results)
	s.log.Printf("     visiting %s", target)

	status, err := s.awaitNavigation(ctx, chromedp.Navigate(target))
	if err != nil {
		return nil, err
	}
	if err := chromedp.Run(ctx,
		chromedp.WaitReady("body", chromedp.ByQuery),
		pause(600*time.Millisecond, 1600*time.Millisecond),
		chromedp.ActionFunc(func(ctx context.Context) error { return s.dismissConsent(ctx, e) }),
	); err != nil {
		return nil, err
	}
	if err := waitForResults(ctx, e); err != nil {
		s.log.Printf("     WARNING: result container never appeared (%v), collecting whatever rendered", err)
	}
	res, err := s.collect(ctx, e)
	if err != nil {
		return nil, err
	}
	res.httpStatus = status
	return res, nil
}

func (s *session) collect(ctx context.Context, e engineDef) (*searchResult, error) {
	res := &searchResult{}
	err := chromedp.Run(ctx,
		chromedp.ActionFunc(humanScroll),
		chromedp.Location(&res.landedURL),
		chromedp.OuterHTML("html", &res.html, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(collectLinksJS(e.resultRoots), &res.links).Do(ctx)
		}),
	)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func findSearchBox(ctx context.Context, e engineDef) (*cdp.Node, string, error) {
	for _, sel := range e.searchBox {
		var nodes []*cdp.Node
		if err := chromedp.Run(ctx, chromedp.Nodes(sel, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0))); err != nil {
			continue
		}
		if len(nodes) > 0 {
			return nodes[0], sel, nil
		}
	}
	return nil, "", fmt.Errorf("no search box found on %s (tried %s)", e.home, strings.Join(e.searchBox, ", "))
}

// waitForResults polls until one of the result containers exists.
func waitForResults(ctx context.Context, e engineDef) error {
	js := fmt.Sprintf(`%s.some((s) => !!document.querySelector(s))`, jsArray(e.resultRoots))
	var ready bool
	return chromedp.Run(ctx, chromedp.Poll(js, &ready,
		chromedp.WithPollingInterval(250*time.Millisecond),
		chromedp.WithPollingTimeout(20*time.Second),
	))
}

// dismissConsent clicks the first consent button that is present. Failures are
// ignored: on most loads there is no dialog at all.
func (s *session) dismissConsent(ctx context.Context, e engineDef) error {
	for _, sel := range e.consentSelectors {
		var nodes []*cdp.Node
		if err := chromedp.Nodes(sel, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0)).Do(ctx); err != nil {
			continue
		}
		if len(nodes) == 0 {
			continue
		}
		if err := chromedp.MouseClickNode(nodes[0]).Do(ctx); err != nil {
			continue
		}
		s.log.Printf("     dismissed consent dialog via %s", sel)
		return pause(800*time.Millisecond, 1500*time.Millisecond).Do(ctx)
	}
	return nil
}

// navigationTimeout bounds the wait for a page load. RunResponse blocks until
// its context is cancelled when the action turns out not to navigate at all, so
// this must stay well under the per query timeout to leave room for a fallback.
const navigationTimeout = 20 * time.Second

// awaitNavigation runs an action that loads a page and reports the HTTP status
// of the resulting document - the response detail that a plain Navigate throws
// away, and the only place a 429 would ever be visible.
func (s *session) awaitNavigation(ctx context.Context, action chromedp.Action) (int, error) {
	navCtx, cancel := context.WithTimeout(ctx, navigationTimeout)
	defer cancel()

	resp, err := chromedp.RunResponse(navCtx, action)
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, nil
	}

	s.log.Printf("     HTTP %d via %s", resp.Status, resp.Protocol)
	switch {
	case resp.Status == 429:
		s.log.Printf("     WARNING: HTTP 429, this one really is rate limiting%s", retryAfter(resp.Headers))
	case resp.Status >= 400:
		s.log.Printf("     WARNING: HTTP %d %s", resp.Status, resp.StatusText)
	}
	return int(resp.Status), nil
}

func retryAfter(headers network.Headers) string {
	for name, value := range headers {
		if strings.EqualFold(name, "retry-after") {
			return fmt.Sprintf(" (Retry-After: %v)", value)
		}
	}
	return ""
}
