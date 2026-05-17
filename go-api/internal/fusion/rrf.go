package fusion

import (
	"sort"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

func ReciprocalRankFusion(lists ...[]retrieval.Document) []retrieval.Document {
	const k = 60.0
	byID := map[string]retrieval.Document{}
	scores := map[string]float64{}

	for _, docs := range lists {
		for rank, doc := range docs {
			byID[doc.ID] = doc
			scores[doc.ID] += 1.0 / (k + float64(rank+1))
		}
	}

	fused := make([]retrieval.Document, 0, len(scores))
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
