package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sqlterm/internal/i18n"
	"time"
)

type LMStudioClient struct {
	baseURL string
	client  *http.Client
	i18nMgr *i18n.Manager
}

func NewLMStudioClient(baseURL string, i18nMgr *i18n.Manager) *LMStudioClient {
	if baseURL == "" {
		baseURL = "http://localhost:1234"
	}

	return &LMStudioClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second, // Increased for complex queries
		},
		i18nMgr: i18nMgr,
	}
}

func (c *LMStudioClient) Chat(ctx context.Context, request ChatRequest) (*ChatResponse, error) {
	url := fmt.Sprintf("%s/v1/chat/completions", c.baseURL)

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf(c.i18nMgr.Get("failed_to_marshal_request"), err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf(c.i18nMgr.Get("failed_to_create_request"), err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(c.i18nMgr.Get("request_failed"), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(c.i18nMgr.Get("api_request_failed"), resp.StatusCode, string(body))
	}

	var response ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf(c.i18nMgr.Get("failed_to_decode_response"), err)
	}

	return &response, nil
}

func (c *LMStudioClient) ListModels(ctx context.Context) ([]ModelInfo, error) {
	url := fmt.Sprintf("%s/v1/models", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf(c.i18nMgr.Get("failed_to_create_request"), err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(c.i18nMgr.Get("request_failed"), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf(c.i18nMgr.Get("api_request_failed"), resp.StatusCode, string(body))
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
		return nil, fmt.Errorf(c.i18nMgr.Get("failed_to_decode_response"), err)
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

	return nil, fmt.Errorf(c.i18nMgr.Get("model_not_found"), modelID)
}

func (c *LMStudioClient) Close() error {
	return nil
}
