package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// dirLock keeps two harvester processes off the same database files.
//
// "One binary, two modes" does not mean one process: nothing stops `-mode
// serve` and `-mode search` running side by side. Multi-process access in the
// Turso engine is recent, so rather than find out how it behaves under a
// concurrent crawl, the second process is refused entry.
type dirLock struct {
	path string
}

func acquireLock(dir string) (*dirLock, error) {
	path := filepath.Join(dir, "harvester.lock")

	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			owner := "unknown process"
			if data, readErr := os.ReadFile(path); readErr == nil {
				if text := strings.TrimSpace(string(data)); text != "" {
					owner = text
				}
			}
			return nil, fmt.Errorf("data directory is locked by %s\n"+
				"if no harvester is running, delete %s and retry", owner, path)
		}
		return nil, fmt.Errorf("creating lock file: %w", err)
	}

	fmt.Fprintf(f, "pid %d, started %s\n", os.Getpid(), nowRFC3339())
	if err := f.Close(); err != nil {
		return nil, err
	}
	return &dirLock{path: path}, nil
}

// release removes the lock. A crash leaves it behind on purpose: a stale lock
// with a pid in it is easier to reason about than silent concurrent access.
func (l *dirLock) release() {
	os.Remove(l.path)
}
