package main

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

//go:embed schema_logs.sql
var logSchema string

type logEntry struct {
	runID   string
	level   string
	source  string
	message string
	at      string
}

// LogStore writes log lines to their own database file. A single goroutine
// drains a buffered channel and inserts in batches: hundreds of scraper
// goroutines each doing their own INSERT would serialise on the write
// connection and turn logging into the bottleneck.
//
// Writes are non-blocking by design. If the buffer fills, lines are dropped and
// counted rather than stalling a crawl - logging must never be able to wedge
// the thing it is logging about.
type LogStore struct {
	db      *sql.DB
	entries chan logEntry
	dropped atomic.Int64
	done    chan struct{}
	once    sync.Once
}

const (
	logBufferSize = 4096
	logBatchSize  = 128
	logFlushEvery = 500 * time.Millisecond
)

func openLogStore(driver, path string) (*LogStore, error) {
	// Enable auto_vacuum and experimental VACUUM (same as main store) so the
	// log file doesn't grow without bound when old rows are deleted.
	dsn := path
	if strings.Contains(dsn, "?") {
		dsn += "&experimental=vacuum"
	} else {
		dsn += "?experimental=vacuum"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("opening log db %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to log db %s: %w", path, err)
	}
	if _, err := db.Exec("PRAGMA auto_vacuum = FULL"); err != nil {
		_ = err // best-effort — only takes on a fresh DB
	}
	if _, err := db.Exec(logSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("applying log schema: %w", err)
	}

	ls := &LogStore{
		db:      db,
		entries: make(chan logEntry, logBufferSize),
		done:    make(chan struct{}),
	}
	go ls.run()
	return ls, nil
}

func (l *LogStore) run() {
	defer close(l.done)

	batch := make([]logEntry, 0, logBatchSize)
	ticker := time.NewTicker(logFlushEvery)
	defer ticker.Stop()

	for {
		select {
		case entry, ok := <-l.entries:
			if !ok {
				l.flush(batch)
				return
			}
			batch = append(batch, entry)
			if len(batch) >= logBatchSize {
				l.flush(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				l.flush(batch)
				batch = batch[:0]
			}
		}
	}
}

func (l *LogStore) flush(batch []logEntry) {
	if len(batch) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return
	}
	defer tx.Rollback()

	for _, e := range batch {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO logs (id, run_id, level, source, message, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			newID(), nullable(e.runID), e.level, nullable(e.source), e.message, e.at); err != nil {
			return
		}
	}
	tx.Commit()
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Write queues a log line. It never blocks.
func (l *LogStore) Write(runID, level, source, message string) {
	select {
	case l.entries <- logEntry{runID: runID, level: level, source: source, message: message, at: nowRFC3339()}:
	default:
		l.dropped.Add(1)
	}
}

// Close drains the buffer and reports anything that was dropped.
func (l *LogStore) Close() (dropped int64, err error) {
	l.once.Do(func() { close(l.entries) })
	<-l.done
	return l.dropped.Load(), l.db.Close()
}

// ---- read path, for the observability UI ----

// LogEntry is one stored log line.
type LogEntry struct {
	ID        string `json:"id"`
	RunID     string `json:"run_id,omitempty"`
	Level     string `json:"level"`
	Source    string `json:"source,omitempty"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

// LogQuery narrows a log read. An empty field is not filtered on.
type LogQuery struct {
	RunID  string
	Level  string
	Source string
	Limit  int
	Offset int
}

// Query reads log lines newest-first, so the viewer reads as a tail. The store
// was write-only until the observability UI needed to show what it had written;
// this is the only read path, and it reads nothing the batching writer holds.
func (l *LogStore) Query(ctx context.Context, q LogQuery) ([]LogEntry, error) {
	limit, offset := clampPage(q.Limit, q.Offset)
	query := `SELECT id, run_id, level, source, message, created_at FROM logs`
	var (
		where []string
		args  []any
	)
	if q.RunID != "" {
		where = append(where, `run_id = ?`)
		args = append(args, q.RunID)
	}
	if q.Level != "" {
		where = append(where, `level = ?`)
		args = append(args, q.Level)
	}
	if q.Source != "" {
		where = append(where, `source = ?`)
		args = append(args, q.Source)
	}
	if len(where) > 0 {
		query += ` WHERE ` + strings.Join(where, ` AND `)
	}
	// id is a UUIDv7, so it breaks ties in creation order rather than randomly
	// when a batch writes several lines within the same timestamp.
	query += ` ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []LogEntry
	for rows.Next() {
		var e LogEntry
		var runID, source sql.NullString
		if err := rows.Scan(&e.ID, &runID, &e.Level, &source, &e.Message, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.RunID = runID.String
		e.Source = source.String
		out = append(out, e)
	}
	return out, rows.Err()
}

// dbLogWriter tees the existing console/file logger into the log database, so
// every line already being written lands in both places without touching a
// single call site.
type dbLogWriter struct {
	store *LogStore
	mu    sync.RWMutex
	runID string
}

func (w *dbLogWriter) setRun(id string) {
	w.mu.Lock()
	w.runID = id
	w.mu.Unlock()
}

func (w *dbLogWriter) Write(p []byte) (int, error) {
	w.mu.RLock()
	runID := w.runID
	w.mu.RUnlock()

	message := strings.TrimRight(string(p), "\n")
	w.store.Write(runID, levelOf(message), "harvester", message)
	return len(p), nil
}

// levelOf infers a level from the message. The logger has no level concept, and
// inferring one here beats rewriting every call site to gain a field nothing
// currently sets.
func levelOf(message string) string {
	switch {
	case strings.Contains(message, "ERROR"):
		return "error"
	case strings.Contains(message, "WARNING"):
		return "warn"
	case strings.Contains(message, "NOTE"):
		return "notice"
	default:
		return "info"
	}
}

// TrimOldLogs deletes log rows older than maxAge, keeping the newest keepLast
// rows regardless of age. Nulling or deleting frees pages for reuse; a later
// VACUUM (or auto_vacuum) actually shrinks the file.
func (l *LogStore) TrimOldLogs(ctx context.Context, maxAge time.Duration, keepLast int) error {
	if keepLast < 0 {
		keepLast = 0
	}
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339Nano)
	_, err := l.db.ExecContext(ctx,
		`DELETE FROM logs
		   WHERE created_at < ?
		     AND id NOT IN (SELECT id FROM logs ORDER BY created_at DESC LIMIT ?)`,
		cutoff, keepLast)
	return err
}

// Vacuum rewrites the database file to return freed pages to the OS.
func (l *LogStore) Vacuum(ctx context.Context) error {
	_, err := l.db.ExecContext(ctx, "VACUUM")
	return err
}
