package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The seeded queue: one pending distill, one failed embed (3 attempts), one
// done embed, one running cleanup.
func TestListJobsReturnsTheWholeQueue(t *testing.T) {
	env := seededEnv(t)

	jobs, err := env.Store.ListJobs(context.Background(), "", "", 0, 0)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(jobs) != 4 {
		t.Fatalf("jobs = %d, want 4", len(jobs))
	}

	byID := map[string]JobSummary{}
	for _, j := range jobs {
		byID[j.ID] = j
	}
	failed, ok := byID["00000000-0000-7000-8000-00000000000e"]
	if !ok {
		t.Fatal("the failed job is missing")
	}
	if failed.Status != jobFailed || failed.Attempts != 3 || failed.Type != jobTypeEmbed {
		t.Errorf("failed job = %+v", failed)
	}
	if failed.Payload == "" {
		t.Error("payload should be carried through for display")
	}
	running := byID["00000000-0000-7000-8000-000000000010"]
	if running.LockedAt == "" {
		t.Error("a running job should report locked_at")
	}
	if byID["00000000-0000-7000-8000-00000000000b"].RunAfter == "" {
		t.Error("run_after (the backoff) should be reported")
	}
}

func TestListJobsFiltersByStatusAndType(t *testing.T) {
	env := seededEnv(t)
	ctx := context.Background()

	pending, err := env.Store.ListJobs(ctx, jobPending, "", 0, 0)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(pending) != 1 || pending[0].Type != jobTypeDistill {
		t.Errorf("pending = %+v, want the one distill job", pending)
	}

	embeds, err := env.Store.ListJobs(ctx, "", jobTypeEmbed, 0, 0)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(embeds) != 2 {
		t.Errorf("embed jobs = %d, want 2", len(embeds))
	}

	both, err := env.Store.ListJobs(ctx, jobDone, jobTypeEmbed, 0, 0)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(both) != 1 || both[0].Status != jobDone {
		t.Errorf("done embeds = %+v, want exactly one", both)
	}

	none, err := env.Store.ListJobs(ctx, "no-such-status", "", 0, 0)
	if err != nil {
		t.Fatalf("an unmatched filter must not error: %v", err)
	}
	if len(none) != 0 {
		t.Errorf("unmatched filter returned %d rows", len(none))
	}
}

func TestListJobsPaginates(t *testing.T) {
	env := seededEnv(t)
	ctx := context.Background()

	first, err := env.Store.ListJobs(ctx, "", "", 2, 0)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	second, err := env.Store.ListJobs(ctx, "", "", 2, 2)
	if err != nil {
		t.Fatalf("ListJobs: %v", err)
	}
	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("pages = %d and %d, want 2 each", len(first), len(second))
	}
	for _, a := range first {
		for _, b := range second {
			if a.ID == b.ID {
				t.Errorf("job %s appears on both pages", a.ID)
			}
		}
	}

	// An out-of-range offset is an empty page, not an error.
	past, err := env.Store.ListJobs(ctx, "", "", 2, 500)
	if err != nil || len(past) != 0 {
		t.Errorf("offset past the end: %d rows, err %v", len(past), err)
	}
}

func TestListJobsEmptyQueue(t *testing.T) {
	_, env := newTestServer(t) // unseeded

	jobs, err := env.Store.ListJobs(context.Background(), "", "", 0, 0)
	if err != nil {
		t.Fatalf("an empty queue must not error: %v", err)
	}
	if len(jobs) != 0 {
		t.Errorf("jobs = %d, want none", len(jobs))
	}
}

func TestJobStatusCountsCoverEveryStatus(t *testing.T) {
	env := seededEnv(t)

	counts, err := env.Store.JobStatusCounts(context.Background())
	if err != nil {
		t.Fatalf("JobStatusCounts: %v", err)
	}
	want := map[string]int{jobPending: 1, jobRunning: 1, jobDone: 1, jobFailed: 1}
	for status, n := range want {
		if counts[status] != n {
			t.Errorf("counts[%s] = %d, want %d", status, counts[status], n)
		}
	}
	// A status nothing holds must still be present, so the UI's breakdown does
	// not gain and lose tiles as the queue drains.
	if len(counts) != len(jobStatuses) {
		t.Errorf("counts has %d keys, want one per known status", len(counts))
	}
}

// ---- endpoint level ----

func TestJobsEndpoint(t *testing.T) {
	env := seededEnv(t)
	srv := env.Server()

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/jobs?status=failed", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var page JobsPage
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(page.Jobs) != 1 || page.Jobs[0].Status != jobFailed {
		t.Errorf("jobs = %+v, want the one failed job", page.Jobs)
	}
	// The counts describe the whole queue, not the filtered page.
	if page.Counts[jobPending] != 1 || page.Counts[jobDone] != 1 {
		t.Errorf("counts = %v, want the unfiltered queue", page.Counts)
	}
}

func TestJobsEndpointEmptyQueue(t *testing.T) {
	srv, _ := newTestServer(t)

	rec := httptest.NewRecorder()
	srv.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/jobs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var page JobsPage
	if err := json.NewDecoder(rec.Body).Decode(&page); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	if len(page.Jobs) != 0 {
		t.Errorf("jobs = %+v, want none", page.Jobs)
	}
	if page.Counts[jobPending] != 0 {
		t.Errorf("pending count = %d, want 0", page.Counts[jobPending])
	}
}
