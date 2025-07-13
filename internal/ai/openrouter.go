package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type OpenRouterClient struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewOpenRouterClient(apiKey string) *OpenRouterClient {
	return &OpenRouterClient{
		apiKey:  apiKey,
		baseURL: "https://openrouter.ai/api/v1",
		client: &http.Client{
			Timeout: 120 * time.Second, // Increased for complex queries
		},
	}
}

func (c *OpenRouterClient) Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://sqlterm.ai")
	req.Header.Set("X-Title", "SQLTerm")

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

func (c *OpenRouterClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := fmt.Sprintf("%s/models", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)

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
		Data []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Created int64  `json:"created"`
			Description string `json:"description"`
			Pricing *struct {
				Prompt     string `json:"prompt"`
				Completion string `json:"completion"`
			} `json:"pricing"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]ModelInfo, len(response.Data))
	for i, model := range response.Data {
		models[i] = ModelInfo{
			ID:          model.ID,
			Name:        model.Name,
			Description: model.Description,
			Provider:    "openrouter",
		}
		
		// Parse pricing if available
		if model.Pricing != nil {
			pricing := &Pricing{}
			
			// Parse prompt pricing (input tokens)
			if promptPrice, err := strconv.ParseFloat(model.Pricing.Prompt, 64); err == nil {
				pricing.InputCostPerToken = promptPrice
			}
			
			// Parse completion pricing (output tokens)
			if completionPrice, err := strconv.ParseFloat(model.Pricing.Completion, 64); err == nil {
				pricing.OutputCostPerToken = completionPrice
			}
			
			models[i].Pricing = pricing
		}
	}

	return models, nil
}

func (c *OpenRouterClient) GetModelInfo(ctx context.Context, modelID string) (*ModelInfo, error) {
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

func (c *OpenRouterClient) Close() error {
	return nil
}