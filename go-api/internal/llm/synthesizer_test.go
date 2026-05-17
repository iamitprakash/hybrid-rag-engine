package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

func TestSynthesizerFallsBackWithoutAPIKey(t *testing.T) {
	synth := NewSynthesizer(Config{})
	answer, err := synth.Synthesize(context.Background(), "what is hybrid retrieval?", []retrieval.Document{
		{Title: "Hybrid Retrieval", Content: "Hybrid retrieval combines BM25 and vector search."},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(answer, "Hybrid Retrieval") {
		t.Fatalf("expected local answer to include source title, got %q", answer)
	}
}

func TestSynthesizerCallsOpenAICompatibleEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"grounded answer"}}]}`))
	}))
	defer server.Close()

	synth := NewSynthesizer(Config{BaseURL: server.URL, APIKey: "test-key", Model: "test-model"})
	answer, err := synth.Synthesize(context.Background(), "query", []retrieval.Document{
		{Title: "Doc", Content: "Context"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if answer != "grounded answer" {
		t.Fatalf("unexpected answer: %q", answer)
	}
}
