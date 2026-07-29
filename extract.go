package main

import (
	"encoding/base64"
	"net/url"
	"strings"
)

// Query parameters engines use to carry the real destination.
// "uddg" is DuckDuckGo, "u" is Bing's base64 wrapper, "q"/"url" are Google's.
var wrapperParams = []string{"uddg", "url", "u", "q", "target"}

// destinations cleans raw hrefs into destination URLs, preserving result order.
func destinations(raw []string, skipLabels, skipHosts []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))

	for _, href := range raw {
		candidate := unwrap(href, 3)
		u, err := url.Parse(candidate)
		if err != nil {
			continue
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			continue
		}
		host := strings.ToLower(u.Hostname())
		if host == "" || skipHost(host, skipLabels, skipHosts) {
			continue
		}

		u.Fragment = ""
		key := strings.TrimSuffix(host+u.EscapedPath(), "/") + "?" + u.RawQuery
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, u.String())
	}
	return out
}

func unwrap(raw string, depth int) string {
	current := raw
	for i := 0; i < depth; i++ {
		next := unwrapOnce(current)
		if next == "" || next == current {
			return current
		}
		current = next
	}
	return current
}

func unwrapOnce(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	for _, key := range wrapperParams {
		v := q.Get(key)
		if v == "" {
			continue
		}
		if isHTTP(v) {
			return v
		}
		if key == "u" {
			if decoded := decodeBingTarget(v); decoded != "" {
				return decoded
			}
		}
	}
	return raw
}

// decodeBingTarget handles Bing's /ck/a?...&u=a1<base64url> click wrapper.
func decodeBingTarget(v string) string {
	s := strings.TrimPrefix(v, "a1")
	s = strings.TrimRight(s, "=")
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return ""
	}
	if decoded := string(b); isHTTP(decoded) {
		return decoded
	}
	return ""
}

func isHTTP(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func skipHost(host string, labels, hosts []string) bool {
	for _, h := range hosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	parts := strings.Split(host, ".")
	for _, label := range labels {
		for _, p := range parts {
			if p == label {
				return true
			}
		}
	}
	return false
}
