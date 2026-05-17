package quality

import (
	"regexp"
	"sort"
	"strings"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

var tokenPattern = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9_-]*`)

func Evaluate(answer string, docs []retrieval.Document) retrieval.GroundingReport {
	contextTerms := map[string]bool{}
	contextTokenCount := 0
	for _, doc := range docs {
		for _, token := range tokenize(doc.Title + " " + doc.Content + " " + doc.Source) {
			contextTerms[token] = true
			contextTokenCount++
		}
	}

	answerTokens := tokenize(answer)
	if len(answerTokens) == 0 {
		return retrieval.GroundingReport{
			ContextDocumentCount: len(docs),
			ContextTokenCount:    contextTokenCount,
			HallucinationRisk:    "unknown",
		}
	}

	supported := 0
	unsupported := map[string]bool{}
	for _, token := range answerTokens {
		if isStopword(token) {
			continue
		}
		if contextTerms[token] {
			supported++
			continue
		}
		unsupported[token] = true
	}

	meaningfulTokens := 0
	for _, token := range answerTokens {
		if !isStopword(token) {
			meaningfulTokens++
		}
	}
	ratio := 1.0
	if meaningfulTokens > 0 {
		ratio = float64(supported) / float64(meaningfulTokens)
	}

	return retrieval.GroundingReport{
		ContextDocumentCount: len(docs),
		ContextTokenCount:    contextTokenCount,
		AnswerTokenCount:     len(answerTokens),
		GroundedTokenRatio:   ratio,
		UnsupportedTerms:     topTerms(unsupported, 12),
		HallucinationRisk:    risk(ratio),
	}
}

func tokenize(text string) []string {
	raw := tokenPattern.FindAllString(strings.ToLower(text), -1)
	tokens := make([]string, 0, len(raw))
	for _, token := range raw {
		if len(token) > 2 {
			tokens = append(tokens, token)
		}
	}
	return tokens
}

func topTerms(terms map[string]bool, limit int) []string {
	out := make([]string, 0, len(terms))
	for term := range terms {
		out = append(out, term)
	}
	sort.Strings(out)
	if len(out) > limit {
		return out[:limit]
	}
	return out
}

func risk(ratio float64) string {
	switch {
	case ratio >= 0.7:
		return "low"
	case ratio >= 0.45:
		return "medium"
	default:
		return "high"
	}
}

func isStopword(token string) bool {
	_, ok := stopwords[token]
	return ok
}

var stopwords = map[string]struct{}{
	"and": {}, "are": {}, "based": {}, "but": {}, "for": {}, "from": {}, "has": {}, "have": {},
	"into": {}, "not": {}, "the": {}, "this": {}, "that": {}, "then": {}, "there": {}, "to": {},
	"using": {}, "when": {}, "with": {}, "you": {}, "your": {}, "can": {}, "could": {}, "would": {},
	"should": {}, "about": {}, "answer": {}, "context": {}, "retrieved": {}, "query": {}, "question": {},
}
