package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
)

// MarkdownRenderer handles markdown rendering with consistent styling
type MarkdownRenderer struct {
	width  int
	height int
}

// NewMarkdownRenderer creates a new markdown renderer with terminal dimensions
func NewMarkdownRenderer() *MarkdownRenderer {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width = 120 // fallback width
	}
	if height <= 0 {
		height = 30 // fallback height
	}

	return &MarkdownRenderer{
		width:  width,
		height: height,
	}
}

// RenderAndDisplay renders markdown content and displays it with consistent formatting
func (mr *MarkdownRenderer) RenderAndDisplay(markdown string) error {
	// Create a glamour renderer
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(mr.width),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		// Fall back to plain text if glamour fails
		fmt.Println("⚠️  Failed to create markdown renderer, showing plain text:")
		fmt.Print(markdown)
		return nil
	}

	// Render the markdown
	out, err := r.Render(markdown)
	if err != nil {
		// Fall back to plain text if rendering fails
		fmt.Println("⚠️  Failed to render markdown, showing plain text:")
		fmt.Print(markdown)
		return nil
	}

	// Display with consistent formatting
	mr.displayWithFormatting(out)
	return nil
}

// displayWithFormatting displays content with header and footer
func (mr *MarkdownRenderer) displayWithFormatting(content string) {
	// Print a header
	fmt.Println("\n📄 Query Results:")
	fmt.Println(strings.Repeat("─", min(mr.width, 80)))
	
	// Display the rendered markdown
	fmt.Print(content)
	
	// Print a footer
	fmt.Println(strings.Repeat("─", min(mr.width, 80)))
	fmt.Println()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}