package chunkingtest

import (
	"testing"

	"hybrid-rag-engine/go-api/internal/chunking"
)

func TestChunkDocumentsPreservesMetadata(t *testing.T) {
	docs := []chunking.RawDocument{
		{
			ID:      "doc-1",
			Title:   "Hybrid Search",
			Content: "one two three four five six seven eight nine ten",
			Source:  "unit-test",
			Metadata: map[string]string{
				"tenant": "demo",
			},
		},
	}

	chunks := chunking.ChunkDocuments(docs, chunking.Options{ChunkSize: 4, Overlap: 1})

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[0].ID != "doc-1-chunk-0000" {
		t.Fatalf("unexpected chunk id: %s", chunks[0].ID)
	}
	if chunks[1].Metadata["chunk_start_word"] != "3" {
		t.Fatalf("expected overlap-aware start word, got %s", chunks[1].Metadata["chunk_start_word"])
	}
	if chunks[0].Metadata["tenant"] != "demo" {
		t.Fatal("expected source metadata to be preserved")
	}
}

func TestChunkDocumentsGeneratesStableDocumentIDWhenMissing(t *testing.T) {
	docs := []chunking.RawDocument{{Title: "A", Content: "one two three four"}}
	first := chunking.ChunkDocuments(docs, chunking.Options{ChunkSize: 2, Overlap: 5})
	second := chunking.ChunkDocuments(docs, chunking.Options{ChunkSize: 2, Overlap: 5})

	if len(first) != 2 || len(second) != 2 {
		t.Fatalf("expected two chunks, got %d and %d", len(first), len(second))
	}
	if first[0].ID != second[0].ID {
		t.Fatalf("expected stable generated ids, got %s and %s", first[0].ID, second[0].ID)
	}
}
