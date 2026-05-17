package vector

import (
	"bytes"
	"context"
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
	payload := map[string]any{
		"vectors": map[string]any{
			"size":     c.dimension,
			"distance": "Cosine",
		},
	}
	return c.request(ctx, http.MethodPut, "/collections/"+c.collection, payload, nil)
}

func (c *Client) Upsert(ctx context.Context, docs []retrieval.Document, vectors [][]float32) error {
	points := make([]map[string]any, 0, len(docs))
	for i, doc := range docs {
		points = append(points, map[string]any{
			"id":     doc.ID,
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
