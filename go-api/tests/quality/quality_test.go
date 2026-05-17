package qualitytest

import (
	"testing"

	"hybrid-rag-engine/go-api/internal/quality"
	"hybrid-rag-engine/go-api/internal/retrieval"
)

func TestEvaluateFlagsUnsupportedTerms(t *testing.T) {
	report := quality.Evaluate("BM25 retrieval uses bananas and rockets", []retrieval.Document{
		{Title: "BM25", Content: "BM25 retrieval uses lexical matching"},
	})

	if report.HallucinationRisk == "low" {
		t.Fatal("expected unsupported terms to raise hallucination risk")
	}
	if len(report.UnsupportedTerms) == 0 {
		t.Fatal("expected unsupported terms")
	}
}

func TestEvaluateReturnsLowRiskForGroundedAnswer(t *testing.T) {
	report := quality.Evaluate("Hybrid retrieval combines BM25 and vector search", []retrieval.Document{
		{Title: "Hybrid Retrieval", Content: "Hybrid retrieval combines BM25 and vector search"},
	})
	if report.HallucinationRisk != "low" {
		t.Fatalf("expected low risk, got %s", report.HallucinationRisk)
	}
	if report.GroundedTokenRatio < 0.7 {
		t.Fatalf("expected high grounded ratio, got %f", report.GroundedTokenRatio)
	}
}
