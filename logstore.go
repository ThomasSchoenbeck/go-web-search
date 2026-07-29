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
	db, err := sql.Open(driver, path)
	if err != nil {
		return nil, fmt.Errorf("opening log db %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to log db %s: %w", path, err)
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
