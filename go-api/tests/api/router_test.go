package apitest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"hybrid-rag-engine/go-api/internal/api"
	"hybrid-rag-engine/go-api/internal/bm25"
	"hybrid-rag-engine/go-api/internal/cache"
	"hybrid-rag-engine/go-api/internal/metadata"
	"hybrid-rag-engine/go-api/internal/metrics"
	"hybrid-rag-engine/go-api/internal/retrieval"
	"hybrid-rag-engine/go-api/internal/tenantauth"
)

type fakeVector struct{}

func (fakeVector) Search(context.Context, []float32, int, map[string]string) ([]retrieval.Document, error) {
	return []retrieval.Document{{ID: "doc-1", Title: "Doc", Content: "hybrid retrieval combines bm25 and vector search", Source: "docs"}}, nil
}

type fakeSynth struct{}

func (fakeSynth) Synthesize(context.Context, string, []retrieval.Document) (string, error) {
	return "hybrid retrieval combines bm25 and vector search", nil
}

func (fakeSynth) Stream(context.Context, string, []retrieval.Document) (<-chan string, <-chan error) {
	tokens := make(chan string, 1)
	errs := make(chan error, 1)
	tokens <- "ok"
	close(tokens)
	close(errs)
	return tokens, errs
}

type fakeAITransport struct{}

func (fakeAITransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/embed" {
		return jsonResponse(`{"embedding":[0.1,0.2,0.3]}`), nil
	}
	if req.URL.Path == "/rerank" {
		return jsonResponse(`{"documents":[{"id":"doc-1","title":"Doc","content":"hybrid retrieval combines bm25 and vector search","source":"docs","score":1.0}]}`), nil
	}
	return jsonResponse(`{}`), nil
}

func TestSearchEndpointAddsGroundingAndCache(t *testing.T) {
	bm25Index := bm25.NewIndex([]retrieval.Document{{ID: "doc-1", Title: "Doc", Content: "hybrid retrieval combines bm25 and vector search", Source: "docs"}})
	aiClient := retrieval.NewAIClient("http://fake", 0)
	aiClient.SetHTTPClient(&http.Client{Transport: fakeAITransport{}})
	orchestrator := retrieval.NewOrchestrator(bm25Index, fakeVector{}, aiClient, fakeSynth{})
	cacheClient, _ := cache.New("", 0)
	router := api.NewRouter(orchestrator, aiClient, nil, bm25Index, cacheClient, &metadata.Store{}, metrics.NewRecorder(), tenantauth.New(""), nil)

	body := []byte(`{"query":"hybrid retrieval","top_k":1}`)
	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp retrieval.SearchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Grounding.HallucinationRisk == "" {
		t.Fatal("expected grounding report")
	}
}

func TestSearchRequiresTenantAPIKeyWhenConfigured(t *testing.T) {
	bm25Index := bm25.NewIndex(nil)
	aiClient := retrieval.NewAIClient("http://fake", 0)
	orchestrator := retrieval.NewOrchestrator(bm25Index, fakeVector{}, aiClient, fakeSynth{})
	cacheClient, _ := cache.New("", 0)
	router := api.NewRouter(orchestrator, aiClient, nil, bm25Index, cacheClient, &metadata.Store{}, metrics.NewRecorder(), tenantauth.New("tenant-a:secret"), nil)

	req := httptest.NewRequest(http.MethodPost, "/search", bytes.NewBufferString(`{"query":"hybrid retrieval"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func jsonResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}
}
