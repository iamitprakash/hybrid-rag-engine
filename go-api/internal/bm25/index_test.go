package bm25

import (
	"testing"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

func TestAddDocumentsUpdatesSearchIndex(t *testing.T) {
	idx := NewIndex([]retrieval.Document{
		{ID: "a", Title: "Vector Search", Content: "semantic retrieval with embeddings"},
	})

	idx.AddDocuments([]retrieval.Document{
		{ID: "b", Title: "Lexical Search", Content: "bm25 keyword retrieval"},
	})

	results := idx.Search("bm25 keyword", 5)
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
	if results[0].ID != "b" {
		t.Fatalf("expected appended document to rank first, got %s", results[0].ID)
	}
}
