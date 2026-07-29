package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

// robotsCache fetches and caches one robots.txt per host for the lifetime of
// the process.
//
// Status handling follows RFC 9309 and is delegated to
// robotstxt.FromStatusAndBytes: 4xx means crawling is allowed, 5xx means it is
// not. A network failure is treated as allow, matching the behaviour of
// mainstream crawlers - a host that cannot serve robots.txt at all should not
// permanently block a single manual research fetch.
type robotsCache struct {
	agent   string
	client  *http.Client
	mu      sync.Mutex
	entries map[string]*robotsEntry
}

type robotsEntry struct {
	once  sync.Once
	group *robotstxt.Group
	err   error
}

func newRobotsCache(agent string, client *http.Client) *robotsCache {
	return &robotsCache{
		agent:   agent,
		client:  client,
		entries: make(map[string]*robotsEntry),
	}
}

// Allowed reports whether target may be fetched, and any crawl delay the host
// asked for.
func (c *robotsCache) Allowed(ctx context.Context, target *url.URL) (bool, time.Duration) {
	group := c.group(ctx, target)
	if group == nil {
		return true, 0
	}
	path := target.EscapedPath()
	if path == "" {
		path = "/"
	}
	if target.RawQuery != "" {
		path += "?" + target.RawQuery
	}
	return group.Test(path), group.CrawlDelay
}

func (c *robotsCache) group(ctx context.Context, target *url.URL) *robotstxt.Group {
	key := target.Scheme + "://" + target.Host

	c.mu.Lock()
	entry, ok := c.entries[key]
	if !ok {
		entry = &robotsEntry{}
		c.entries[key] = entry
	}
	c.mu.Unlock()

	entry.once.Do(func() {
		entry.group = c.fetch(ctx, key)
	})
	return entry.group
}

func (c *robotsCache) fetch(ctx context.Context, origin string) *robotstxt.Group {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+"/robots.txt", nil)
	if err != nil {
		return nil
	}
	req.Header.Set("User-Agent", c.agent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil // unreachable robots.txt: allow, as crawlers conventionally do
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512<<10))
	if err != nil {
		return nil
	}

	data, err := robotstxt.FromStatusAndBytes(resp.StatusCode, body)
	if err != nil {
		return nil
	}
	return data.FindGroup(c.agent)
}
