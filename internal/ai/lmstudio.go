package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type LMStudioClient struct {
	baseURL string
	client  *http.Client
}

func NewLMStudioClient(baseURL string) *LMStudioClient {
	if baseURL == "" {
		baseURL = "http://localhost:1234"
	}

	return &LMStudioClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second, // Increased for complex queries
		},
	}
}

func (c *LMStudioClient) Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	url := fmt.Sprintf("%s/v1/chat/completions", c.baseURL)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &response, nil
}

func (c *LMStudioClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := fmt.Sprintf("%s/v1/models", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Object string `json:"object"`
		Data   []struct {
			ID      string `json:"id"`
			Object  string `json:"object"`
			Created int64  `json:"created"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]ModelInfo, len(response.Data))
	for i, model := range response.Data {
		models[i] = ModelInfo{
			ID:          model.ID,
			Name:        model.ID,
			Description: "LM Studio local model",
			Provider:    "lmstudio",
		}
	}

	return models, nil
}

func (c *LMStudioClient) GetModelInfo(ctx context.Context, modelID string) (*ModelInfo, error) {
	models, err := c.ListModels(ctx)
	if err != nil {
		return nil, err
	}

	for _, model := range models {
		if model.ID == modelID {
			return &model, nil
		}
	}

	return nil, fmt.Errorf("model %s not found", modelID)
}

func (c *LMStudioClient) Close() error {
	return nil
}
