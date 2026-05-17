package vector

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

type Client struct {
	baseURL    string
	collection string
	dimension  int
	httpClient *http.Client
}

func NewClient(baseURL string, collection string, dimension int) *Client {
	return &Client{
		baseURL:    baseURL,
		collection: collection,
		dimension:  dimension,
		httpClient: &http.Client{Timeout: 20 * time.Second},
	}
}

func (c *Client) EnsureCollection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/collections/"+c.collection, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	payload := map[string]any{
		"vectors": map[string]any{
			"size":     c.dimension,
			"distance": "Cosine",
		},
	}
	return c.request(ctx, http.MethodPut, "/collections/"+c.collection, payload, nil)
}

func (c *Client) Upsert(ctx context.Context, docs []retrieval.Document, vectors [][]float32) error {
	if len(docs) != len(vectors) {
		return fmt.Errorf("documents and vectors length mismatch: %d != %d", len(docs), len(vectors))
	}
	points := make([]map[string]any, 0, len(docs))
	for i, doc := range docs {
		points = append(points, map[string]any{
			"id":     pointID(doc.ID),
			"vector": vectors[i],
			"payload": map[string]any{
				"id":       doc.ID,
				"title":    doc.Title,
				"content":  doc.Content,
				"source":   doc.Source,
				"metadata": doc.Metadata,
			},
		})
	}
	return c.request(ctx, http.MethodPut, "/collections/"+c.collection+"/points?wait=true", map[string]any{"points": points}, nil)
}

func (c *Client) Search(ctx context.Context, embedding []float32, limit int) ([]retrieval.Document, error) {
	var out struct {
		Result []struct {
			Score   float64 `json:"score"`
			Payload struct {
				ID       string            `json:"id"`
				Title    string            `json:"title"`
				Content  string            `json:"content"`
				Source   string            `json:"source"`
				Metadata map[string]string `json:"metadata"`
			} `json:"payload"`
		} `json:"result"`
	}
	payload := map[string]any{"vector": embedding, "limit": limit, "with_payload": true}
	if err := c.request(ctx, http.MethodPost, "/collections/"+c.collection+"/points/search", payload, &out); err != nil {
		return nil, err
	}
	docs := make([]retrieval.Document, 0, len(out.Result))
	for _, point := range out.Result {
		docs = append(docs, retrieval.Document{
			ID:       point.Payload.ID,
			Title:    point.Payload.Title,
			Content:  point.Payload.Content,
			Source:   point.Payload.Source,
			Metadata: point.Payload.Metadata,
			Score:    point.Score,
		})
	}
	return docs, nil
}

func (c *Client) request(ctx context.Context, method string, path string, payload any, out any) error {
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, &body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("qdrant returned status %d for %s %s", resp.StatusCode, method, path)
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func pointID(id string) string {
	sum := sha1.Sum([]byte(id))
	bytes := sum[:16]
	bytes[6] = (bytes[6] & 0x0f) | 0x50
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}
