package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"hybrid-rag-engine/go-api/internal/retrieval"
)

type Config struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

type Synthesizer struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

func NewSynthesizer(config Config) *Synthesizer {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 45 * time.Second
	}
	baseURL := strings.TrimRight(config.BaseURL, "/")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	model := config.Model
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Synthesizer{
		baseURL: baseURL,
		apiKey:  config.APIKey,
		model:   model,
		client:  &http.Client{Timeout: timeout},
	}
}

func (s *Synthesizer) Synthesize(ctx context.Context, query string, docs []retrieval.Document) (string, error) {
	if len(docs) == 0 {
		return "I could not find enough relevant context to answer the query.", nil
	}
	if s.apiKey == "" {
		return localAnswer(query, docs), nil
	}

	payload := chatRequest{
		Model: s.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt()},
			{Role: "user", Content: userPrompt(query, docs)},
		},
		Temperature: 0.2,
	}
	var out chatResponse
	if err := s.post(ctx, payload, &out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("llm returned no choices")
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}

func (s *Synthesizer) Stream(ctx context.Context, query string, docs []retrieval.Document) (<-chan string, <-chan error) {
	tokens := make(chan string)
	errs := make(chan error, 1)

	go func() {
		defer close(tokens)
		defer close(errs)

		if len(docs) == 0 {
			tokens <- "I could not find enough relevant context to answer the query."
			return
		}
		if s.apiKey == "" {
			streamLocal(ctx, localAnswer(query, docs), tokens, errs)
			return
		}
		if err := s.streamRemote(ctx, query, docs, tokens); err != nil {
			errs <- err
		}
	}()

	return tokens, errs
}

func localAnswer(query string, docs []retrieval.Document) string {
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

func streamLocal(ctx context.Context, answer string, tokens chan<- string, errs chan<- error) {
	parts := strings.Split(answer, " ")
	for i, part := range parts {
		select {
		case <-ctx.Done():
			errs <- ctx.Err()
			return
		case tokens <- part + suffix(i, len(parts)):
		}
	}
}

func suffix(index int, length int) string {
	if index == length-1 {
		return ""
	}
	return " "
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

type streamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func (s *Synthesizer) post(ctx context.Context, payload chatRequest, out *chatResponse) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("llm returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (s *Synthesizer) streamRemote(ctx context.Context, query string, docs []retrieval.Document, tokens chan<- string) error {
	payload := chatRequest{
		Model: s.model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt()},
			{Role: "user", Content: userPrompt(query, docs)},
		},
		Temperature: 0.2,
		Stream:      true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+s.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("llm returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(responseBody)))
	}

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			return nil
		}
		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return err
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content == "" {
				continue
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case tokens <- choice.Delta.Content:
			}
		}
	}
	return scanner.Err()
}

func systemPrompt() string {
	return "You are a retrieval-grounded assistant. Answer using only the supplied context. If the context is insufficient, say so briefly. Cite document titles inline when useful."
}

func userPrompt(query string, docs []retrieval.Document) string {
	var builder strings.Builder
	builder.WriteString("Question:\n")
	builder.WriteString(query)
	builder.WriteString("\n\nRetrieved context:\n")
	for i, doc := range docs {
		fmt.Fprintf(&builder, "[%d] Title: %s\nSource: %s\nContent: %s\n\n", i+1, doc.Title, doc.Source, doc.Content)
	}
	return builder.String()
}
