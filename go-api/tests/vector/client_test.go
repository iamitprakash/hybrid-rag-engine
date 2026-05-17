package vectortest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hybrid-rag-engine/go-api/internal/retrieval"
	"hybrid-rag-engine/go-api/internal/vector"
)

func TestSearchSendsFilterPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/collections/docs/points/search" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		filter, ok := payload["filter"].(map[string]any)
		if !ok {
			t.Fatalf("expected filter in payload: %#v", payload)
		}
		must, ok := filter["must"].([]any)
		if !ok || len(must) != 2 {
			t.Fatalf("expected two filter clauses, got %#v", filter["must"])
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":[{"score":0.9,"payload":{"id":"doc-1","title":"Doc","content":"Context","source":"docs","metadata":{"tenant":"demo"}}}]}`))
	}))
	defer server.Close()

	client := vector.NewClient(server.URL, "docs", 384)
	results, err := client.Search(context.Background(), []float32{0.1, 0.2}, 5, map[string]string{"tenant": "demo", "source": "docs"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].ID != "doc-1" {
		t.Fatalf("unexpected results: %#v", results)
	}
}

func TestUpsertRejectsMismatchedLengths(t *testing.T) {
	client := vector.NewClient("http://example.com", "docs", 384)
	err := client.Upsert(context.Background(), []retrieval.Document{{ID: "doc-1"}}, nil)
	if err == nil {
		t.Fatal("expected length mismatch error")
	}
}
