package quality

import (
	"testing"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

func TestEvaluateFlagsUnsupportedTerms(t *testing.T) {
	report := Evaluate("BM25 retrieval uses bananas and rockets", []retrieval.Document{
		{Title: "BM25", Content: "BM25 retrieval uses lexical matching"},
	})

	if report.HallucinationRisk == "low" {
		t.Fatal("expected unsupported terms to raise hallucination risk")
	}
	if len(report.UnsupportedTerms) == 0 {
		t.Fatal("expected unsupported terms")
	}
}
