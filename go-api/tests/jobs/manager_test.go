package jobstest

import (
	"context"
	"testing"
	"time"

	"hybrid-rag-engine/go-api/internal/jobs"
)

func TestManagerCompletesJob(t *testing.T) {
	manager := jobs.NewManager()
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
