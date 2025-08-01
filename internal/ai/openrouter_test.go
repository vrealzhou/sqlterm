package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOpenRouterClient(t *testing.T) {
	client := NewOpenRouterClient("test-api-key")
	if client.apiKey != "test-api-key" {
		t.Errorf("Expected apiKey to be 'test-api-key', got '%s'", client.apiKey)
	}
	if client.baseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("Expected baseURL to be 'https://openrouter.ai/api/v1', got '%s'", client.baseURL)
	}
	if client.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}
	if client.client.Timeout != 120*time.Second {
		t.Errorf("Expected timeout to be 120s, got %v", client.client.Timeout)
	}
}

func TestOpenRouterClient_Chat(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Expected path '/api/v1/chat/completions', got '%s'", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header 'Bearer test-api-key', got '%s'", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}

		// Return a mock response
		response := `{
			"id": "chatcmpl-123",
			"object": "chat.completion",
			"created": 1234567890,
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello, world!"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 20,
				"total_tokens": 30
			}
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewOpenRouterClient("test-api-key")
	client.baseURL = server.URL

	// Create a chat request
	request := ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	// Make the request
	ctx := context.Background()
	response, err := client.Chat(ctx, request)
	if err != nil {
		t.Fatalf("Chat request failed: %v", err)
	}

	// Verify response
	if response == nil {
		t.Fatal("Expected response, got nil")
	}
	if response.ID != "chatcmpl-123" {
		t.Errorf("Expected ID 'chatcmpl-123', got '%s'", response.ID)
	}
	if response.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", response.Model)
	}
	if len(response.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(response.Choices))
	}
	if response.Choices[0].Message.Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got '%s'", response.Choices[0].Message.Content)
	}
	if response.Usage.PromptTokens != 10 {
		t.Errorf("Expected prompt_tokens 10, got %d", response.Usage.PromptTokens)
	}
	if response.Usage.CompletionTokens != 20 {
		t.Errorf("Expected completion_tokens 20, got %d", response.Usage.CompletionTokens)
	}
	if response.Usage.TotalTokens != 30 {
		t.Errorf("Expected total_tokens 30, got %d", response.Usage.TotalTokens)
	}
}

func TestOpenRouterClient_Chat_Error(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "test error"}}`))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewOpenRouterClient("test-api-key")
	client.baseURL = server.URL

	// Create a chat request
	request := ChatRequest{
		Model: "test-model",
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}

	// Make the request
	ctx := context.Background()
	response, err := client.Chat(ctx, request)
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if response != nil {
		t.Error("Expected nil response on error")
	}
	if !strings.Contains(err.Error(), "API request failed") {
		t.Errorf("Expected error to contain 'API request failed', got '%s'", err.Error())
	}
}

func TestOpenRouterClient_ListModels(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/models" {
			t.Errorf("Expected path '/models', got '%s'", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header 'Bearer test-api-key', got '%s'", r.Header.Get("Authorization"))
		}

		// Return a mock response
		response := `{
			"data": [
				{
					"id": "model1",
					"name": "Model 1",
					"created": 1234567890,
					"description": "Test model 1",
					"pricing": {
						"prompt": "0.001",
						"completion": "0.002"
					}
				},
				{
					"id": "model2",
					"name": "Model 2",
					"created": 1234567891,
					"description": "Test model 2"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewOpenRouterClient("test-api-key")
	client.baseURL = server.URL

	// Make the request
	ctx := context.Background()
	models, err := client.ListModels(ctx)
	if err != nil {
		t.Fatalf("ListModels request failed: %v", err)
	}

	// Verify response
	if len(models) != 2 {
		t.Errorf("Expected 2 models, got %d", len(models))
	}
	if models[0].ID != "model1" {
		t.Errorf("Expected first model ID 'model1', got '%s'", models[0].ID)
	}
	if models[0].Name != "Model 1" {
		t.Errorf("Expected first model name 'Model 1', got '%s'", models[0].Name)
	}
	if models[0].Description != "Test model 1" {
		t.Errorf("Expected first model description 'Test model 1', got '%s'", models[0].Description)
	}
	if models[0].Provider != "openrouter" {
		t.Errorf("Expected first model provider 'openrouter', got '%s'", models[0].Provider)
	}
	if models[0].Pricing == nil {
		t.Error("Expected pricing information for first model")
	} else {
		if models[0].Pricing.InputCostPerToken != 0.001 {
			t.Errorf("Expected input cost 0.001, got %f", models[0].Pricing.InputCostPerToken)
		}
		if models[0].Pricing.OutputCostPerToken != 0.002 {
			t.Errorf("Expected output cost 0.002, got %f", models[0].Pricing.OutputCostPerToken)
		}
	}
	if models[1].ID != "model2" {
		t.Errorf("Expected second model ID 'model2', got '%s'", models[1].ID)
	}
}

func TestOpenRouterClient_GetModelInfo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"data": [
				{
					"id": "model1",
					"name": "Model 1",
					"created": 1234567890,
					"description": "Test model 1"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewOpenRouterClient("test-api-key")
	client.baseURL = server.URL

	// Get model info
	ctx := context.Background()
	modelInfo, err := client.GetModelInfo(ctx, "model1")
	if err != nil {
		t.Fatalf("GetModelInfo request failed: %v", err)
	}

	// Verify response
	if modelInfo == nil {
		t.Fatal("Expected model info, got nil")
	}
	if modelInfo.ID != "model1" {
		t.Errorf("Expected model ID 'model1', got '%s'", modelInfo.ID)
	}
	if modelInfo.Name != "Model 1" {
		t.Errorf("Expected model name 'Model 1', got '%s'", modelInfo.Name)
	}

	// Test with non-existent model
	modelInfo, err = client.GetModelInfo(ctx, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for non-existent model, got nil")
	}
	if modelInfo != nil {
		t.Error("Expected nil model info for non-existent model")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected error to contain 'not found', got '%s'", err.Error())
	}
}

func TestOpenRouterClient_Close(t *testing.T) {
	client := NewOpenRouterClient("test-api-key")
	err := client.Close()
	if err != nil {
		t.Errorf("Expected Close to return nil, got %v", err)
	}
}