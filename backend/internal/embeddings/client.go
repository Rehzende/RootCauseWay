// Package embeddings provides a client for OpenAI-compatible /embeddings
// endpoints, used for semantic search over the knowledge base and incidents.
package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultModel is used when EMBEDDINGS_MODEL is not set.
	DefaultModel = "text-embedding-3-small"
	// Dimension is the expected embedding vector size (matches vector(1536) columns).
	Dimension = 1536
	// maxInputChars truncates overly long inputs before sending to the API.
	maxInputChars = 8000
)

// Embedder converts text into an embedding vector. Implementations must be
// safe for concurrent use. A nil Embedder means embeddings are disabled.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// Client calls an OpenAI-compatible POST {baseURL}/embeddings endpoint.
type Client struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

// New creates a Client. model defaults to DefaultModel when empty.
func New(baseURL, apiKey, model string) *Client {
	if model == "" {
		model = DefaultModel
	}
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NewFromConfig returns a ready Embedder, or nil when baseURL is empty
// (embeddings disabled — callers must fall back gracefully).
func NewFromConfig(baseURL, apiKey, model string) Embedder {
	if baseURL == "" {
		return nil
	}
	return New(baseURL, apiKey, model)
}

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed returns the embedding vector for the given text.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, fmt.Errorf("embeddings: empty input text")
	}
	if len(text) > maxInputChars {
		text = text[:maxInputChars]
	}

	body, err := json.Marshal(embeddingRequest{Model: c.model, Input: text})
	if err != nil {
		return nil, fmt.Errorf("embeddings: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("embeddings: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embeddings: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, fmt.Errorf("embeddings: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("embeddings: API returned status %d: %s", resp.StatusCode, truncate(string(respBody), 200))
	}

	var parsed embeddingResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("embeddings: decode response: %w", err)
	}
	if len(parsed.Data) == 0 {
		return nil, fmt.Errorf("embeddings: response contained no embeddings")
	}
	vec := parsed.Data[0].Embedding
	if len(vec) != Dimension {
		return nil, fmt.Errorf("embeddings: expected dimension %d, got %d", Dimension, len(vec))
	}
	return vec, nil
}

// VectorString serializes a vector to the pgvector text literal format
// (e.g. "[0.1,0.2,...]") suitable for passing as $1::vector.
func VectorString(v []float32) string {
	var b strings.Builder
	b.Grow(len(v) * 10)
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
