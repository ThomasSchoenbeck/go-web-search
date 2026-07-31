package main

import (
	"io"
	"log"
	"os"
)

// artifacts owns the process-wide logger. Lines go to the console and to any
// extra writers (the log database); nothing is written to disk as files. Runs
// are recorded only in the database (the runs table), not as folders.
type artifacts struct {
	Log *log.Logger
}

// newArtifacts builds the logger that mirrors to stdout and each extra writer.
func newArtifacts(extra ...io.Writer) (*artifacts, error) {
	writers := append([]io.Writer{os.Stdout}, extra...)
	return &artifacts{
		Log: log.New(io.MultiWriter(writers...), "", log.LstdFlags),
	}, nil
}

// Close is a no-op: there is no log file to flush.
func (a *artifacts) Close() error { return nil }
