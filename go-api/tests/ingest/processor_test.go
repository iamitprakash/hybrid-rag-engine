package ingesttest

import (
	"context"
	"fmt"
	"testing"

	"hybrid-rag-engine/go-api/internal/ingest"
	"hybrid-rag-engine/go-api/internal/retrieval"
)

type fakeEmbedder struct{}

func (fakeEmbedder) Embed(text string) ([]float32, error) {
	return []float32{float32(len(text))}, nil
}

type fakeVectorWriter struct {
	ensureCalls int
	upserts     [][]retrieval.Document
}

func (f *fakeVectorWriter) EnsureCollection(context.Context) error {
	f.ensureCalls++
	return nil
}

func (f *fakeVectorWriter) Upsert(_ context.Context, docs []retrieval.Document, vectors [][]float32) error {
	if len(docs) != len(vectors) {
		return fmt.Errorf("mismatched docs and vectors")
	}
	cloned := make([]retrieval.Document, len(docs))
	copy(cloned, docs)
	f.upserts = append(f.upserts, cloned)
	return nil
}

func TestProcessorBatchesAndUpserts(t *testing.T) {
	writer := &fakeVectorWriter{}
	processor := ingest.NewProcessor(fakeEmbedder{}, writer, 2, 2)
	docs := []retrieval.Document{
		{ID: "1", Title: "A", Content: "a"},
		{ID: "2", Title: "B", Content: "b"},
		{ID: "3", Title: "C", Content: "c"},
	}

	if err := processor.Process(context.Background(), docs); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if writer.ensureCalls != 1 {
		t.Fatalf("expected one ensure call, got %d", writer.ensureCalls)
	}
	if len(writer.upserts) != 2 {
		t.Fatalf("expected two upsert batches, got %d", len(writer.upserts))
	}
	if len(writer.upserts[0]) != 2 || len(writer.upserts[1]) != 1 {
		t.Fatalf("unexpected batch sizes: %d and %d", len(writer.upserts[0]), len(writer.upserts[1]))
	}
}
