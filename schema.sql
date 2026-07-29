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

-- Scrape tables are created now so the scraper stage is purely additive. They
-- stay empty until it lands.
CREATE TABLE IF NOT EXISTS scrapes (
    id             TEXT PRIMARY KEY,
    url_id         TEXT NOT NULL REFERENCES urls (id),
    run_id         TEXT REFERENCES runs (id),
    http_status    INTEGER,
    content_type   TEXT,
    fetched_with   TEXT,
    robots_allowed INTEGER NOT NULL DEFAULT 1,
    title          TEXT,
    raw_html       TEXT,
    clean_html     TEXT,
    text_content   TEXT,
    error          TEXT,
    duration_ms    INTEGER,
    created_at     TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS scrapes_url_idx ON scrapes (url_id, created_at);
CREATE INDEX IF NOT EXISTS scrapes_run_idx ON scrapes (run_id);

CREATE TABLE IF NOT EXISTS scrape_images (
    id         TEXT PRIMARY KEY,
    scrape_id  TEXT NOT NULL REFERENCES scrapes (id),
    url        TEXT NOT NULL,
    alt        TEXT,
    width      INTEGER,
    height     INTEGER,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS scrape_images_scrape_idx ON scrape_images (scrape_id);
