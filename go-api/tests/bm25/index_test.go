package bm25test

import (
	"testing"

	"hybrid-rag-engine/go-api/internal/bm25"
	"hybrid-rag-engine/go-api/internal/retrieval"
)

func TestAddDocumentsUpdatesSearchIndex(t *testing.T) {
	idx := bm25.NewIndex([]retrieval.Document{
		{ID: "a", Title: "Vector Search", Content: "semantic retrieval with embeddings"},
	})

	idx.AddDocuments([]retrieval.Document{
		{ID: "b", Title: "Lexical Search", Content: "bm25 keyword retrieval"},
	})

	results := idx.Search("bm25 keyword", 5, nil)
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].ID != "b" {
		t.Fatalf("expected appended document to rank first, got %s", results[0].ID)
	}
}

func TestSearchFiltersByMetadataAndSource(t *testing.T) {
	idx := bm25.NewIndex([]retrieval.Document{
		{ID: "a", Title: "Tenant A", Content: "hybrid retrieval", Source: "alpha", Metadata: map[string]string{"tenant": "a"}},
		{ID: "b", Title: "Tenant B", Content: "hybrid retrieval", Source: "docs", Metadata: map[string]string{"tenant": "b"}},
	})

	results := idx.Search("hybrid retrieval", 5, map[string]string{"tenant": "b", "source": "docs"})

	if len(results) != 1 {
		t.Fatalf("expected one filtered result, got %d", len(results))
	}
	if results[0].ID != "b" {
		t.Fatalf("expected tenant b document, got %s", results[0].ID)
	}
}
