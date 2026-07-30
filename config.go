package main

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

// Duration lets TOML carry human readable values like "4s".
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("bad duration %q: %w", text, err)
	}
	d.Duration = parsed
	return nil
}

type Config struct {
	Database DatabaseConfig `toml:"database"`
	Browser  BrowserConfig  `toml:"browser"`
	Search   SearchConfig   `toml:"search"`
	Scrape   ScrapeConfig   `toml:"scrape"`
	Server   ServerConfig   `toml:"server"`
	LLM      LLMConfig      `toml:"llm"`
	Cache    TierConfig     `toml:"cache"`
	Memory   MemoryConfig   `toml:"memory"`
}

type DatabaseConfig struct {
	// Driver is the name the Turso package registers with database/sql. It is
	// configurable because the Go bindings have moved packages recently and the
	// registered name is the kind of thing that changes with them.
	Driver  string `toml:"driver"`
	DataDir string `toml:"data_dir"`
	MainDB  string `toml:"main_db"`
	LogDB   string `toml:"log_db"`
	// ExclusiveLock refuses a second process access to the data directory.
	// Turso supports multi-process access, so this defaults off; it exists as
	// an escape hatch because that support is recent.
	ExclusiveLock bool `toml:"exclusive_lock"`
	// MaxOpenConns caps database/sql connections. 1 serialises writes, which is
	// the conservative choice while the Turso bindings are beta. Raise it to
	// let Turso's concurrency actually do something.
	MaxOpenConns int `toml:"max_open_conns"`
}

type BrowserConfig struct {
	Headless    bool   `toml:"headless"`
	UserDataDir string `toml:"user_data_dir"`
	Profile     string `toml:"profile"`
	FixUA       bool   `toml:"fix_ua"`
	NoSandbox   bool   `toml:"no_sandbox"`
	StartURL    string `toml:"start_url"`
}

type SearchConfig struct {
	TermsFile    string         `toml:"terms_file"`
	Engines      []string       `toml:"engines"`
	Typed        []string       `toml:"typed"`
	Results      map[string]int `toml:"results"`
	MinDelay     Duration       `toml:"min_delay"`
	MaxDelay     Duration       `toml:"max_delay"`
	QueryTimeout Duration       `toml:"query_timeout"`
	ArtifactDir  string         `toml:"artifact_dir"`
	// EngineStagger delays each engine's start relative to the previous one so
	// the three queries for a term do not leave as one correlated burst.
	// EngineJitter is added at random on top.
	EngineStagger Duration `toml:"engine_stagger"`
	EngineJitter  Duration `toml:"engine_jitter"`
}

type ScrapeConfig struct {
	Enabled           bool     `toml:"enabled"`
	UserAgent         string   `toml:"user_agent"`
	RobotsUserAgent   string   `toml:"robots_user_agent"`
	MaxDomains        int      `toml:"max_domains"`
	PerDomainDelay    Duration `toml:"per_domain_delay"`
	RespectCrawlDelay bool     `toml:"respect_crawl_delay"`
	HTTPTimeout       Duration `toml:"http_timeout"`
	MaxBodyBytes      int64    `toml:"max_body_bytes"`
	BrowserFallback   bool     `toml:"browser_fallback"`
	// MaxBrowserTabs bounds concurrent fallback tabs. Each is a Chrome
	// renderer, so this trades memory for throughput.
	MaxBrowserTabs int `toml:"max_browser_tabs"`
	// MinTextChars is the JS-rendered heuristic: a document whose extracted
	// text falls below this is retried in the browser.
	MinTextChars int      `toml:"min_text_chars"`
	MaxURLs      int      `toml:"max_urls"`
	TotalTimeout Duration `toml:"total_timeout"`
	// SnippetChars caps inline text returned to an LLM. Full content stays
	// available through the by-id endpoints.
	SnippetChars int `toml:"snippet_chars"`
}

type ServerConfig struct {
	Addr         string   `toml:"addr"`
	ReadTimeout  Duration `toml:"read_timeout"`
	WriteTimeout Duration `toml:"write_timeout"`
	// APIKey, when set, is required as `Authorization: Bearer <key>`.
	APIKey string `toml:"api_key"`
}

// LLMConfig lists model endpoints the app can call. Entries are keyed by role
// via Kind ("chat" or "embed"); the first entry of each kind is the active one.
// Every endpoint is OpenAI-compatible, so a self-hosted llama.cpp and a hosted
// provider are configured the same way. Config file only, no CLI flags.
type LLMConfig struct {
	Models []LLMModel `toml:"model"`
}

type LLMModel struct {
	Name     string `toml:"name"`
	Kind     string `toml:"kind"`     // "chat" or "embed"
	Endpoint string `toml:"endpoint"` // OpenAI-compatible base URL
	Model    string `toml:"model"`    // model id sent in the request body
	APIKey   string `toml:"api_key"`  // optional bearer token
	// Dim is the embedding dimension for an "embed" model. It is stamped onto
	// every vector row so a model/dim change can be detected and migrated.
	Dim int `toml:"dim"`
	// QueryPrefix and DocPrefix implement asymmetric embedding: a query and a
	// stored document are embedded with different instructions. Qwen3-Embedding
	// wants an instruction on the query and none on the document.
	QueryPrefix string `toml:"query_prefix"`
	DocPrefix   string `toml:"doc_prefix"`
}

// TierConfig drives the shared sliding-expiry / promotion behaviour across the
// search cache, scrape cache and memory. "permanent" rows never expire.
type TierConfig struct {
	ShortTTL         Duration `toml:"short_ttl"`
	LongTTL          Duration `toml:"long_ttl"`
	PromoteAfterHits int      `toml:"promote_after_hits"`
	CleanupInterval  Duration `toml:"cleanup_interval"`
}

// MemoryConfig tunes the confidence gates that decide whether a memory hit may
// skip a web search, and the semantic-upsert threshold used when storing facts.
type MemoryConfig struct {
	SimilarityThreshold float64 `toml:"similarity_threshold"`
	UpsertThreshold     float64 `toml:"upsert_threshold"`
	TopK                int     `toml:"top_k"`
	Gate3Enabled        bool    `toml:"gate3_enabled"`
	RememberDefault     string  `toml:"remember_default"`
}

func defaultConfig() Config {
	return Config{
		Database: DatabaseConfig{
			Driver:       "turso",
			DataDir:      "./data",
			MainDB:       "go-web-search.db",
			LogDB:        "go-web-search-logs.db",
			MaxOpenConns: 4,
		},
		Browser: BrowserConfig{
			Headless:    false,
			UserDataDir: "./chrome-user-data",
			Profile:     "Tom",
			FixUA:       true,
			StartURL:    "https://www.google.com/",
		},
		Search: SearchConfig{
			TermsFile:     "terms.txt",
			Engines:       []string{"google", "bing", "duckduckgo"},
			Typed:         []string{"google", "bing", "duckduckgo"},
			Results:       map[string]int{},
			MinDelay:      Duration{4 * time.Second},
			MaxDelay:      Duration{11 * time.Second},
			QueryTimeout:  Duration{90 * time.Second},
			ArtifactDir:   "runs",
			EngineStagger: Duration{1500 * time.Millisecond},
			EngineJitter:  Duration{1200 * time.Millisecond},
		},
		Scrape: ScrapeConfig{
			Enabled:           true,
			UserAgent:         "go-web-search/0.1 (+local research tool)",
			RobotsUserAgent:   "go-web-search",
			MaxDomains:        8,
			PerDomainDelay:    Duration{1 * time.Second},
			RespectCrawlDelay: true,
			HTTPTimeout:       Duration{20 * time.Second},
			MaxBodyBytes:      5 << 20,
			BrowserFallback:   true,
			MaxBrowserTabs:    3,
			MinTextChars:      500,
			MaxURLs:           25,
			TotalTimeout:      Duration{120 * time.Second},
			SnippetChars:      2000,
		},
		Server: ServerConfig{
			Addr:         "0.0.0.0:8081",
			ReadTimeout:  Duration{30 * time.Second},
			WriteTimeout: Duration{180 * time.Second},
		},
		LLM: LLMConfig{
			Models: []LLMModel{
				{
					Name:     "chat",
					Kind:     "chat",
					Endpoint: "http://192.168.178.64:8080",
					Model:    "local-chat",
				},
				{
					Name:        "embed",
					Kind:        "embed",
					Endpoint:    "http://192.168.178.64:8080",
					Model:       "Qwen3-Embedding-8B",
					Dim:         4096,
					QueryPrefix: "Instruct: Given a web search query, retrieve relevant passages that answer the query\nQuery: ",
				},
			},
		},
		Cache: TierConfig{
			ShortTTL:         Duration{10 * 24 * time.Hour},
			LongTTL:          Duration{45 * 24 * time.Hour},
			PromoteAfterHits: 3,
			CleanupInterval:  Duration{1 * time.Hour},
		},
		Memory: MemoryConfig{
			SimilarityThreshold: 0.85,
			UpsertThreshold:     0.95,
			TopK:                8,
			Gate3Enabled:        true,
			RememberDefault:     "short",
		},
	}
}

// loadConfig starts from the defaults and overlays the file if there is one.
// A missing file at the default path is not an error; a missing file that was
// asked for explicitly is.
func loadConfig(path string, explicit bool) (Config, error) {
	cfg := defaultConfig()
	if path == "" {
		return cfg, nil
	}

	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) && !explicit {
			return cfg, nil
		}
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}

	meta, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("config %s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		// Silently ignored config keys are how people lose an afternoon.
		return cfg, fmt.Errorf("config %s: unknown keys %v", path, undecoded)
	}
	if cfg.Search.Results == nil {
		cfg.Search.Results = map[string]int{}
	}
	return cfg, nil
}
