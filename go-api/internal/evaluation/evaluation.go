package evaluation

import (
	"context"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

type Example struct {
	Name        string            `json:"name,omitempty"`
	Query       string            `json:"query"`
	Filters     map[string]string `json:"filters,omitempty"`
	ExpectedIDs []string          `json:"expected_ids"`
}

type Request struct {
	Examples []Example `json:"examples"`
	TopK     int       `json:"top_k"`
}

type QueryReport struct {
	Name           string   `json:"name,omitempty"`
	Query          string   `json:"query"`
	ExpectedIDs    []string `json:"expected_ids"`
	ReturnedIDs    []string `json:"returned_ids"`
	Hit            bool     `json:"hit"`
	ReciprocalRank float64  `json:"reciprocal_rank"`
	GroundingRisk  string   `json:"grounding_risk,omitempty"`
}

type Report struct {
	Queries      int           `json:"queries"`
	Hits         int           `json:"hits"`
	HitRate      float64       `json:"hit_rate"`
	MRR          float64       `json:"mrr"`
	QueryReports []QueryReport `json:"query_reports"`
}

type Searcher interface {
	SearchWithFilters(ctx context.Context, req retrieval.SearchRequest) (retrieval.SearchResponse, error)
}

func Run(ctx context.Context, searcher Searcher, req Request) (Report, error) {
	topK := req.TopK
	if topK <= 0 {
		topK = 5
	}

	report := Report{Queries: len(req.Examples)}
	for _, example := range req.Examples {
		resp, err := searcher.SearchWithFilters(ctx, retrieval.SearchRequest{
			Query:   example.Query,
			TopK:    topK,
			Filters: example.Filters,
		})
		if err != nil {
			return Report{}, err
		}
		queryReport := QueryReport{
			Name:          example.Name,
			Query:         example.Query,
			ExpectedIDs:   example.ExpectedIDs,
			ReturnedIDs:   collectIDs(resp.Documents),
			GroundingRisk: resp.Grounding.HallucinationRisk,
		}
		queryReport.Hit, queryReport.ReciprocalRank = score(example.ExpectedIDs, queryReport.ReturnedIDs)
		if queryReport.Hit {
			report.Hits++
		}
		report.MRR += queryReport.ReciprocalRank
		report.QueryReports = append(report.QueryReports, queryReport)
	}
	if report.Queries > 0 {
		report.HitRate = float64(report.Hits) / float64(report.Queries)
		report.MRR = report.MRR / float64(report.Queries)
	}
	return report, nil
}

func collectIDs(docs []retrieval.Document) []string {
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		ids = append(ids, doc.ID)
	}
	return ids
}

func score(expected []string, returned []string) (bool, float64) {
	expectedSet := map[string]bool{}
	for _, id := range expected {
		expectedSet[id] = true
	}
	for index, id := range returned {
		if expectedSet[id] {
			return true, 1.0 / float64(index+1)
		}
	}
	return false, 0
}
