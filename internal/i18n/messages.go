package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed *.json
var messageFiles embed.FS

// Message represents a localized message
type Message struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

// Messages holds all messages for a language
type Messages struct {
	Language string    `json:"language"`
	Messages []Message `json:"messages"`
}

// Manager handles internationalization
type Manager struct {
	currentLanguage string
	messages        map[string]map[string]string // language -> message_id -> text
}

// NewManager creates a new i18n manager
func NewManager(language string) (*Manager, error) {
	if language == "" {
		return nil, fmt.Errorf("language cannot be empty")
	}

	manager := &Manager{
		currentLanguage: language,
		messages:        make(map[string]map[string]string),
	}

	if err := manager.loadMessages(); err != nil {
		return nil, fmt.Errorf("failed to load messages: %w", err)
	}

	// Validate that the requested language exists
	if _, exists := manager.messages[language]; !exists {
		return nil, fmt.Errorf("language '%s' not supported", language)
	}

	return manager, nil
}

// loadMessages loads all message files from embedded filesystem
func (m *Manager) loadMessages() error {
	files, err := messageFiles.ReadDir(".")
	if err != nil {
		return fmt.Errorf("failed to read message files: %w", err)
	}

	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".json") {
			continue
		}

		// Extract language from filename (e.g., "en_au.json" -> "en_au")
		language := strings.TrimSuffix(file.Name(), ".json")

		content, err := messageFiles.ReadFile(file.Name())
		if err != nil {
			return fmt.Errorf("failed to read message file %s: %w", file.Name(), err)
		}

		var messages Messages
		if err := json.Unmarshal(content, &messages); err != nil {
			return fmt.Errorf("failed to parse message file %s: %w", file.Name(), err)
		}

		// Build message map for this language
		langMessages := make(map[string]string)
		for _, msg := range messages.Messages {
			langMessages[msg.ID] = msg.Text
		}

		m.messages[language] = langMessages
	}

	return nil
}

// Get retrieves a localized message by ID
func (m *Manager) Get(messageID string) string {
	// Try current language first
	if langMessages, exists := m.messages[m.currentLanguage]; exists {
		if message, exists := langMessages[messageID]; exists {
			return message
		}
	}

	// Fallback to English if message not found in current language
	if langMessages, exists := m.messages["en_au"]; exists {
		if message, exists := langMessages[messageID]; exists {
			return message
		}
	}

	// Return message ID if not found (for debugging)
	return fmt.Sprintf("[%s]", messageID)
}

// GetWithArgs retrieves a localized message by ID and formats it with arguments
func (m *Manager) GetWithArgs(messageID string, args ...interface{}) string {
	message := m.Get(messageID)
	return fmt.Sprintf(message, args...)
}

// SetLanguage changes the current language
func (m *Manager) SetLanguage(language string) error {
	if language == "" {
		return fmt.Errorf("language cannot be empty")
	}

	if _, exists := m.messages[language]; !exists {
		return fmt.Errorf("language '%s' not supported", language)
	}

	m.currentLanguage = language
	return nil
}

// GetCurrentLanguage returns the current language
func (m *Manager) GetCurrentLanguage() string {
	return m.currentLanguage
}

// GetAvailableLanguages returns all available languages
func (m *Manager) GetAvailableLanguages() []string {
	var languages []string
	for lang := range m.messages {
		languages = append(languages, lang)
	}
	return languages
}
