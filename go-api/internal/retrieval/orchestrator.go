package retrieval

import (
	"context"
	"errors"
	"sort"
	"sync"
)

type BM25Searcher interface {
	Search(query string, limit int, filters map[string]string) []Document
}

type VectorSearcher interface {
	Search(ctx context.Context, embedding []float32, limit int, filters map[string]string) ([]Document, error)
}

type Synthesizer interface {
	Synthesize(ctx context.Context, query string, docs []Document) (string, error)
	Stream(ctx context.Context, query string, docs []Document) (<-chan string, <-chan error)
}

type Orchestrator struct {
	bm25        BM25Searcher
	vector      VectorSearcher
	ai          *AIClient
	synthesizer Synthesizer
}

func NewOrchestrator(bm25 BM25Searcher, vector VectorSearcher, ai *AIClient, synthesizer Synthesizer) *Orchestrator {
	return &Orchestrator{bm25: bm25, vector: vector, ai: ai, synthesizer: synthesizer}
}

func (o *Orchestrator) Search(ctx context.Context, query string, topK int) (SearchResponse, error) {
	reranked, trace, err := o.Retrieve(ctx, query, topK, nil)
	if err != nil {
		return SearchResponse{}, err
	}
	answer, err := o.synthesizer.Synthesize(ctx, query, reranked)
	if err != nil {
		return SearchResponse{}, err
	}

	return SearchResponse{
		Query:     query,
		Answer:    answer,
		Documents: reranked,
		Trace:     trace,
	}, nil
}

func (o *Orchestrator) Stream(ctx context.Context, query string, topK int) (<-chan string, <-chan error, []Document, Trace, error) {
	reranked, trace, err := o.Retrieve(ctx, query, topK, nil)
	if err != nil {
		return nil, nil, nil, Trace{}, err
	}
	tokens, errs := o.synthesizer.Stream(ctx, query, reranked)
	return tokens, errs, reranked, trace, nil
}

func (o *Orchestrator) SearchWithFilters(ctx context.Context, req SearchRequest) (SearchResponse, error) {
	reranked, trace, err := o.Retrieve(ctx, req.Query, req.TopK, req.Filters)
	if err != nil {
		return SearchResponse{}, err
	}
	answer, err := o.synthesizer.Synthesize(ctx, req.Query, reranked)
	if err != nil {
		return SearchResponse{}, err
	}

	return SearchResponse{
		Query:     req.Query,
		Answer:    answer,
		Documents: reranked,
		Trace:     trace,
	}, nil
}

func (o *Orchestrator) StreamWithFilters(ctx context.Context, req SearchRequest) (<-chan string, <-chan error, []Document, Trace, error) {
	reranked, trace, err := o.Retrieve(ctx, req.Query, req.TopK, req.Filters)
	if err != nil {
		return nil, nil, nil, Trace{}, err
	}
	tokens, errs := o.synthesizer.Stream(ctx, req.Query, reranked)
	return tokens, errs, reranked, trace, nil
}

func (o *Orchestrator) Retrieve(ctx context.Context, query string, topK int, filters map[string]string) ([]Document, Trace, error) {
	if topK <= 0 {
		topK = 5
	}

	embedding, err := o.ai.Embed(query)
	if err != nil {
		return nil, Trace{}, err
	}

	var bm25Docs []Document
	var vectorDocs []Document
	var vectorErr error
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		bm25Docs = o.bm25.Search(query, 25, filters)
	}()

	go func() {
		defer wg.Done()
		vectorDocs, vectorErr = o.vector.Search(ctx, embedding, 25, filters)
	}()

	wg.Wait()
	if vectorErr != nil {
		return nil, Trace{}, vectorErr
	}

	fused := reciprocalRankFusion(bm25Docs, vectorDocs)
	if len(fused) == 0 {
		return nil, Trace{}, errors.New("no retrieval candidates found")
	}
	reranked, err := o.ai.Rerank(query, fused, topK)
	if err != nil {
		return nil, Trace{}, err
	}

	return reranked, Trace{
		BM25Hits:     len(bm25Docs),
		VectorHits:   len(vectorDocs),
		FusedHits:    len(fused),
		RerankedHits: len(reranked),
	}, nil
}

func reciprocalRankFusion(lists ...[]Document) []Document {
	const k = 60.0
	byID := map[string]Document{}
	scores := map[string]float64{}

	for _, docs := range lists {
		for rank, doc := range docs {
			byID[doc.ID] = doc
			scores[doc.ID] += 1.0 / (k + float64(rank+1))
		}
	}

	fused := make([]Document, 0, len(scores))
	for id, score := range scores {
		doc := byID[id]
		doc.Score = score
		fused = append(fused, doc)
	}
	sort.Slice(fused, func(i, j int) bool {
		return fused[i].Score > fused[j].Score
	})
	return fused
}
