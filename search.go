package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	cdpdom "github.com/chromedp/cdproto/dom"
	"github.com/chromedp/cdproto/input"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type searchResult struct {
	landedURL  string
	html       string
	links      []string
	httpStatus int
}

// searchByBox loads the engine's homepage, puts the term into its search field
// the way a paste would, and submits the form the way Enter would. The engine
// then builds its own result URL, which carries whatever session and tracking
// parameters it expects to see - none of which we could have guessed.
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

	typeInto := func() error {
		return chromedp.Run(ctx,
			chromedp.MouseClickNode(node),
			pause(200*time.Millisecond, 600*time.Millisecond),
			// Paste the term, the way a user's Ctrl+V would: focus the box via
			// the DOM (a mouse click can be intercepted by whatever sits over
			// the box) and insert the text with one Input.insertText call.
			// That bypasses the key-event pipeline entirely - on the broken
			// setups seen so far the individual key events never reached the
			// page at all, while the rest of CDP (navigation, DOM) worked.
			chromedp.ActionFunc(func(ctx context.Context) error {
				if err := cdpdom.Focus().WithNodeID(node.NodeID).Do(ctx); err != nil {
					return err
				}
				return input.InsertText(term).Do(ctx)
			}),
			// Let the page's input handlers settle before the form is
			// submitted, so the box holds the term and not a half-processed
			// suggestion.
			pause(500*time.Millisecond, 1200*time.Millisecond),
		)
	}
	if err := typeInto(); err != nil {
		return nil, fmt.Errorf("entering search term: %w", err)
	}

	// Read the box back before pressing Enter. An empty box means the click
	// missed the field (an overlay, the page still hydrating) and Enter would
	// do nothing - the "no navigation" and "no results" warnings that follow
	// are that wasted 40 s, so retry once and then fail fast.
	var typed string
	if err := chromedp.Run(ctx, chromedp.Value(selector, &typed)); err != nil {
		s.log.Printf("     WARNING: could not read the search box back (%v), proceeding", err)
	} else if strings.TrimSpace(typed) == "" {
		s.log.Printf("     search box is empty after typing, retrying once")
		if err := typeInto(); err != nil {
			return nil, fmt.Errorf("entering search term: %w", err)
		}
		if err := chromedp.Run(ctx, chromedp.Value(selector, &typed)); err == nil && strings.TrimSpace(typed) == "" {
			return nil, fmt.Errorf("search box on %s did not accept the term", e.home)
		}
	}

	// Submit the form the way Enter would, without a key event. requestSubmit
	// fires the same submit event the browser fires when Enter is pressed in
	// the box - the page's own handler runs and the engine builds its own
	// result URL - while needing nothing but a JS context, which works here:
	// the box read-back above used one. Key events are exactly what the broken
	// setups drop, which is why the term gets in as a paste (see typeInto).
	//
	// The submit triggers a navigation. Waiting for that navigation to finish
	// before polling is the whole point: polling immediately binds to the
	// homepage's JavaScript execution context, which is destroyed the moment
	// the results page loads, producing "Cannot find context with specified
	// id".
	submitJS := fmt.Sprintf(`document.querySelector(%s).form.requestSubmit()`, strconv.Quote(selector))
	status, err := s.awaitNavigation(ctx, chromedp.Evaluate(submitJS, nil))
	if err != nil {
		// Log where the page actually is: still home means the submit never
		// navigated; any other URL (consent, challenge, interstitial) means a
		// different page was served and the selectors need to know about it.
		landed := "(unknown)"
		_ = chromedp.Run(ctx, chromedp.Location(&landed))
		s.log.Printf("     WARNING: no navigation seen after submit (%v); page is at %s, box holds %q - falling back to a plain settle", err, landed, typed)
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

// findSearchBox returns the first node matching one of the engine's
// search-box selectors, retrying until a node appears or the deadline passes.
// A single query can land while the page is still hydrating, or between
// documents during a redirect, when the DOM query matches nothing or its
// execution context is already dead - a one-shot attempt then reports no
// search box even though the box is about to render. Each pass re-attaches to
// the current document, so a slow page or a mid-load navigation recovers.
// If the box never appears, the error reports where the page actually landed:
// that is what distinguishes a variant or challenge page from a race.
func findSearchBox(ctx context.Context, e engineDef) (*cdp.Node, string, error) {
	deadline := time.Now().Add(10 * time.Second)
	for {
		for _, sel := range e.searchBox {
			var nodes []*cdp.Node
			if err := chromedp.Run(ctx, chromedp.Nodes(sel, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0))); err != nil {
				continue
			}
			if len(nodes) > 0 {
				return nodes[0], sel, nil
			}
		}
		if time.Now().After(deadline) {
			landed := ""
			if err := chromedp.Run(ctx, chromedp.Location(&landed)); err == nil && landed != e.home {
				return nil, "", fmt.Errorf("no search box found: page landed on %s instead of %s (tried %s)", landed, e.home, strings.Join(e.searchBox, ", "))
			}
			return nil, "", fmt.Errorf("no search box found on %s (tried %s)", e.home, strings.Join(e.searchBox, ", "))
		}
		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
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
