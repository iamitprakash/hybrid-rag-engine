package chunking

import "testing"

func TestChunkDocumentsPreservesMetadata(t *testing.T) {
	docs := []RawDocument{
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

	chunks := ChunkDocuments(docs, Options{ChunkSize: 4, Overlap: 1})

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
