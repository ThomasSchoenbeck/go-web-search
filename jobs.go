package main

import (
	"context"
	"log"
	"sync"
	"time"
)

// JobHandler processes one job's payload. Returning an error triggers retry with
// backoff up to maxAttempts, after which the job is marked failed.
type JobHandler func(ctx context.Context, payload string) error

type recurringJob struct {
	jobType  string
	payload  string
	interval time.Duration
}

// JobRunner polls the jobs table and runs claimed jobs on a worker pool. It is
// the single place background work is registered and managed: handlers are
// registered by type, and recurring work is registered as a scheduled enqueue.
type JobRunner struct {
	store       *Store
	log         *log.Logger
	workers     int
	poll        time.Duration
	maxAttempts int
	staleAfter  time.Duration

	handlers  map[string]JobHandler
	recurring []recurringJob
}

func newJobRunner(store *Store, logger *log.Logger, workers int, poll time.Duration) *JobRunner {
	if workers < 1 {
		workers = 1
	}
	if poll <= 0 {
		poll = time.Second
	}
	return &JobRunner{
		store:       store,
		log:         logger,
		workers:     workers,
		poll:        poll,
		maxAttempts: 5,
		staleAfter:  5 * time.Minute,
		handlers:    make(map[string]JobHandler),
	}
}

// Register wires a handler for a job type. Call before Start.
func (r *JobRunner) Register(jobType string, h JobHandler) {
	r.handlers[jobType] = h
}

// RegisterRecurring schedules a job of the given type to be enqueued every
// interval. The work still flows through the pool, so it inherits retry and
// crash recovery. Call before Start.
func (r *JobRunner) RegisterRecurring(jobType, payload string, interval time.Duration) {
	if interval <= 0 {
		return
	}
	r.recurring = append(r.recurring, recurringJob{jobType: jobType, payload: payload, interval: interval})
}

// Start launches the worker pool, the poller, the reaper and the recurring
// schedulers. They all stop when ctx is cancelled. It also runs the reaper once
// synchronously so a previous crash is recovered before the first poll.
func (r *JobRunner) Start(ctx context.Context) {
	if n, err := r.store.ResetStaleJobs(ctx, r.staleAfter); err != nil {
		r.log.Printf("job reaper (startup): %v", err)
	} else if n > 0 {
		r.log.Printf("job reaper: reset %d stale job(s) to pending", n)
	}

	jobs := make(chan Job)

	var wg sync.WaitGroup
	for i := 0; i < r.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				r.run(ctx, j)
			}
		}()
	}

	go r.reapLoop(ctx)
	for _, rec := range r.recurring {
		go r.recurLoop(ctx, rec)
	}

	go func() {
		ticker := time.NewTicker(r.poll)
		defer ticker.Stop()
		defer close(jobs)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				claimed, err := r.store.ClaimJobs(ctx, r.workers)
				if err != nil {
					r.log.Printf("job claim: %v", err)
					continue
				}
				for _, j := range claimed {
					select {
					case jobs <- j:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
}

func (r *JobRunner) run(ctx context.Context, j Job) {
	h, ok := r.handlers[j.Type]
	if !ok {
		r.log.Printf("job %s: no handler for type %q, marking failed", j.ID, j.Type)
		r.store.FailJob(ctx, j.ID)
		return
	}
	if err := h(ctx, j.Payload); err == nil {
		if err := r.store.CompleteJob(ctx, j.ID); err != nil {
			r.log.Printf("job %s: complete: %v", j.ID, err)
		}
		return
	} else {
		attempts := j.Attempts + 1
		if attempts >= r.maxAttempts {
			r.log.Printf("job %s (%s): failed after %d attempts: %v", j.ID, j.Type, attempts, err)
			r.store.FailJob(ctx, j.ID)
			return
		}
		backoff := time.Duration(attempts*attempts) * time.Second
		r.log.Printf("job %s (%s): attempt %d failed, retrying in %s: %v", j.ID, j.Type, attempts, backoff, err)
		if err := r.store.RetryJob(ctx, j.ID, attempts, backoff); err != nil {
			r.log.Printf("job %s: retry: %v", j.ID, err)
		}
	}
}

func (r *JobRunner) reapLoop(ctx context.Context) {
	ticker := time.NewTicker(r.staleAfter)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n, err := r.store.ResetStaleJobs(ctx, r.staleAfter); err != nil {
				r.log.Printf("job reaper: %v", err)
			} else if n > 0 {
				r.log.Printf("job reaper: reset %d stale job(s)", n)
			}
		}
	}
}

func (r *JobRunner) recurLoop(ctx context.Context, rec recurringJob) {
	ticker := time.NewTicker(rec.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := r.store.EnqueueJob(ctx, rec.jobType, rec.payload); err != nil {
				r.log.Printf("recurring %s: %v", rec.jobType, err)
			}
		}
	}
}
