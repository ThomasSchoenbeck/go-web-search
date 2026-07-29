package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// artifacts owns the output directory for a single execution.
type artifacts struct {
	Dir  string
	Log  *log.Logger
	file *os.File
}

// newArtifacts creates baseDir/run-<timestamp>-<random>/ with a raw/ subfolder
// and opens workflow_results.log, which mirrors everything to the console.
func newArtifacts(baseDir string, extra ...io.Writer) (*artifacts, error) {
	id, err := randomID(4)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(baseDir, fmt.Sprintf("run-%s-%s", time.Now().Format("20060102-150405"), id))
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(dir, "workflow_results.log"))
	if err != nil {
		return nil, err
	}
	writers := append([]io.Writer{os.Stdout, f}, extra...)
	return &artifacts{
		Dir:  dir,
		Log:  log.New(io.MultiWriter(writers...), "", log.LstdFlags),
		file: f,
	}, nil
}

func (a *artifacts) Path(parts ...string) string {
	return filepath.Join(append([]string{a.Dir}, parts...)...)
}

func (a *artifacts) Close() error { return a.file.Close() }

func randomID(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
