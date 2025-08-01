package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewOllamaClient(t *testing.T) {
	// Test with custom baseURL
	client := NewOllamaClient("http://localhost:5678")
	if client.baseURL != "http://localhost:5678" {
		t.Errorf("Expected baseURL to be 'http://localhost:5678', got '%s'", client.baseURL)
	}
	if client.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}
	if client.client.Timeout != 120*time.Second {
		t.Errorf("Expected timeout to be 120s, got %v", client.client.Timeout)
	}

	// Test with empty baseURL (should use default)
	client = NewOllamaClient("")
	if client.baseURL != "http://localhost:11434" {
		t.Errorf("Expected default baseURL to be 'http://localhost:11434', got '%s'", client.baseURL)
	}
}

func TestOllamaClient_Chat(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/api/chat" {
			t.Errorf("Expected path '/api/chat', got '%s'", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type 'application/json', got '%s'", r.Header.Get("Content-Type"))
		}

		// Return a mock response
		response := `{
			"model": "test-model",
			"created_at": "2023-08-04T19:22:45.499127Z",
			"message": {
				"role": "assistant",
				"content": "Hello, world!"
			},
			"done": true,
			"total_duration": 1234567890,
			"load_duration": 123456789,
			"prompt_eval_count": 10,
			"prompt_eval_duration": 123456789,
			"eval_count": 20,
			"eval_duration": 123456789
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewOllamaClient(server.URL)

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
	if response.Model != "test-model" {
		t.Errorf("Expected model 'test-model', got '%s'", response.Model)
	}
	if len(response.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(response.Choices))
	}
	if response.Choices[0].Message.Content != "Hello, world!" {
		t.Errorf("Expected content 'Hello, world!', got '%s'", response.Choices[0].Message.Content)
	}
	if response.Choices[0].Message.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", response.Choices[0].Message.Role)
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

func TestOllamaClient_Chat_Error(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "test error"}`))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewOllamaClient(server.URL)

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

func TestOllamaClient_ListModels(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/api/tags" {
			t.Errorf("Expected path '/api/tags', got '%s'", r.URL.Path)
		}

		// Return a mock response
		response := `{
			"models": [
				{
					"name": "model1:latest",
					"modified_at": "2023-08-04T19:22:45.499127Z",
					"size": 1234567890,
					"digest": "sha256:test1"
				},
				{
					"name": "model2:v1.0",
					"modified_at": "2023-08-04T19:22:45.499127Z",
					"size": 9876543210,
					"digest": "sha256:test2"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewOllamaClient(server.URL)

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
	if models[0].ID != "model1:latest" {
		t.Errorf("Expected first model ID 'model1:latest', got '%s'", models[0].ID)
	}
	if models[0].Name != "model1:latest" {
		t.Errorf("Expected first model name 'model1:latest', got '%s'", models[0].Name)
	}
	if !strings.Contains(models[0].Description, "Local Ollama model") {
		t.Errorf("Expected first model description to contain 'Local Ollama model', got '%s'", models[0].Description)
	}
	if models[0].Provider != "ollama" {
		t.Errorf("Expected first model provider 'ollama', got '%s'", models[0].Provider)
	}
	if models[1].ID != "model2:v1.0" {
		t.Errorf("Expected second model ID 'model2:v1.0', got '%s'", models[1].ID)
	}
}

func TestOllamaClient_GetModelInfo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"models": [
				{
					"name": "model1:latest",
					"modified_at": "2023-08-04T19:22:45.499127Z",
					"size": 1234567890,
					"digest": "sha256:test1"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewOllamaClient(server.URL)

	// Get model info
	ctx := context.Background()
	modelInfo, err := client.GetModelInfo(ctx, "model1:latest")
	if err != nil {
		t.Fatalf("GetModelInfo request failed: %v", err)
	}

	// Verify response
	if modelInfo == nil {
		t.Fatal("Expected model info, got nil")
	}
	if modelInfo.ID != "model1:latest" {
		t.Errorf("Expected model ID 'model1:latest', got '%s'", modelInfo.ID)
	}
	if modelInfo.Name != "model1:latest" {
		t.Errorf("Expected model name 'model1:latest', got '%s'", modelInfo.Name)
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

func TestOllamaClient_Close(t *testing.T) {
	client := NewOllamaClient("http://localhost:11434")
	err := client.Close()
	if err != nil {
		t.Errorf("Expected Close to return nil, got %v", err)
	}
}