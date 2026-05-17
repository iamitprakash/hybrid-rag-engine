package metricstest

import (
	"testing"
	"time"

	"hybrid-rag-engine/go-api/internal/metrics"
)

func TestRecorderSnapshot(t *testing.T) {
	recorder := metrics.NewRecorder()
	recorder.RecordSearch(100*time.Millisecond, 0.5, "high")
	recorder.RecordCacheHit()
	recorder.RecordError()

	snapshot := recorder.Snapshot()
	if snapshot.SearchRequests != 1 {
		t.Fatalf("expected one request, got %d", snapshot.SearchRequests)
	}
	if snapshot.CacheHits != 1 {
		t.Fatalf("expected one cache hit, got %d", snapshot.CacheHits)
	}
	if snapshot.HighRiskGroundingHits != 1 {
		t.Fatalf("expected one high risk hit, got %d", snapshot.HighRiskGroundingHits)
	}
	if snapshot.SearchErrors != 1 {
		t.Fatalf("expected one search error, got %d", snapshot.SearchErrors)
	}
	if snapshot.AverageLatencyMs <= 0 {
		t.Fatalf("expected positive latency average, got %f", snapshot.AverageLatencyMs)
	}
}
