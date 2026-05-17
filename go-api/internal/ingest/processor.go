package ingest

import (
	"context"
	"sync"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

type Embedder interface {
	Embed(text string) ([]float32, error)
}

type VectorWriter interface {
	EnsureCollection(ctx context.Context) error
	Upsert(ctx context.Context, docs []retrieval.Document, vectors [][]float32) error
}

type Processor struct {
	embedder  Embedder
	vector    VectorWriter
	batchSize int
	workers   int
}

func NewProcessor(embedder Embedder, vector VectorWriter, batchSize int, workers int) *Processor {
	if batchSize <= 0 {
		batchSize = 16
	}
	if workers <= 0 {
		workers = 4
	}
	return &Processor{
		embedder:  embedder,
		vector:    vector,
		batchSize: batchSize,
		workers:   workers,
	}
}

func (p *Processor) Process(ctx context.Context, docs []retrieval.Document) error {
	if len(docs) == 0 {
		return nil
	}
	if err := p.vector.EnsureCollection(ctx); err != nil {
		return err
	}
	for start := 0; start < len(docs); start += p.batchSize {
		end := start + p.batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[start:end]
		vectors, err := p.embedBatch(ctx, batch)
		if err != nil {
			return err
		}
		if err := p.vector.Upsert(ctx, batch, vectors); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) embedBatch(ctx context.Context, docs []retrieval.Document) ([][]float32, error) {
	type result struct {
		index  int
		vector []float32
		err    error
	}

	results := make([][]float32, len(docs))
	out := make(chan result, len(docs))
	sem := make(chan struct{}, p.workers)
	var wg sync.WaitGroup

	for index, doc := range docs {
		wg.Add(1)
		go func(index int, doc retrieval.Document) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				out <- result{index: index, err: ctx.Err()}
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			vector, err := p.embedder.Embed(doc.Title + "\n" + doc.Content)
			out <- result{index: index, vector: vector, err: err}
		}(index, doc)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	for result := range out {
		if result.err != nil {
			return nil, result.err
		}
		results[result.index] = result.vector
	}
	return results, nil
}
