package chroma

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	config "training_with_ai/configs"
)

type Client struct {
	baseURL    string
	collection string
	topK       int
	httpClient *http.Client
	apiKey     string
}

type queryRequest struct {
	QueryEmbeddings [][]float32 `json:"query_embeddings"`
	NResults        int         `json:"n_results"`
	Include         []string    `json:"include"`
}

type queryResponse struct {
	Documents [][]string `json:"documents"`
}

func NewClient(cfg config.ChromaConfig) (*Client, error) {
	if cfg.BaseURL == "" || cfg.Collection == "" {
		return nil, errors.New("chroma config is incomplete")
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	topK := cfg.TopK
	if topK <= 0 {
		topK = 3
	}

	return &Client{
		baseURL:    cfg.BaseURL,
		collection: cfg.Collection,
		topK:       topK,
		httpClient: &http.Client{Timeout: timeout},
		apiKey:     cfg.APIKey,
	}, nil
}

func (c *Client) Query(ctx context.Context, embedding []float32) ([]string, error) {
	payload := queryRequest{
		QueryEmbeddings: [][]float32{embedding},
		NResults:        c.topK,
		Include:         []string{"documents"},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	url := c.baseURL + "/api/v1/collections/" + c.collection + "/query"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusBadRequest {
		return nil, errors.New("chroma response status is not ok")
	}

	var parsed queryResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, err
	}
	if len(parsed.Documents) == 0 {
		return []string{}, nil
	}

	return parsed.Documents[0], nil
}
