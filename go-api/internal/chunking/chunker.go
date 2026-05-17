package chunking

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

type RawDocument struct {
	ID       string            `json:"id"`
	Title    string            `json:"title" binding:"required"`
	Content  string            `json:"content" binding:"required"`
	Source   string            `json:"source,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Options struct {
	ChunkSize int `json:"chunk_size"`
	Overlap   int `json:"overlap"`
}

func ChunkDocuments(rawDocs []RawDocument, opts Options) []retrieval.Document {
	chunkSize := opts.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 180
	}
	overlap := opts.Overlap
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}

	var chunks []retrieval.Document
	for _, raw := range rawDocs {
		words := splitWords(raw.Content)
		if len(words) == 0 {
			continue
		}
		docID := raw.ID
		if docID == "" {
			docID = stableID(raw.Title + "\n" + raw.Content)
		}

		step := chunkSize - overlap
		for start, chunkIndex := 0, 0; start < len(words); start, chunkIndex = start+step, chunkIndex+1 {
			end := start + chunkSize
			if end > len(words) {
				end = len(words)
			}
			content := strings.Join(words[start:end], " ")
			metadata := copyMetadata(raw.Metadata)
			metadata["document_id"] = docID
			metadata["chunk_index"] = fmt.Sprintf("%d", chunkIndex)
			metadata["chunk_start_word"] = fmt.Sprintf("%d", start)
			metadata["chunk_end_word"] = fmt.Sprintf("%d", end)

			chunks = append(chunks, retrieval.Document{
				ID:       fmt.Sprintf("%s-chunk-%04d", docID, chunkIndex),
				Title:    raw.Title,
				Content:  content,
				Source:   raw.Source,
				Metadata: metadata,
			})
			if end == len(words) {
				break
			}
		}
	}
	return chunks
}

func splitWords(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r)
	})
}

func stableID(text string) string {
	sum := sha1.Sum([]byte(text))
	return hex.EncodeToString(sum[:])[:16]
}

func copyMetadata(metadata map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range metadata {
		out[key] = value
	}
	return out
}
