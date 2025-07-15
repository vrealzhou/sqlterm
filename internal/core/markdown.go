package core

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/glamour"
	"golang.org/x/term"
	"sqlterm/internal/i18n"
)

// MarkdownRenderer handles markdown rendering with consistent styling
type MarkdownRenderer struct {
	width   int
	height  int
	i18nMgr *i18n.Manager
}

// NewMarkdownRenderer creates a new markdown renderer with terminal dimensions
func NewMarkdownRenderer(i18nMgr *i18n.Manager) *MarkdownRenderer {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || width <= 0 {
		width = 120 // fallback width
	}
	if height <= 0 {
		height = 30 // fallback height
	}

	return &MarkdownRenderer{
		width:   width,
		height:  height,
		i18nMgr: i18nMgr,
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
		fmt.Println(mr.i18nMgr.Get("markdown_render_failed_plain_text"))
		fmt.Print(markdown)
		return nil
	}

	// Render the markdown
	out, err := r.Render(markdown)
	if err != nil {
		// Fall back to plain text if rendering fails
		fmt.Println(mr.i18nMgr.Get("markdown_render_failed_showing_plain"))
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
	fmt.Println(mr.i18nMgr.Get("query_results_plain_header"))
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