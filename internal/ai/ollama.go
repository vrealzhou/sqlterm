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

type OllamaClient struct {
	baseURL string
	client  *http.Client
}

func NewOllamaClient(baseURL string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	
	return &OllamaClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second, // Increased for complex queries
		},
	}
}

func (c *OllamaClient) Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	url := fmt.Sprintf("%s/api/chat", c.baseURL)
	
	// Convert to Ollama format
	ollamaRequest := struct {
		Model    string        `json:"model"`
		Messages []ChatMessage `json:"messages"`
		Stream   bool          `json:"stream"`
		Options  map[string]interface{} `json:"options,omitempty"`
	}{
		Model:    request.Model,
		Messages: request.Messages,
		Stream:   false,
		Options:  make(map[string]interface{}),
	}

	if request.Temperature > 0 {
		ollamaRequest.Options["temperature"] = request.Temperature
	}
	if request.MaxTokens > 0 {
		ollamaRequest.Options["num_predict"] = request.MaxTokens
	}

	jsonData, err := json.Marshal(ollamaRequest)
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

	var ollamaResponse struct {
		Model     string `json:"model"`
		CreatedAt string `json:"created_at"`
		Message   struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		Done               bool `json:"done"`
		TotalDuration      int64 `json:"total_duration"`
		LoadDuration       int64 `json:"load_duration"`
		PromptEvalCount    int `json:"prompt_eval_count"`
		PromptEvalDuration int64 `json:"prompt_eval_duration"`
		EvalCount          int `json:"eval_count"`
		EvalDuration       int64 `json:"eval_duration"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&ollamaResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Convert to standard format
	response := &ChatResponse{
		ID:      fmt.Sprintf("ollama-%d", time.Now().Unix()),
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   ollamaResponse.Model,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		}{{
			Index: 0,
			Message: struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			}{
				Role:    ollamaResponse.Message.Role,
				Content: ollamaResponse.Message.Content,
			},
			FinishReason: "stop",
		}},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     ollamaResponse.PromptEvalCount,
			CompletionTokens: ollamaResponse.EvalCount,
			TotalTokens:      ollamaResponse.PromptEvalCount + ollamaResponse.EvalCount,
		},
	}

	return response, nil
}

func (c *OllamaClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := fmt.Sprintf("%s/api/tags", c.baseURL)
	
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
		Models []struct {
			Name       string `json:"name"`
			ModifiedAt string `json:"modified_at"`
			Size       int64  `json:"size"`
			Digest     string `json:"digest"`
		} `json:"models"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	models := make([]ModelInfo, len(response.Models))
	for i, model := range response.Models {
		models[i] = ModelInfo{
			ID:          model.Name,
			Name:        model.Name,
			Description: fmt.Sprintf("Local Ollama model (Size: %.1fGB)", float64(model.Size)/(1024*1024*1024)),
			Provider:    "ollama",
		}
	}

	return models, nil
}

func (c *OllamaClient) GetModelInfo(ctx context.Context, modelID string) (*ModelInfo, error) {
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

func (c *OllamaClient) Close() error {
	return nil
}