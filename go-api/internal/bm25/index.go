package bm25

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

type Index struct {
	mu        sync.RWMutex
	docs      []retrieval.Document
	docTokens []map[string]int
	docFreq   map[string]int
	avgLen    float64
}

var tokenPattern = regexp.MustCompile(`[a-zA-Z0-9]+`)

func NewIndex(docs []retrieval.Document) *Index {
	idx := &Index{docFreq: map[string]int{}}
	idx.AddDocuments(docs)
	return idx
}

func (idx *Index) AddDocuments(docs []retrieval.Document) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.docs = append(idx.docs, docs...)
	idx.rebuildLocked()
}

func (idx *Index) Documents() []retrieval.Document {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	docs := make([]retrieval.Document, len(idx.docs))
	copy(docs, idx.docs)
	return docs
}

func (idx *Index) rebuildLocked() {
	idx.docTokens = nil
	idx.docFreq = map[string]int{}
	totalLen := 0
	for _, doc := range idx.docs {
		tokens := tokenize(doc.Title + " " + doc.Content)
		counts := map[string]int{}
		for _, token := range tokens {
			counts[token]++
		}
		for token := range counts {
			idx.docFreq[token]++
		}
		totalLen += len(tokens)
		idx.docTokens = append(idx.docTokens, counts)
	}
	if len(idx.docs) > 0 {
		idx.avgLen = float64(totalLen) / float64(len(idx.docs))
	}
}

func (idx *Index) Search(query string, limit int, filters map[string]string) []retrieval.Document {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}
	queryTokens := tokenize(query)
	results := make([]retrieval.Document, 0, len(idx.docs))
	for i, doc := range idx.docs {
		if !matchesFilters(doc, filters) {
			continue
		}
		score := idx.score(queryTokens, idx.docTokens[i])
		if score > 0 {
			candidate := doc
			candidate.Score = score
			results = append(results, candidate)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > limit {
		return results[:limit]
	}
	return results
}

func matchesFilters(doc retrieval.Document, filters map[string]string) bool {
	if len(filters) == 0 {
		return true
	}
	for key, value := range filters {
		switch key {
		case "source":
			if doc.Source != value {
				return false
			}
		case "id":
			if doc.ID != value {
				return false
			}
		default:
			if doc.Metadata == nil || doc.Metadata[key] != value {
				return false
			}
		}
	}
	return true
}

func (idx *Index) score(queryTokens []string, doc map[string]int) float64 {
	const k1 = 1.5
	const b = 0.75
	docLen := 0
	for _, count := range doc {
		docLen += count
	}
	if docLen == 0 || idx.avgLen == 0 {
		return 0
	}
	var score float64
	for _, token := range queryTokens {
		tf := float64(doc[token])
		if tf == 0 {
			continue
		}
		df := float64(idx.docFreq[token])
		idf := math.Log(1 + (float64(len(idx.docs))-df+0.5)/(df+0.5))
		denominator := tf + k1*(1-b+b*(float64(docLen)/idx.avgLen))
		score += idf * ((tf * (k1 + 1)) / denominator)
	}
	return score
}

func tokenize(text string) []string {
	raw := tokenPattern.FindAllString(strings.ToLower(text), -1)
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		if len(token) > 1 {
			tokens = append(tokens, token)
		}
	}
	return tokens
}
