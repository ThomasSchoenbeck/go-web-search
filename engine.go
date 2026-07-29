package main

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type engineDef struct {
	name string
	// home is the landing page used by typed mode.
	home string
	// searchPath builds the direct results URL used by URL mode.
	searchPath func(term string) string
	// resultsParam is the query parameter controlling result count, empty when
	// the engine has none. Only ever applied in URL mode.
	resultsParam string
	// searchBox selectors are tried in order; the first present one is used.
	searchBox []string
	// resultRoots scope link collection and signal that results have rendered.
	resultRoots []string
	// consentSelectors are cookie/consent buttons to click if present.
	consentSelectors []string
	// skipLabels are DNS labels marking engine infrastructure ("google" also
	// matches policies.google.com).
	skipLabels []string
	skipHosts  []string
}

// searchURL is only used in URL mode. Language and personalisation parameters
// are deliberately absent: hl/gl/pws are rank-tracker fingerprints, and the
// browser sends its own Accept-Language anyway.
func (e engineDef) searchURL(term string, results int) string {
	u := e.searchPath(term)
	if results > 0 && e.resultsParam != "" {
		u += "&" + e.resultsParam + "=" + strconv.Itoa(results)
	}
	return u
}

var sharedSkipLabels = []string{
	"gstatic", "googleusercontent", "googleapis",
	"googlesyndication", "googleadservices", "doubleclick",
}

func allEngines() []engineDef {
	return []engineDef{
		{
			name: "google",
			home: "https://www.google.com/",
			searchPath: func(term string) string {
				return "https://www.google.com/search?q=" + url.QueryEscape(term)
			},
			resultsParam: "num",
			searchBox:    []string{`textarea[name="q"]`, `input[name="q"]`, "#APjFqb"},
			resultRoots:  []string{"#rso", "#search", "#center_col"},
			consentSelectors: []string{
				"button#L2AGLb",
				"button#W0wltc",
				`form[action*="consent"] button`,
				`button[aria-label*="Accept"]`,
			},
			skipLabels: append([]string{"google"}, sharedSkipLabels...),
		},
		{
			name: "bing",
			home: "https://www.bing.com/",
			searchPath: func(term string) string {
				return "https://www.bing.com/search?q=" + url.QueryEscape(term)
			},
			resultsParam: "count",
			searchBox:    []string{"#sb_form_q", `textarea[name="q"]`, `input[name="q"]`},
			resultRoots:  []string{"#b_results", "main"},
			consentSelectors: []string{
				"#bnp_btn_accept",
				`button[aria-label*="Accept"]`,
			},
			skipLabels: append([]string{"bing", "microsofttranslator"}, sharedSkipLabels...),
			skipHosts:  []string{"go.microsoft.com", "login.live.com"},
		},
		{
			name: "duckduckgo",
			home: "https://duckduckgo.com/",
			searchPath: func(term string) string {
				return "https://duckduckgo.com/?q=" + url.QueryEscape(term)
			},
			resultsParam: "",
			searchBox:    []string{"#searchbox_input", `input[name="q"]`},
			resultRoots:  []string{`[data-testid="mainline"]`, "ol.react-results--main", "#links"},
			skipLabels:   append([]string{"duckduckgo", "duck"}, sharedSkipLabels...),
			skipHosts:    []string{"spreadprivacy.com"},
		},
	}
}

func enginesByNames(csv string) ([]engineDef, error) {
	byName := make(map[string]engineDef)
	for _, e := range allEngines() {
		byName[e.name] = e
	}
	var out []engineDef
	for _, name := range splitList(csv) {
		e, ok := byName[canonicalEngine(name)]
		if !ok {
			return nil, fmt.Errorf("unknown engine %q (available: google, bing, duckduckgo)", name)
		}
		out = append(out, e)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no engines selected")
	}
	return out, nil
}

func canonicalEngine(name string) string {
	if name == "ddg" {
		return "duckduckgo"
	}
	return name
}

func splitList(csv string) []string {
	var out []string
	for _, raw := range strings.Split(csv, ",") {
		v := strings.ToLower(strings.TrimSpace(raw))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// parseEngineSet reads a csv engine list into a lookup set. "none" is accepted
// as an explicit empty set.
func parseEngineSet(csv string) map[string]bool {
	set := make(map[string]bool)
	for _, name := range splitList(csv) {
		if name == "none" {
			continue
		}
		set[canonicalEngine(name)] = true
	}
	return set
}

// parseResultCounts reads "google=20,bing=30". 0 (or absent) means: send no
// count parameter at all and take the engine default.
func parseResultCounts(spec string) (map[string]int, error) {
	counts := make(map[string]int)
	for _, pair := range splitList(spec) {
		name, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("bad -results entry %q, want engine=count", pair)
		}
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || n < 0 {
			return nil, fmt.Errorf("bad result count in %q", pair)
		}
		counts[canonicalEngine(strings.TrimSpace(name))] = n
	}
	return counts, nil
}

var blockHints = []string{
	"unusual traffic",
	"verify you are human",
	"are you a robot",
	"our systems have detected",
	"recaptcha",
	"solve the challenge",
}

// looksBlocked reports whether the landed page is a challenge rather than
// results. It is a logging heuristic, so a query about captchas can trip it.
func looksBlocked(landedURL, html string) bool {
	lower := strings.ToLower(landedURL)
	if strings.Contains(lower, "/sorry/") || strings.Contains(lower, "captcha") {
		return true
	}
	body := strings.ToLower(html)
	if len(body) > 200000 {
		body = body[:200000]
	}
	for _, hint := range blockHints {
		if strings.Contains(body, hint) {
			return true
		}
	}
	return false
}

// landedQuery pulls the query back out of the URL the engine navigated to, so
// typed mode can confirm the search that ran is the search that was asked for.
func landedQuery(landed string) string {
	u, err := url.Parse(landed)
	if err != nil {
		return ""
	}
	q := u.Query()
	for _, key := range []string{"q", "p", "query"} {
		if v := q.Get(key); v != "" {
			return v
		}
	}
	return ""
}

func sameQuery(a, b string) bool {
	norm := func(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
	return norm(a) == norm(b)
}
