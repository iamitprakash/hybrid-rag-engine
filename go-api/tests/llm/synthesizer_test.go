package llmtest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"hybrid-rag-engine/go-api/internal/llm"
	"hybrid-rag-engine/go-api/internal/retrieval"
)

func TestSynthesizerFallsBackWithoutAPIKey(t *testing.T) {
	synth := llm.NewSynthesizer(llm.Config{})
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

	synth := llm.NewSynthesizer(llm.Config{BaseURL: server.URL, APIKey: "test-key", Model: "test-model"})
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

func TestSynthesizerStreamsRemoteTokens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hello\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer server.Close()

	synth := llm.NewSynthesizer(llm.Config{BaseURL: server.URL, APIKey: "test-key", Model: "test-model"})
	tokens, errs := synth.Stream(context.Background(), "query", []retrieval.Document{{Title: "Doc", Content: "Context"}})

	var builder strings.Builder
	for token := range tokens {
		builder.WriteString(token)
	}
	if err := <-errs; err != nil {
		t.Fatalf("unexpected stream error: %v", err)
	}
	if builder.String() != "hello world" {
		t.Fatalf("unexpected token stream: %q", builder.String())
	}
}
