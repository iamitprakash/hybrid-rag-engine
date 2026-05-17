package llm

import (
	"fmt"
	"strings"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

type Synthesizer struct{}

func NewSynthesizer() *Synthesizer {
	return &Synthesizer{}
}

func (s *Synthesizer) Synthesize(query string, docs []retrieval.Document) string {
	if len(docs) == 0 {
		return "I could not find enough relevant context to answer the query."
	}

	var sources []string
	for i, doc := range docs {
		if i == 3 {
			break
		}
		sources = append(sources, fmt.Sprintf("%s: %s", doc.Title, doc.Content))
	}

	return fmt.Sprintf(
		"Based on the retrieved context, the answer to %q is grounded in: %s",
		query,
		strings.Join(sources, " "),
	)
}
