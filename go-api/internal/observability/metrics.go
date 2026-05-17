package observability

import (
	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsBackend struct {
	enabled            bool
	handler            http.Handler
	searchRequests     prometheus.Counter
	searchErrors       prometheus.Counter
	cacheHits          prometheus.Counter
	groundingHighRisk  prometheus.Counter
	searchLatencyMs    prometheus.Histogram
	groundedTokenRatio prometheus.Histogram
}

func NewMetricsBackend() *MetricsBackend {
	if os.Getenv("PROMETHEUS_METRICS_ENABLED") == "" || os.Getenv("PROMETHEUS_METRICS_ENABLED") == "false" {
		return &MetricsBackend{}
	}
	registry := prometheus.NewRegistry()
	factory := promauto.With(registry)
	return &MetricsBackend{
		enabled: true,
		handler: promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		searchRequests: factory.NewCounter(prometheus.CounterOpts{
			Name: "hybrid_rag_search_requests_total",
			Help: "Total number of search requests",
		}),
		searchErrors: factory.NewCounter(prometheus.CounterOpts{
			Name: "hybrid_rag_search_errors_total",
			Help: "Total number of search errors",
		}),
		cacheHits: factory.NewCounter(prometheus.CounterOpts{
			Name: "hybrid_rag_cache_hits_total",
			Help: "Total number of search cache hits",
		}),
		groundingHighRisk: factory.NewCounter(prometheus.CounterOpts{
			Name: "hybrid_rag_grounding_high_risk_total",
			Help: "Total number of high-risk grounding reports",
		}),
		searchLatencyMs: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "hybrid_rag_search_latency_ms",
			Help:    "Search latency in milliseconds",
			Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2000},
		}),
		groundedTokenRatio: factory.NewHistogram(prometheus.HistogramOpts{
			Name:    "hybrid_rag_grounded_token_ratio",
			Help:    "Grounded token ratio per search response",
			Buckets: []float64{0.1, 0.25, 0.5, 0.75, 0.9, 1.0},
		}),
	}
}

func (m *MetricsBackend) Enabled() bool {
	return m != nil && m.enabled
}

func (m *MetricsBackend) Handler() http.Handler {
	return m.handler
}

func (m *MetricsBackend) RecordSearch(latencyMs float64, groundedRatio float64, hallucinationRisk string) {
	if !m.Enabled() {
		return
	}
	m.searchRequests.Inc()
	m.searchLatencyMs.Observe(latencyMs)
	m.groundedTokenRatio.Observe(groundedRatio)
	if hallucinationRisk == "high" {
		m.groundingHighRisk.Inc()
	}
}

func (m *MetricsBackend) RecordError() {
	if m.Enabled() {
		m.searchErrors.Inc()
	}
}

func (m *MetricsBackend) RecordCacheHit() {
	if m.Enabled() {
		m.cacheHits.Inc()
	}
}
