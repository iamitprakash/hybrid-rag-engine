package retrieval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type AIClient struct {
	baseURL string
	client  *http.Client
}

type embedRequest struct {
	Text string `json:"text"`
}

type embedResponse struct {
	Embedding []float32 `json:"embedding"`
}

type rerankRequest struct {
	Query     string     `json:"query"`
	Documents []Document `json:"documents"`
	TopK      int        `json:"top_k"`
}

type rerankResponse struct {
	Documents []Document `json:"documents"`
}

func NewAIClient(baseURL string, timeout time.Duration) *AIClient {
	return &AIClient{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
	}
}

func (c *AIClient) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.client = client
	}
}

func (c *AIClient) Embed(text string) ([]float32, error) {
	var out embedResponse
	if err := c.post("/embed", embedRequest{Text: text}, &out); err != nil {
		return nil, err
	}
	return out.Embedding, nil
}

func (c *AIClient) Rerank(query string, docs []Document, topK int) ([]Document, error) {
	var out rerankResponse
	if err := c.post("/rerank", rerankRequest{Query: query, Documents: docs, TopK: topK}, &out); err != nil {
		return nil, err
	}
	return out.Documents, nil
}

func (c *AIClient) post(path string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := c.client.Post(c.baseURL+path, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("ai service returned status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
