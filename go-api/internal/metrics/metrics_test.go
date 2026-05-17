package metrics

import (
	"testing"
	"time"
)

func TestRecorderSnapshot(t *testing.T) {
	recorder := NewRecorder()
	recorder.RecordSearch(100*time.Millisecond, 0.5, "high")
	recorder.RecordCacheHit()

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
}
