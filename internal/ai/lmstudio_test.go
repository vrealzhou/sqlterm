package ai

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/vrealzhou/sqlterm/internal/i18n"
)

func TestNewLMStudioClient(t *testing.T) {
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}
	
	// Test with custom baseURL
	client := NewLMStudioClient("http://localhost:5678", i18nMgr)
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
	client = NewLMStudioClient("", i18nMgr)
	if client.baseURL != "http://localhost:1234" {
		t.Errorf("Expected default baseURL to be 'http://localhost:1234', got '%s'", client.baseURL)
	}
}

func TestLMStudioClient_Chat(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("Expected path '/v1/chat/completions', got '%s'", r.URL.Path)
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
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}
	client := NewLMStudioClient(server.URL, i18nMgr)

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

func TestLMStudioClient_Chat_Error(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "test error"}`))
	}))
	defer server.Close()

	// Create client with test server URL
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}
	client := NewLMStudioClient(server.URL, i18nMgr)

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

func TestLMStudioClient_ListModels(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/v1/models" {
			t.Errorf("Expected path '/v1/models', got '%s'", r.URL.Path)
		}

		// Return a mock response
		response := `{
			"object": "list",
			"data": [
				{
					"id": "model1",
					"object": "model",
					"created": 1234567890,
					"owned_by": "test"
				},
				{
					"id": "model2",
					"object": "model",
					"created": 1234567891,
					"owned_by": "test"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}
	client := NewLMStudioClient(server.URL, i18nMgr)

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
	if models[0].Name != "model1" {
		t.Errorf("Expected first model name 'model1', got '%s'", models[0].Name)
	}
	if models[0].Description != "LM Studio local model" {
		t.Errorf("Expected first model description 'LM Studio local model', got '%s'", models[0].Description)
	}
	if models[0].Provider != "lmstudio" {
		t.Errorf("Expected first model provider 'lmstudio', got '%s'", models[0].Provider)
	}
	if models[1].ID != "model2" {
		t.Errorf("Expected second model ID 'model2', got '%s'", models[1].ID)
	}
}

func TestLMStudioClient_GetModelInfo(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := `{
			"object": "list",
			"data": [
				{
					"id": "model1",
					"object": "model",
					"created": 1234567890,
					"owned_by": "test"
				}
			]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}
	client := NewLMStudioClient(server.URL, i18nMgr)

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
	if modelInfo.Name != "model1" {
		t.Errorf("Expected model name 'model1', got '%s'", modelInfo.Name)
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

func TestLMStudioClient_Close(t *testing.T) {
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}
	client := NewLMStudioClient("http://localhost:1234", i18nMgr)
	err = client.Close()
	if err != nil {
		t.Errorf("Expected Close to return nil, got %v", err)
	}
}