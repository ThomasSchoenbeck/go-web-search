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
	Jobs      JobsConfig      `toml:"jobs"`
	Retention RetentionConfig `toml:"retention"`
	Cache     TierConfig      `toml:"cache"`
	Memory    MemoryConfig    `toml:"memory"`

	Observability ObservabilityConfig `toml:"observability"`
}

// ObservabilityConfig holds the tunables the observability SPA reads at startup
// from GET /api/ui-config. Everything here is non-secret by construction: the
// assumed deployment leaves api_key unset and delegates auth to an edge, so
// that endpoint answers anyone who can reach the port. Never add a credential
// to this struct.
type ObservabilityConfig struct {
	// PollInterval and PollEnabled seed the SPA's live-refresh defaults for the
	// jobs and logs views. The UI can override both for the current session; it
	// never writes back here.
	PollInterval Duration `toml:"poll_interval"`
	PollEnabled  bool     `toml:"poll_enabled"`
	// ProjectionSampleCap bounds how many embedding vectors the 2-D projection
	// view may pull. Vector search is a linear scan, so this is a real limit.
	ProjectionSampleCap int `toml:"projection_sample_cap"`
	// CausalityMaxURLs bounds the whole-run causality graph. A broad run can
	// find thousands of URLs, each dragging its scrape and facts into the
	// response; past this many the graph is truncated and says so.
	CausalityMaxURLs int `toml:"causality_max_urls"`
	// JobTimingSample bounds how many finished jobs /api/stats averages a
	// completion time over. The whole history would be an unbounded scan for a
	// single number; 0 skips the timing entirely.
	JobTimingSample int `toml:"job_timing_sample"`
}

// RetentionConfig governs shrinking the database beyond the tier expiry: one
// sweep that clears large already-processed blobs, plus VACUUM to reclaim space.
type RetentionConfig struct {
	// TrimRaw enables the sweep. It nulls scrape raw_html/clean_html and deletes
	// search SERP HTML for rows older than RawMaxAge, except the newest
	// RawKeepLast of each kind (kept for debugging).
	TrimRaw     bool     `toml:"trim_raw"`
	RawMaxAge   Duration `toml:"raw_max_age"`
	RawKeepLast int      `toml:"raw_keep_last"`
	// VacuumOnStartup runs VACUUM once at boot, before workers start.
	VacuumOnStartup bool `toml:"vacuum_on_startup"`
	// VacuumAt is an optional daily "HH:MM" (local) to also VACUUM mid-run;
	// empty disables it. VACUUM holds an exclusive lock for its duration.
	VacuumAt string `toml:"vacuum_at"`
}

// JobsConfig tunes the background job runner.
type JobsConfig struct {
	// StaleAfter is how long a claimed job may run before the reaper assumes the
	// worker died and requeues it. It MUST exceed the longest a job can
	// legitimately take — for distill that is one LLM call bounded by
	// llm.timeout — otherwise long jobs get reaped mid-flight and run again,
	// doubling the work. Trade-off: a genuinely crashed job takes this long to
	// be recovered.
	StaleAfter Duration `toml:"stale_after"`
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
	// AutoVacuum sets PRAGMA auto_vacuum, and only takes effect on a FRESH
	// database — SQLite/Turso cannot flip it on a non-empty file. "full"
	// reclaims freed pages continuously; "none" relies solely on explicit
	// VACUUM. Applied best-effort (a driver that lacks it is ignored).
	AutoVacuum string `toml:"auto_vacuum"`
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
	TermsFile string   `toml:"terms_file"`
	Engines   []string `toml:"engines"`
	Typed     []string `toml:"typed"`
	// ExcludeHosts drops result URLs on these registrable domains across every
	// engine, on top of each engine's built-in skip list. "msn.com" also
	// matches "www.msn.com". Configure here instead of editing engine.go.
	ExcludeHosts []string       `toml:"exclude_hosts"`
	Results      map[string]int `toml:"results"`
	MinDelay     Duration       `toml:"min_delay"`
	MaxDelay     Duration       `toml:"max_delay"`
	QueryTimeout Duration       `toml:"query_timeout"`
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
	// Timeout bounds every LLM HTTP call. A multimodel llama-server cold-loads a
	// model into VRAM on first hit, which can exceed a minute, so this must be
	// generous. 0 falls back to the built-in default.
	Timeout Duration `toml:"timeout"`
	// EmbedConcurrency caps how many embedding requests run at once during a
	// bulk re-embed. The embedding server accepts parallel requests; this lets
	// the app use them. Values <=1 embed sequentially.
	EmbedConcurrency int `toml:"embed_concurrency"`
}

type LLMModel struct {
	Name     string `toml:"name"`
	Kind     string `toml:"kind"`     // "chat" or "embed"
	Endpoint string `toml:"endpoint"` // OpenAI-compatible base URL
	Model    string `toml:"model"`    // model id sent in the request body
	APIKey   string `toml:"api_key"`  // optional bearer token
	// APIPath is the OpenAI route prefix prepended to /chat/completions and
	// /embeddings. A nil value (key absent) defaults to "/v1"; set it to "" for
	// a multimodel llama-server that serves those routes at the root.
	APIPath *string `toml:"api_path"`
	// Dim is the embedding dimension for an "embed" model. It is stamped onto
	// every vector row so a model/dim change can be detected and migrated.
	Dim int `toml:"dim"`
	// QueryPrefix and DocPrefix implement asymmetric embedding: a query and a
	// stored document are embedded with different instructions. Qwen3-Embedding
	// wants an instruction on the query and none on the document.
	QueryPrefix string `toml:"query_prefix"`
	DocPrefix   string `toml:"doc_prefix"`
	// NoThink appends Qwen's "/no_think" soft switch to the chat prompt to
	// disable the model's thinking, independent of build-specific request
	// fields. Only applied to the chat role.
	NoThink bool `toml:"no_think"`
	// MaxTokens caps chat output when > 0; 0 (default) omits the field and lets
	// the server decide. Only applied to the chat role.
	MaxTokens int `toml:"max_tokens"`
	// ExtraBody is merged verbatim into the chat request JSON, for server- or
	// model-specific knobs the fixed fields don't cover — e.g. disabling a
	// thinking model or setting a reasoning budget. Only applied to the chat role.
	ExtraBody map[string]any `toml:"extra_body"`
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
			AutoVacuum:   "full",
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
			// Must match config.toml and the SPA's dev proxy target
			// (web/vite.config.ts): with no config file present this default is
			// the listener the dev server proxies to.
			Addr:         "0.0.0.0:8082",
			ReadTimeout:  Duration{30 * time.Second},
			WriteTimeout: Duration{180 * time.Second},
		},
		LLM: LLMConfig{
			Timeout:          Duration{120 * time.Second},
			EmbedConcurrency: 4,
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
		Jobs: JobsConfig{
			StaleAfter: Duration{30 * time.Minute},
		},
		Retention: RetentionConfig{
			TrimRaw:         true,
			RawMaxAge:       Duration{48 * time.Hour},
			RawKeepLast:     10,
			VacuumOnStartup: true,
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
		Observability: ObservabilityConfig{
			PollInterval:        Duration{5 * time.Second},
			PollEnabled:         false,
			ProjectionSampleCap: 2000,
			CausalityMaxURLs:    200,
			JobTimingSample:     200,
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
