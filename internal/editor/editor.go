package editor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"sqlterm/internal/core"
)

type Editor struct {
	textarea   textarea.Model
	viewport   viewport.Model
	keymap     keymap
	width      int
	height     int
	connection core.Connection
	tables     []string
	columns    map[string][]string
	results    string
	err        error
	ready      bool
}

type keymap struct {
	Execute key.Binding
	Quit    key.Binding
}

func NewEditor(conn core.Connection) *Editor {
	ta := textarea.New()
	ta.Placeholder = "Enter your SQL query here..."
	ta.Focus()

	vp := viewport.New(80, 10)

	km := keymap{
		Execute: key.NewBinding(
			key.WithKeys("ctrl+e", "enter"),
			key.WithHelp("ctrl+e/enter", "execute query"),
		),
		Quit: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "quit"),
		),
	}

	editor := &Editor{
		textarea:   ta,
		viewport:   vp,
		keymap:     km,
		ready:      false,
		connection: conn,
		columns:    make(map[string][]string),
	}

	// Load database schema information
	editor.loadSchema()

	return editor
}

func (e *Editor) Init() tea.Cmd {
	return textarea.Blink
}

func (e *Editor) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, e.keymap.Execute):
			return e, e.executeQuery
		case key.Matches(msg, e.keymap.Quit):
			return e, tea.Quit
		}

	case tea.WindowSizeMsg:
		e.width = msg.Width
		e.height = msg.Height
		e.textarea.SetWidth(msg.Width)
		e.viewport.Width = msg.Width
		e.viewport.Height = msg.Height - 10
		e.ready = true

	case error:
		e.err = msg
		return e, nil
	}

	e.textarea, cmd = e.textarea.Update(msg)
	cmds = append(cmds, cmd)

	return e, tea.Batch(cmds...)
}

func (e *Editor) View() string {
	if !e.ready {
		return "\n  Initializing..."
	}

	var s strings.Builder

	// Header
	s.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62")).
		Render("SQL Editor - Press Ctrl+E to execute, Esc to quit"))
	s.WriteString("\n\n")

	// Text area
	s.WriteString(e.textarea.View())
	s.WriteString("\n\n")

	// Results
	if e.results != "" {
		s.WriteString(lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("36")).
			Render("Results:"))
		s.WriteString("\n")
		s.WriteString(e.results)
		s.WriteString("\n\n")
	}

	if e.err != nil {
		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")).
			Render(fmt.Sprintf("Error: %v", e.err)))
		s.WriteString("\n\n")
	}

	// Help
	help := lipgloss.JoinHorizontal(lipgloss.Left,
		e.keymap.Execute.Help().Key,
		" ",
		e.keymap.Execute.Help().Desc,
		"  ",
		e.keymap.Quit.Help().Key,
		" ",
		e.keymap.Quit.Help().Desc,
	)
	s.WriteString(help)

	return s.String()
}

func (e *Editor) executeQuery() tea.Msg {
	query := e.textarea.Value()
	if query == "" {
		return fmt.Errorf("no query to execute")
	}

	// Simple validation - check if it looks like a SQL query
	if !e.isSQLQuery(query) {
		return fmt.Errorf("not a valid SQL query")
	}

	result, err := e.connection.Execute(query)
	if err != nil {
		return err
	}
	defer result.Close()

	// Format results as a simple table
	var sb strings.Builder
	
	// Header
	for i, col := range result.Columns {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(col.Name)
	}
	sb.WriteString("\n")
	
	// Separator
	for i := range result.Columns {
		if i > 0 {
			sb.WriteString(" | ")
		}
		sb.WriteString(strings.Repeat("-", len(result.Columns[i].Name)))
	}
	sb.WriteString("\n")
	
	// Data rows
	for row := range result.Itor() {
		for i, val := range row {
			if i > 0 {
				sb.WriteString(" | ")
			}
			sb.WriteString(val.String())
		}
		sb.WriteString("\n")
	}

	e.results = sb.String()
	return nil
}

func (e *Editor) loadSchema() {
	if e.connection == nil {
		return
	}

	// Load tables
	tables, err := e.connection.ListTables()
	if err != nil {
		e.err = err
		return
	}
	e.tables = tables

	// Load columns for each table
	for _, table := range tables {
		info, err := e.connection.DescribeTable(table)
		if err != nil {
			continue
		}
		
		var cols []string
		for _, col := range info.Columns {
			cols = append(cols, col.Name)
		}
		e.columns[table] = cols
	}
}

// Autocomplete provides SQL keyword and schema-based autocomplete
func (e *Editor) Autocomplete(input string) []string {
	var suggestions []string
	
	// SQL keywords
	keywords := []string{
		"SELECT", "FROM", "WHERE", "INSERT", "UPDATE", "DELETE",
		"CREATE", "DROP", "ALTER", "TABLE", "INDEX", "VIEW",
		"JOIN", "INNER", "LEFT", "RIGHT", "OUTER", "ON",
		"GROUP", "BY", "ORDER", "HAVING", "LIMIT", "OFFSET",
		"AND", "OR", "NOT", "IN", "LIKE", "BETWEEN",
		"AS", "DISTINCT", "COUNT", "SUM", "AVG", "MIN", "MAX",
	}
	
	// Add table names
	tables := e.tables
	
	// Get current word being typed
	words := strings.Fields(input)
	if len(words) > 0 {
		currentWord := strings.ToUpper(words[len(words)-1])
		
		// Suggest keywords
		for _, keyword := range keywords {
			if strings.HasPrefix(keyword, currentWord) {
				suggestions = append(suggestions, keyword)
			}
		}
		
		// Suggest tables
		for _, table := range tables {
			if strings.HasPrefix(strings.ToUpper(table), currentWord) {
				suggestions = append(suggestions, table)
			}
		}
	} else {
		// Suggest common starting keywords
		suggestions = append(suggestions, "SELECT", "INSERT", "UPDATE", "DELETE", "CREATE")
	}
	
	return suggestions
}

// isSQLQuery does basic validation to check if text looks like SQL
func (e *Editor) isSQLQuery(text string) bool {
	text = strings.TrimSpace(strings.ToUpper(text))
	
	// Common SQL keywords that indicate a SQL query
	keywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", 
		"DROP", "ALTER", "WITH", "BEGIN", "COMMIT", "ROLLBACK",
	}
	
	for _, keyword := range keywords {
		if strings.HasPrefix(text, keyword) {
			return true
		}
	}
	
	return false
}