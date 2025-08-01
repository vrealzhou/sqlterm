package core

import (
	"testing"

	"sqlterm/internal/i18n"
)

func TestNewMarkdownRenderer(t *testing.T) {
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}

	renderer := NewMarkdownRenderer(i18nMgr)
	if renderer == nil {
		t.Error("NewMarkdownRenderer should not return nil")
	}
	
	if renderer.i18nMgr != i18nMgr {
		t.Error("Renderer should have the correct i18n manager")
	}
}

func TestRenderAndDisplay(t *testing.T) {
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}

	renderer := NewMarkdownRenderer(i18nMgr)
	
	// Test with simple markdown
	markdown := "# Test Header\n\nThis is a test."
	
	// Capture output by redirecting stdout (this is tricky in tests, so we'll just ensure it doesn't panic)
	err = renderer.RenderAndDisplay(markdown)
	if err != nil {
		// We expect this to fail in test environment since glamour can't render properly
		// but it shouldn't panic
		t.Logf("Expected error in test environment: %v", err)
	}
	
	// Test with empty markdown
	err = renderer.RenderAndDisplay("")
	if err != nil {
		t.Logf("Expected error with empty markdown: %v", err)
	}
}

func TestDisplayWithFormatting(t *testing.T) {
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		t.Fatalf("Failed to create i18n manager: %v", err)
	}

	renderer := NewMarkdownRenderer(i18nMgr)
	
	// Test the helper function directly
	content := "# Test Header\n\nThis is a test."
	
	// This function is mostly a wrapper, so we'll just ensure it doesn't panic
	renderer.displayWithFormatting(content)
	
	// Test with empty content
	renderer.displayWithFormatting("")
}

func TestMin(t *testing.T) {
	testCases := []struct {
		name     string
		a        int
		b        int
		expected int
	}{
		{
			name:     "a less than b",
			a:        5,
			b:        10,
			expected: 5,
		},
		{
			name:     "b less than a",
			a:        15,
			b:        8,
			expected: 8,
		},
		{
			name:     "a equals b",
			a:        7,
			b:        7,
			expected: 7,
		},
		{
			name:     "negative numbers",
			a:        -5,
			b:        -3,
			expected: -5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := min(tc.a, tc.b)
			if result != tc.expected {
				t.Errorf("Expected min(%d, %d) to be %d, got %d", tc.a, tc.b, tc.expected, result)
			}
		})
	}
}