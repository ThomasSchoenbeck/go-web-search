-- Log database. Deliberately a separate file: log writes are frequent and
-- uninteresting, and keeping them out of the main database means a busy log
-- writer can never block a search or scrape transaction.

CREATE TABLE IF NOT EXISTS logs (
    id         TEXT PRIMARY KEY,
    run_id     TEXT,
    level      TEXT NOT NULL,
    source     TEXT,
    message    TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS logs_run_idx ON logs (run_id, created_at);
CREATE INDEX IF NOT EXISTS logs_level_idx ON logs (level, created_at);
