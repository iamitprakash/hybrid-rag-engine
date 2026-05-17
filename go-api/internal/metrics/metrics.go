package metrics

import (
	"sync"
	"time"
)

type Snapshot struct {
	SearchRequests        int64   `json:"search_requests"`
	SearchErrors          int64   `json:"search_errors"`
	CacheHits             int64   `json:"cache_hits"`
	AverageLatencyMs      float64 `json:"average_latency_ms"`
	AverageGroundedRatio  float64 `json:"average_grounded_ratio"`
	HighRiskGroundingHits int64   `json:"high_risk_grounding_hits"`
}

type Recorder struct {
	mu                   sync.Mutex
	searchRequests       int64
	searchErrors         int64
	cacheHits            int64
	totalLatency         time.Duration
	totalGroundedRatio   float64
	groundingReportCount int64
	highRiskGrounding    int64
}

func NewRecorder() *Recorder {
	return &Recorder{}
}

func (r *Recorder) RecordSearch(latency time.Duration, groundedRatio float64, hallucinationRisk string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.searchRequests++
	r.totalLatency += latency
	r.totalGroundedRatio += groundedRatio
	r.groundingReportCount++
	if hallucinationRisk == "high" {
		r.highRiskGrounding++
	}
}

func (r *Recorder) RecordError() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.searchErrors++
}

func (r *Recorder) RecordCacheHit() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cacheHits++
}

func (r *Recorder) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()

	var avgLatency float64
	if r.searchRequests > 0 {
		avgLatency = float64(r.totalLatency.Milliseconds()) / float64(r.searchRequests)
	}
	var avgGroundedRatio float64
	if r.groundingReportCount > 0 {
		avgGroundedRatio = r.totalGroundedRatio / float64(r.groundingReportCount)
	}
	return Snapshot{
		SearchRequests:        r.searchRequests,
		SearchErrors:          r.searchErrors,
		CacheHits:             r.cacheHits,
		AverageLatencyMs:      avgLatency,
		AverageGroundedRatio:  avgGroundedRatio,
		HighRiskGroundingHits: r.highRiskGrounding,
	}
}
