package main

import (
	"context"
	"fmt"
	"time"
)

// Job status values.
const (
	jobPending = "pending"
	jobRunning = "running"
	jobDone    = "done"
	jobFailed  = "failed"
)

// Job is one unit of background work claimed from the jobs table.
type Job struct {
	ID       string
	Type     string
	Payload  string
	Status   string
	Attempts int
}

// EnqueueJob adds a pending job to run as soon as a worker is free.
func (s *Store) EnqueueJob(ctx context.Context, jobType, payload string) (string, error) {
	id := newID()
	now := nowRFC3339()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO jobs (id, type, payload, status, attempts, run_after, created_at, updated_at)
		 VALUES (?, ?, ?, 'pending', 0, ?, ?, ?)`,
		id, jobType, payload, now, now, now); err != nil {
		return "", fmt.Errorf("enqueue %s: %w", jobType, err)
	}
	return id, nil
}

// ClaimJobs marks up to limit runnable jobs as running and returns them. A job
// is runnable when it is pending and its run_after has passed. The claim UPDATE
// guards on the pending status so two workers cannot take the same row even with
// several open connections.
func (s *Store) ClaimJobs(ctx context.Context, limit int) ([]Job, error) {
	if limit <= 0 {
		limit = 1
	}
	now := nowRFC3339()
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, type, payload, attempts FROM jobs
		  WHERE status = 'pending' AND run_after <= ?
		  ORDER BY run_after LIMIT ?`, now, limit)
	if err != nil {
		return nil, err
	}
	var candidates []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Type, &j.Payload, &j.Attempts); err != nil {
			rows.Close()
			return nil, err
		}
		candidates = append(candidates, j)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var claimed []Job
	for _, j := range candidates {
		res, err := s.db.ExecContext(ctx,
			`UPDATE jobs SET status = 'running', locked_at = ?, updated_at = ?
			  WHERE id = ? AND status = 'pending'`, now, now, j.ID)
		if err != nil {
			return nil, err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			j.Status = jobRunning
			claimed = append(claimed, j)
		}
	}
	return claimed, nil
}

// CompleteJob marks a job done.
func (s *Store) CompleteJob(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'done', locked_at = NULL, updated_at = ? WHERE id = ?`,
		nowRFC3339(), id)
	return err
}

// RetryJob returns a job to pending with an incremented attempt count and a
// delayed run_after (backoff).
func (s *Store) RetryJob(ctx context.Context, id string, attempts int, backoff time.Duration) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'pending', attempts = ?, run_after = ?, locked_at = NULL, updated_at = ?
		  WHERE id = ?`,
		attempts, now.Add(backoff).Format(time.RFC3339Nano), now.Format(time.RFC3339Nano), id)
	return err
}

// FailJob marks a job permanently failed after its attempts are exhausted.
func (s *Store) FailJob(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'failed', locked_at = NULL, updated_at = ? WHERE id = ?`,
		nowRFC3339(), id)
	return err
}

// ResetStaleJobs returns running jobs whose lock is older than the cutoff back
// to pending. This is the crash-recovery path: a process that died mid-job left
// rows marked running that nothing else will ever finish.
func (s *Store) ResetStaleJobs(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().UTC().Add(-olderThan).Format(time.RFC3339Nano)
	res, err := s.db.ExecContext(ctx,
		`UPDATE jobs SET status = 'pending', locked_at = NULL, updated_at = ?
		  WHERE status = 'running' AND locked_at IS NOT NULL AND locked_at < ?`,
		nowRFC3339(), cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
