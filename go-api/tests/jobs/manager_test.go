package jobstest

import (
	"context"
	"testing"
	"time"

	"hybrid-rag-engine/go-api/internal/jobs"
)

func TestManagerCompletesJob(t *testing.T) {
	manager := jobs.NewManager(nil)
	job := manager.Start(context.Background(), "ingest", "tenant-a", func(context.Context) (jobs.IngestResult, error) {
		return jobs.IngestResult{Documents: 1, Chunks: 2}, nil
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		current, ok := manager.Get(job.ID)
		if !ok {
			t.Fatal("expected job to exist")
		}
		if current.Status == jobs.StatusCompleted {
			if current.Result.Chunks != 2 {
				t.Fatalf("unexpected result: %#v", current.Result)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not complete in time")
}

func TestManagerListsRecentJobs(t *testing.T) {
	manager := jobs.NewManager(nil)
	first := manager.Start(context.Background(), "ingest", "tenant-a", func(context.Context) (jobs.IngestResult, error) {
		return jobs.IngestResult{}, nil
	})
	second := manager.Start(context.Background(), "ingest", "tenant-b", func(context.Context) (jobs.IngestResult, error) {
		return jobs.IngestResult{}, nil
	})

	list, err := manager.ListRecent(context.Background(), "tenant-a", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one tenant-scoped job, got %d", len(list))
	}
	if list[0].ID != first.ID && list[0].ID != second.ID {
		t.Fatalf("unexpected job id: %s", list[0].ID)
	}
	if list[0].Tenant != "tenant-a" {
		t.Fatalf("expected tenant-a job, got %s", list[0].Tenant)
	}
}
