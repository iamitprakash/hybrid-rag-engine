package evaluationtest

import (
	"context"
	"testing"

	"hybrid-rag-engine/go-api/internal/evaluation"
	"hybrid-rag-engine/go-api/internal/retrieval"
)

type fakeSearcher struct {
	responses map[string]retrieval.SearchResponse
}

func (f fakeSearcher) SearchWithFilters(_ context.Context, req retrieval.SearchRequest) (retrieval.SearchResponse, error) {
	return f.responses[req.Query], nil
}

func TestRunComputesHitRateAndMRR(t *testing.T) {
	report, err := evaluation.Run(context.Background(), fakeSearcher{
		responses: map[string]retrieval.SearchResponse{
			"q1": {Documents: []retrieval.Document{{ID: "doc-1"}}, Grounding: retrieval.GroundingReport{HallucinationRisk: "low"}},
			"q2": {Documents: []retrieval.Document{{ID: "doc-3"}, {ID: "doc-2"}}, Grounding: retrieval.GroundingReport{HallucinationRisk: "medium"}},
		},
	}, evaluation.Request{
		Examples: []evaluation.Example{
			{Query: "q1", ExpectedIDs: []string{"doc-1"}},
			{Query: "q2", ExpectedIDs: []string{"doc-2"}},
		},
		TopK: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Hits != 2 {
		t.Fatalf("expected 2 hits, got %d", report.Hits)
	}
	if report.MRR != 0.75 {
		t.Fatalf("expected MRR 0.75, got %f", report.MRR)
	}
}
