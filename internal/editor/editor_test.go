package editor

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func TestEditorInitialization(t *testing.T) {
	// Test that the editor struct can be created
	// We can't easily test with a real connection, so we'll just test the struct fields
	
	editor := &Editor{
		textarea: textarea.New(),
		viewport: viewport.New(80, 10),
		tables:   []string{"users", "posts"},
		columns:  make(map[string][]string),
	}

	if editor == nil {
		t.Fatal("Editor should not be nil")
	}

	// Test that fields are properly initialized by checking their behavior
	ta := editor.textarea
	if ta.Value() != "" {
		t.Error("New textarea should be empty")
	}

	vp := editor.viewport
	if vp.Width == 0 && vp.Height == 0 {
		// This is expected for a new viewport
		t.Log("Viewport initialized with default dimensions")
	}

	if len(editor.tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(editor.tables))
	}
}

func TestEditorAutocomplete(t *testing.T) {
	editor := &Editor{
		tables:  []string{"users", "posts"},
		columns: make(map[string][]string),
	}

	// Test basic autocomplete functionality
	suggestions := editor.Autocomplete("")
	
	// Should have some suggestions
	if len(suggestions) == 0 {
		t.Error("Autocomplete should return some suggestions for empty input")
	}

	// Test with partial input
	suggestions = editor.Autocomplete("SEL")
	
	found := false
	for _, suggestion := range suggestions {
		if suggestion == "SELECT" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Autocomplete should suggest SELECT for 'SEL'")
	}
}

func TestEditorIsSQLQuery(t *testing.T) {
	editor := &Editor{}

	testCases := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "SELECT query",
			input:    "SELECT * FROM users",
			expected: true,
		},
		{
			name:     "INSERT query",
			input:    "INSERT INTO users (name) VALUES ('John')",
			expected: true,
		},
		{
			name:     "Non-SQL text",
			input:    "This is just regular text",
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := editor.isSQLQuery(tc.input)
			if result != tc.expected {
				t.Errorf("Expected isSQLQuery('%s') to return %v, got %v", tc.input, tc.expected, result)
			}
		})
	}
}

// Mock model for testing Bubbletea integration
type mockModel struct {
	textarea textarea.Model
	viewport viewport.Model
	width    int
	height   int
	ready    bool
}

func (m mockModel) Init() tea.Cmd {
	return nil
}

func (m mockModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m, nil
}

func (m mockModel) View() string {
	return "Mock view"
}

func TestEditorBubbleteaIntegration(t *testing.T) {
	// Test that Editor implements the tea.Model interface
	editor := &Editor{
		textarea: textarea.New(),
		viewport: viewport.New(80, 10),
	}

	var _ tea.Model = editor

	// Test basic styling doesn't panic
	style := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62"))

	text := style.Render("Test")
	if text == "" {
		t.Error("Styled text should not be empty")
	}
}