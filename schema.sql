-- Main database. Applied on every open; every statement is IF NOT EXISTS so
-- this doubles as the migration for a fresh file.
--
-- All primary keys are UUIDv7 stored as TEXT: time-ordered, so inserts stay at
-- the right edge of the index instead of scattering the way v4 does.
-- All timestamps are RFC3339 in UTC.

CREATE TABLE IF NOT EXISTS runs (
    id           TEXT PRIMARY KEY,
    mode         TEXT NOT NULL,
    artifact_dir TEXT,
    started_at   TEXT NOT NULL,
    finished_at  TEXT,
    created_at   TEXT NOT NULL
);

-- One row per (run, term, engine).
CREATE TABLE IF NOT EXISTS searches (
    id            TEXT PRIMARY KEY,
    run_id        TEXT NOT NULL REFERENCES runs (id),
    term          TEXT NOT NULL,
    engine        TEXT NOT NULL,
    search_mode   TEXT NOT NULL,
    requested_url TEXT,
    landed_url    TEXT,
    http_status   INTEGER,
    blocked       INTEGER NOT NULL DEFAULT 0,
    anchor_count  INTEGER NOT NULL DEFAULT 0,
    error         TEXT,
    duration_ms   INTEGER,
    created_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS searches_run_idx ON searches (run_id, created_at);
CREATE INDEX IF NOT EXISTS searches_term_idx ON searches (term);

-- Raw SERP HTML lives in its own table so that scanning `searches` never has to
-- drag half a megabyte of markup per row along with it.
CREATE TABLE IF NOT EXISTS search_raw (
    id         TEXT PRIMARY KEY,
    search_id  TEXT NOT NULL REFERENCES searches (id),
    html       TEXT NOT NULL,
    byte_size  INTEGER NOT NULL,
    created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS search_raw_search_idx ON search_raw (search_id);

-- Canonical URL registry. Deduplicated across every run and engine, so the
-- scraper has a single place to work from.
CREATE TABLE IF NOT EXISTS urls (
    id            TEXT PRIMARY KEY,
    url           TEXT NOT NULL UNIQUE,
    domain        TEXT NOT NULL,
    first_seen_at TEXT NOT NULL,
    created_at    TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS urls_domain_idx ON urls (domain);

-- Which search found which URL, and where it ranked.
CREATE TABLE IF NOT EXISTS search_urls (
    id         TEXT PRIMARY KEY,
    search_id  TEXT NOT NULL REFERENCES searches (id),
    url_id     TEXT NOT NULL REFERENCES urls (id),
    rank       INTEGER NOT NULL,
    created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS search_urls_unique_idx ON search_urls (search_id, url_id);
CREATE INDEX IF NOT EXISTS search_urls_url_idx ON search_urls (url_id);

-- Scrape cache: URL -> fetched, cleaned content, keyed exactly on URL. Replaces
-- the old per-fetch `scrapes` / `scrape_images` tables (green-field, no shim).
-- A content hash plus etag/last_modified drive cheap conditional refresh; images
-- are stored inline as JSON. Tiered with sliding expiry as a re-fetch shield.
CREATE TABLE IF NOT EXISTS scrape_cache (
    id             TEXT PRIMARY KEY,
    url            TEXT NOT NULL UNIQUE,
    run_id         TEXT,
    http_status    INTEGER,
    content_type   TEXT,
    fetched_with   TEXT,
    robots_allowed INTEGER NOT NULL DEFAULT 1,
    title          TEXT,
    raw_html       TEXT,
    clean_html     TEXT,
    text_content   TEXT,
    images         TEXT,
    content_hash   TEXT,
    etag           TEXT,
    last_modified  TEXT,
    error          TEXT,
    tier           TEXT NOT NULL DEFAULT 'short',
    hit_count      INTEGER NOT NULL DEFAULT 0,
    expires_at     TEXT,
    fetched_at     TEXT NOT NULL,
    duration_ms    INTEGER,
    created_at     TEXT NOT NULL,
    updated_at     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS scrape_cache_run_idx ON scrape_cache (run_id);
CREATE INDEX IF NOT EXISTS scrape_cache_expiry_idx ON scrape_cache (expires_at);
CREATE INDEX IF NOT EXISTS scrape_cache_hash_idx ON scrape_cache (content_hash);

-- Key/value metadata for the app itself: the active embedding model and
-- dimension, and re-embed migration state. One row per key.
CREATE TABLE IF NOT EXISTS system_meta (
    key        TEXT PRIMARY KEY,
    value      TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Background work queue. One row per unit of deferred work (embed, distill,
-- cleanup, re-embed). A poller claims pending rows onto a worker pool; a reaper
-- returns rows stuck in 'running' (a crashed worker) back to 'pending'.
CREATE TABLE IF NOT EXISTS jobs (
    id         TEXT PRIMARY KEY,
    type       TEXT NOT NULL,
    payload    TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'pending',
    attempts   INTEGER NOT NULL DEFAULT 0,
    run_after  TEXT NOT NULL,
    locked_at  TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS jobs_claim_idx ON jobs (status, run_after);

-- Search cache: a normalized query maps to the result URLs a prior search found.
-- The query is also embedded (in the vectors table, owner_kind 'search') so a
-- differently worded but equivalent query can hit semantically. Tiered with
-- sliding expiry; expires_at NULL means permanent.
CREATE TABLE IF NOT EXISTS search_cache (
    id         TEXT PRIMARY KEY,
    query_norm TEXT NOT NULL UNIQUE,
    query      TEXT NOT NULL,
    results    TEXT NOT NULL,
    tier       TEXT NOT NULL DEFAULT 'short',
    hit_count  INTEGER NOT NULL DEFAULT 0,
    expires_at TEXT,
    fetched_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS search_cache_expiry_idx ON search_cache (expires_at);

-- Memory: atomic facts distilled from scraped pages, each embedded (vectors
-- table, owner_kind 'memory') so a question can retrieve relevant facts even
-- when worded differently. volatility (set by the distiller) drives a freshness
-- gate independent of the durability tier. Tiered with sliding expiry.
CREATE TABLE IF NOT EXISTS memory_facts (
    id         TEXT PRIMARY KEY,
    text       TEXT NOT NULL,
    source_url TEXT,
    volatility TEXT,
    tier       TEXT NOT NULL DEFAULT 'short',
    hit_count  INTEGER NOT NULL DEFAULT 0,
    expires_at TEXT,
    fetched_at TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS memory_facts_expiry_idx ON memory_facts (expires_at);
