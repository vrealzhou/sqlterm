package i18n

import (
	"fmt"
	"strings"
	"testing"
)

func TestNewManager(t *testing.T) {
	testCases := []struct {
		name     string
		language string
		hasError bool
	}{
		{
			name:     "English language",
			language: "en_au",
			hasError: false,
		},
		{
			name:     "Chinese language",
			language: "zh_cn",
			hasError: false,
		},
		{
			name:     "Invalid language",
			language: "invalid_lang",
			hasError: true,
		},
		{
			name:     "Empty language",
			language: "",
			hasError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := NewManager(tc.language)

			if tc.hasError {
				if err == nil {
					t.Errorf("Expected error for language '%s', but got none", tc.language)
				}
				if manager != nil {
					t.Error("Expected manager to be nil when error occurs")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for language '%s': %v", tc.language, err)
				}
				if manager == nil {
					t.Error("Expected manager to be non-nil when no error occurs")
				}
				if manager.currentLanguage != tc.language {
					t.Errorf("Expected current language to be '%s', got '%s'", tc.language, manager.currentLanguage)
				}
			}
		})
	}
}

func TestManager_Get(t *testing.T) {
	manager, err := NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	testCases := []struct {
		name      string
		messageID string
		expectKey bool // true if we expect the key to exist
	}{
		{
			name:      "Existing message",
			messageID: "ai_not_configured",
			expectKey: true,
		},
		{
			name:      "Another existing message",
			messageID: "help_connect",
			expectKey: true,
		},
		{
			name:      "Non-existing message",
			messageID: "non_existent_key",
			expectKey: false,
		},
		{
			name:      "Empty message ID",
			messageID: "",
			expectKey: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := manager.Get(tc.messageID)

			if tc.expectKey {
				// Should return the actual message, not the key
				if result == tc.messageID {
					t.Errorf("Expected to get message for key '%s', but got the key itself", tc.messageID)
				}
				if result == "" {
					t.Errorf("Expected non-empty message for key '%s'", tc.messageID)
				}
			} else {
				// Should return the key in [key] format when not found
				expected := fmt.Sprintf("[%s]", tc.messageID)
				if result != expected {
					t.Errorf("Expected to get key '%s' back for non-existent message, got '%s'", expected, result)
				}
			}
		})
	}
}

func TestManager_GetWithArgs(t *testing.T) {
	manager, err := NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	testCases := []struct {
		name      string
		messageID string
		args      []interface{}
		contains  []string // Check that result contains these strings
	}{
		{
			name:      "Message with single argument",
			messageID: "ai_conversation_history",
			args:      []interface{}{5},
			contains:  []string{"5"},
		},
		{
			name:      "Message with multiple arguments",
			messageID: "provider_info",
			args:      []interface{}{"openrouter", "claude-3.5-sonnet", 100, 50},
			contains:  []string{"openrouter", "claude-3.5-sonnet", "100", "50"},
		},
		{
			name:      "Message with string argument",
			messageID: "connecting_to",
			args:      []interface{}{"test-db"},
			contains:  []string{"test-db"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := manager.GetWithArgs(tc.messageID, tc.args...)

			for _, expected := range tc.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("Expected result to contain '%s', but it didn't. Result: %s", expected, result)
				}
			}

			// Should not be empty
			if result == "" {
				t.Error("GetWithArgs should not return empty string for valid message")
			}
		})
	}
}

func TestManager_SetLanguage(t *testing.T) {
	manager, err := NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test changing to Chinese
	err = manager.SetLanguage("zh_cn")
	if err != nil {
		t.Fatalf("Failed to set language to zh_cn: %v", err)
	}
	if manager.currentLanguage != "zh_cn" {
		t.Errorf("Expected current language to be 'zh_cn', got '%s'", manager.currentLanguage)
	}

	// Test that messages are actually in Chinese now
	message := manager.Get("ai_not_configured")
	if !strings.Contains(message, "AI 未配置") {
		t.Error("Expected Chinese message after setting language to zh_cn")
	}

	// Test changing back to English
	err = manager.SetLanguage("en_au")
	if err != nil {
		t.Fatalf("Failed to set language to en_au: %v", err)
	}
	if manager.currentLanguage != "en_au" {
		t.Errorf("Expected current language to be 'en_au', got '%s'", manager.currentLanguage)
	}

	// Test that messages are back to English
	message = manager.Get("ai_not_configured")
	if !strings.Contains(message, "AI is not configured") {
		t.Error("Expected English message after setting language to en_au")
	}
}

func TestManager_GetAvailableLanguages(t *testing.T) {
	manager, err := NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	languages := manager.GetAvailableLanguages()

	if len(languages) == 0 {
		t.Error("Expected at least one available language")
	}

	// Should contain both English and Chinese
	expectedLanguages := []string{"en_au", "zh_cn"}
	for _, expected := range expectedLanguages {
		found := false
		for _, lang := range languages {
			if lang == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language '%s' to be in available languages %v", expected, languages)
		}
	}
}

func TestManager_MessageConsistency(t *testing.T) {
	// Test that both languages have the same message keys
	enManager, err := NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create English manager: %v", err)
	}

	zhManager, err := NewManager("zh_cn")
	if err != nil {
		t.Fatalf("Failed to create Chinese manager: %v", err)
	}

	// Test some key messages that should exist in both languages
	testKeys := []string{
		"ai_not_configured",
		"help_connect",
		"saved_connections",
		"connecting_to",
		"connected_to",
		"status_connected",
		"help_config_title",
		"help_exec_title",
	}

	for _, key := range testKeys {
		t.Run("Key_"+key, func(t *testing.T) {
			enMessage := enManager.Get(key)
			zhMessage := zhManager.Get(key)

			// Both should return actual messages, not the key
			if enMessage == key {
				t.Errorf("English manager returned key instead of message for '%s'", key)
			}
			if zhMessage == key {
				t.Errorf("Chinese manager returned key instead of message for '%s'", key)
			}

			// Messages should be different (since they're in different languages)
			if enMessage == zhMessage && enMessage != "" {
				t.Errorf("English and Chinese messages are identical for key '%s': %s", key, enMessage)
			}

			// Both should be non-empty
			if enMessage == "" {
				t.Errorf("English message is empty for key '%s'", key)
			}
			if zhMessage == "" {
				t.Errorf("Chinese message is empty for key '%s'", key)
			}
		})
	}
}

func TestManager_ErrorHandling(t *testing.T) {
	manager, err := NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Test with nil args
	result := manager.GetWithArgs("provider_info", nil)
	if result == "" {
		t.Error("GetWithArgs should handle nil args gracefully")
	}

	// Test with wrong number of args
	result = manager.GetWithArgs("provider_info", "only_one_arg")
	if result == "" {
		t.Error("GetWithArgs should handle wrong number of args gracefully")
	}

	// Test setting invalid language should return error
	originalLang := manager.currentLanguage
	err = manager.SetLanguage("invalid_language")
	if err == nil {
		t.Error("Expected error when setting invalid language")
	}

	// Should still work (fallback behavior)
	message := manager.Get("ai_not_configured")
	if message == "" {
		t.Error("Manager should still work after failed language setting")
	}

	// Language should not have changed if invalid
	if manager.currentLanguage == "invalid_language" {
		t.Error("Manager should not change to invalid language")
	}

	// Should still be original language
	if manager.currentLanguage != originalLang {
		t.Errorf("Expected language to remain '%s', got '%s'", originalLang, manager.currentLanguage)
	}
}

func TestManager_LanguageSpecificContent(t *testing.T) {
	testCases := []struct {
		name          string
		language      string
		messageKey    string
		shouldContain string
	}{
		{
			name:          "English AI not configured",
			language:      "en_au",
			messageKey:    "ai_not_configured",
			shouldContain: "AI is not configured",
		},
		{
			name:          "Chinese AI not configured",
			language:      "zh_cn",
			messageKey:    "ai_not_configured",
			shouldContain: "AI 未配置",
		},
		{
			name:          "English help connect",
			language:      "en_au",
			messageKey:    "help_connect",
			shouldContain: "Connect to a database connection",
		},
		{
			name:          "Chinese help connect",
			language:      "zh_cn",
			messageKey:    "help_connect",
			shouldContain: "连接到数据库连接",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			manager, err := NewManager(tc.language)
			if err != nil {
				t.Fatalf("Failed to create manager for language '%s': %v", tc.language, err)
			}

			message := manager.Get(tc.messageKey)
			if !strings.Contains(message, tc.shouldContain) {
				t.Errorf("Expected message to contain '%s', but got: %s", tc.shouldContain, message)
			}
		})
	}
}

// Benchmark tests
func BenchmarkManager_Get(b *testing.B) {
	manager, err := NewManager("en_au")
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Get("ai_not_configured")
	}
}

func BenchmarkManager_GetWithArgs(b *testing.B) {
	manager, err := NewManager("en_au")
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetWithArgs("provider_info", "openrouter", "claude-3.5-sonnet", 100, 50)
	}
}

func BenchmarkManager_SetLanguage(b *testing.B) {
	manager, err := NewManager("en_au")
	if err != nil {
		b.Fatalf("Failed to create manager: %v", err)
	}

	languages := []string{"en_au", "zh_cn"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.SetLanguage(languages[i%len(languages)])
	}
}
