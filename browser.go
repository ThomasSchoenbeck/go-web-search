package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	cdpbrowser "github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/emulation"
	"github.com/chromedp/cdproto/page"
	cdpruntime "github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

// stealthJS is deliberately minimal. Every extra patch is another thing that
// can look wrong under inspection: a faked navigator.plugins array is a worse
// fingerprint than the real (empty-ish) one, and wrapping permissions.query
// breaks its native toString(). Removing the webdriver flag is the one patch
// that is strictly a subtraction.
const stealthJS = `Object.defineProperty(navigator, 'webdriver', { get: () => undefined });`

type sessionOpts struct {
	headless    bool
	noSandbox   bool
	userDataDir string
	profileName string
	normalizeUA bool
	log         *log.Logger
}

// chromedpErrorf redirects chromedp's own error output into the run log, which
// otherwise goes to stderr and never reaches workflow_results.log.
//
// "unhandled node event" and "unhandled page event" are dropped. chromedp
// enables the DOM domain as soon as a query action runs, and every modern site
// then emits protocol events chromedp has no handler for -
// DOM.adoptedStyleSheetsModified fires on any page using constructed
// stylesheets. Nothing is wrong and nothing is lost; it is pure noise.
func chromedpErrorf(logger *log.Logger) func(string, ...any) {
	return func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		if strings.Contains(msg, "unhandled node event") || strings.Contains(msg, "unhandled page event") {
			return
		}
		logger.Printf("chromedp: %s", msg)
	}
}

// session is one long-lived Chrome instance. Work happens in tabs created by
// newTab, so several engines can run at once.
type session struct {
	ctx     context.Context
	cancels []context.CancelFunc
	log     *log.Logger

	// uaOverride is retained because Emulation.setUserAgentOverride applies per
	// target, not per browser: every new tab has to be told again or it leaks
	// HeadlessChrome while the first tab looks clean.
	mu         sync.Mutex
	uaOverride *emulation.SetUserAgentOverrideParams
}

// newTab opens an isolated tab with the stealth script and any user agent
// override reapplied. Both are per-target settings.
func (s *session) newTab() (context.Context, context.CancelFunc, error) {
	tabCtx, cancel := chromedp.NewContext(s.ctx)
	if err := chromedp.Run(tabCtx); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("opening tab: %w", err)
	}

	s.mu.Lock()
	override := s.uaOverride
	s.mu.Unlock()

	err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		if _, err := page.AddScriptToEvaluateOnNewDocument(stealthJS).Do(ctx); err != nil {
			return err
		}
		if override != nil {
			return override.Do(ctx)
		}
		return nil
	}))
	if err != nil {
		cancel()
		return nil, nil, fmt.Errorf("preparing tab: %w", err)
	}
	return tabCtx, cancel, nil
}

func launch(parent context.Context, o sessionOpts) (*session, error) {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.UserDataDir(o.userDataDir),
		chromedp.Flag("profile-directory", o.profileName),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("useAutomationExtension", false),
		chromedp.Flag("disable-infobars", true),
		// A persistent profile that was not closed cleanly triggers the restore
		// bubble, which steals focus and covers the page.
		chromedp.Flag("hide-crash-restore-bubble", true),
		// On a server there is usually no keyring, and a half-configured one
		// can block Chrome at startup. Forcing the basic store also keeps the
		// cookie encryption key deterministic, so the profile directory stays
		// readable if it is ever moved between Linux machines.
		chromedp.Flag("password-store", "basic"),
	)
	if !o.headless {
		// A false bool drops the flag entirely, which is what launches a real
		// window. hide-scrollbars comes from chromedp's Headless default and is
		// itself a headless tell, so undo it too. No window-size override
		// either: let the profile's own geometry apply.
		opts = append(opts,
			chromedp.Flag("headless", false),
			chromedp.Flag("hide-scrollbars", false),
		)
	}
	if o.noSandbox {
		opts = append(opts, chromedp.NoSandbox)
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(parent, opts...)
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx, chromedp.WithErrorf(chromedpErrorf(o.log)))
	s := &session{
		ctx:     browserCtx,
		cancels: []context.CancelFunc{cancelBrowser, cancelAlloc},
		log:     o.log,
	}

	err := chromedp.Run(browserCtx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, err := page.AddScriptToEvaluateOnNewDocument(stealthJS).Do(ctx)
		return err
	}))
	if err != nil {
		s.close()
		return nil, fmt.Errorf("starting chrome: %w", err)
	}

	if err := s.reportUserAgent(o.normalizeUA); err != nil {
		s.log.Printf("WARNING: user agent handling failed: %v", err)
	}
	return s, nil
}

// close asks Chrome to shut down gracefully. This matters with a persistent
// profile: cookies are only flushed to disk on a clean exit, so killing the
// process throws away the session the run just built.
//
// Closing the window by hand is also a clean exit - Chrome flushes on its way
// out. But chromedp cancels the context when the connection drops, and asking a
// browser that is already gone to close itself hangs, so that case is detected
// and skipped.
func (s *session) close() {
	if s.ctx.Err() != nil {
		s.log.Printf("browser already exited on its own; profile was saved by Chrome")
		for i := len(s.cancels) - 1; i >= 0; i-- {
			s.cancels[i]()
		}
		return
	}

	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()
	if err := chromedp.Cancel(ctx); err != nil {
		s.log.Printf("WARNING: graceful shutdown failed (%v) - profile data may not have been saved", err)
	}
	for i := len(s.cancels) - 1; i >= 0; i-- {
		s.cancels[i]()
	}
}

// closed reports whether Chrome has gone away, for example because the window
// was closed from the remote desktop.
func (s *session) closed() <-chan struct{} { return s.ctx.Done() }

// uaHints mirrors the result of navigator.userAgentData.getHighEntropyValues.
type uaHints struct {
	Architecture    string      `json:"architecture"`
	Bitness         string      `json:"bitness"`
	Brands          []brandPair `json:"brands"`
	FullVersionList []brandPair `json:"fullVersionList"`
	Mobile          bool        `json:"mobile"`
	Model           string      `json:"model"`
	Platform        string      `json:"platform"`
	PlatformVersion string      `json:"platformVersion"`
	Wow64           bool        `json:"wow64"`
}

type brandPair struct {
	Brand   string `json:"brand"`
	Version string `json:"version"`
}

// reportUserAgent logs the real user agent and, when asked, rewrites the
// headless one. Nothing is invented: the string comes from the running binary
// via Browser.getVersion and only the word HeadlessChrome is replaced, so the
// version numbers stay true and keep matching the Sec-CH-UA client hints.
func (s *session) reportUserAgent(normalize bool) error {
	var ua string
	err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		_, _, _, agent, _, err := cdpbrowser.GetVersion().Do(ctx)
		ua = agent
		return err
	}))
	if err != nil {
		return err
	}
	s.log.Printf("chrome user agent: %s", ua)

	if !strings.Contains(ua, "HeadlessChrome") {
		return nil
	}
	if !normalize {
		s.log.Printf("WARNING: user agent advertises HeadlessChrome (use -fix-ua to rewrite it)")
		return nil
	}

	fixed := strings.ReplaceAll(ua, "HeadlessChrome", "Chrome")
	params := emulation.SetUserAgentOverride(fixed)

	// The Sec-CH-UA headers are built from client hint metadata, not from the
	// UA string, so overriding the string alone leaves HeadlessChrome visible
	// in the brand list. Read the live hints and patch the same word there.
	hints, hintErr := s.readUAHints()
	if hintErr != nil {
		s.log.Printf("WARNING: could not read client hints (%v) - rewriting the UA string only; Sec-CH-UA may still say HeadlessChrome", hintErr)
	} else {
		params = params.WithUserAgentMetadata(&emulation.UserAgentMetadata{
			Brands:          rebrand(hints.Brands),
			FullVersionList: rebrand(hints.FullVersionList),
			Platform:        hints.Platform,
			PlatformVersion: hints.PlatformVersion,
			Architecture:    hints.Architecture,
			Model:           hints.Model,
			Mobile:          hints.Mobile,
			Bitness:         hints.Bitness,
			Wow64:           hints.Wow64,
		})
	}

	if err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return params.Do(ctx)
	})); err != nil {
		return err
	}

	s.mu.Lock()
	s.uaOverride = params
	s.mu.Unlock()

	s.log.Printf("user agent rewritten to: %s", fixed)
	return nil
}

func (s *session) readUAHints() (*uaHints, error) {
	const js = `navigator.userAgentData
  ? navigator.userAgentData.getHighEntropyValues(['architecture','bitness','model','platformVersion','fullVersionList','wow64'])
  : null`

	awaitPromise := func(p *cdpruntime.EvaluateParams) *cdpruntime.EvaluateParams {
		return p.WithAwaitPromise(true)
	}

	var hints uaHints
	err := chromedp.Run(s.ctx, chromedp.ActionFunc(func(ctx context.Context) error {
		return chromedp.Evaluate(js, &hints, awaitPromise).Do(ctx)
	}))
	if err != nil {
		return nil, err
	}
	return &hints, nil
}

// rebrand swaps HeadlessChrome for the brand real Chrome reports.
func rebrand(in []brandPair) []*emulation.UserAgentBrandVersion {
	out := make([]*emulation.UserAgentBrandVersion, 0, len(in))
	for _, b := range in {
		name := b.Brand
		if strings.Contains(name, "HeadlessChrome") {
			name = strings.ReplaceAll(name, "HeadlessChrome", "Google Chrome")
		}
		out = append(out, &emulation.UserAgentBrandVersion{Brand: name, Version: b.Version})
	}
	return out
}

// humanScroll walks down the page in a few smooth steps, which both triggers
// lazily rendered results and avoids an instant machine-speed read.
func humanScroll(ctx context.Context) error {
	steps := 3 + rand.Intn(3)
	for i := 0; i < steps; i++ {
		js := fmt.Sprintf(`window.scrollBy({ top: %d, left: 0, behavior: 'smooth' })`, 350+rand.Intn(450))
		if err := chromedp.Evaluate(js, nil).Do(ctx); err != nil {
			return err
		}
		if err := pause(250*time.Millisecond, 700*time.Millisecond).Do(ctx); err != nil {
			return err
		}
	}
	return chromedp.Evaluate(`window.scrollTo({ top: 0, behavior: 'smooth' })`, nil).Do(ctx)
}

func pause(min, max time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		d := min
		if max > min {
			d = min + time.Duration(rand.Int63n(int64(max-min)))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(d):
			return nil
		}
	})
}

// collectLinksJS returns hrefs from search result blocks, skipping
// navigation, footers, sidebars, and other non-result chrome inside the
// result container. The resultRoots scope the search; within each root,
// only anchors inside recognised result-element selectors are collected.
func collectLinksJS(roots []string) string {
	resultSelectors := []string{
		"[role=listing] > li",          // Google organic results
		"[role=listing] a[data-href]",   // Google result links with data-href
		"div.g",                         // Google result cards
		"ol.react-results--main > li",   // DuckDuckGo organic results
		"article[data-result-type]",     // Bing organic results
		"li.b_algo",                     // Bing result cards
		"div.result",                    // generic
		"div.search-result",             // generic
		"div[data-result]",              // generic
	}
	return fmt.Sprintf(`(() => {
  const rootSelectors = %s;
  const resultSelectors = %s;
  const rootScopes = [];
  for (const sel of rootSelectors) {
    document.querySelectorAll(sel).forEach((n) => rootScopes.push(n));
  }
  const roots = rootScopes.length ? rootScopes : [document.body];
  const seen = new Set();
  const out = [];
  for (const root of roots) {
    // Try to find result blocks inside the root
    let resultBlocks = [];
    for (const sel of resultSelectors) {
      root.querySelectorAll(sel).forEach((n) => resultBlocks.push(n));
    }
    // If no result blocks found, fall back to all anchors (safety net)
    if (resultBlocks.length === 0) {
      for (const a of root.querySelectorAll('a[href]')) {
        const href = a.href;
        if (!href || seen.has(href)) continue;
        skipIfNav(a);
        seen.add(href);
        out.push(href);
      }
      continue;
    }
    // Collect anchors from result blocks only
    for (const block of resultBlocks) {
      for (const a of block.querySelectorAll('a[href]')) {
        const href = a.href || a.dataset?.href;
        if (!href || seen.has(href)) continue;
        seen.add(href);
        out.push(href);
      }
    }
  }
  function skipIfNav(el) {
    const nav = el.closest('nav, footer, aside');
    if (nav) return;
    const role = (el.getAttribute('role') || '').toLowerCase();
    if (role === 'navigation' || role === 'complementary') return;
  }
  return out;
})()`, jsArray(roots), jsArray(resultSelectors))
}

func jsArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, v := range values {
		quoted = append(quoted, strconv.Quote(v))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}
