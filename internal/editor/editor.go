package editor

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"sqlterm/internal/core"
)

type Mode int

const (
	Normal Mode = iota
	Insert
	Visual
	Command
)

type Editor struct {
	textarea    textarea.Model
	viewport    viewport.Model
	keymap      KeyMap
	width       int
	height      int
	connection  core.Connection
	tables      []string
	columns     map[string][]string
	results     string
	err         error
	ready       bool
	mode        Mode
	statusBar   string
	history     []string
	historyPos  int
	completions []string
	completionPos int
	sync.Mutex
}

type KeyMap struct {
	Execute   key.Binding
	Quit      key.Binding
	Insert    key.Binding
	Mode      key.Binding
	Up        key.Binding
	Down      key.Binding
	Left      key.Binding
	Right     key.Binding
	Save      key.Binding
	Help      key.Binding
	Autocomplete key.Binding
}

func NewEditor(conn core.Connection) *Editor {
	ta := textarea.New()
	ta.Placeholder = "Enter your SQL query here..."
	ta.Focus()
	ta.ShowLineNumbers = true
	
	vp := viewport.New(80, 10)

	km := KeyMap{
		Execute: key.NewBinding(
			key.WithKeys("ctrl+e", "enter"),
			key.WithHelp("ctrl+e/enter", "execute query"),
		),
		Quit: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
			key.WithHelp("esc", "quit"),
		),
		Insert: key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "insert mode"),
		),
		Mode: key.NewBinding(
			key.WithKeys(":"), // vi command mode
			key.WithHelp(":", "command mode"),
		),
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down"),
			key.WithHelp("↓", "move down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "move left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "move right"),
		),
		Save: key.NewBinding(
			key.WithKeys("ctrl+s"),
			key.WithHelp("ctrl+s", "save query"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Autocomplete: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("tab", "autocomplete"),
		),
	}

	editor := &Editor{
		textarea:    ta,
		viewport:    vp,
		keymap:      km,
		ready:       false,
		connection:  conn,
		columns:     make(map[string][]string),
		mode:        Insert, // Start in insert mode for easier use
		history:     make([]string, 0),
		historyPos:  -1,
		completions: make([]string, 0),
		completionPos: -1,
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

	// Handle different modes
	switch e.mode {
	case Normal:
		return e.handleNormalMode(msg)
	case Insert:
		return e.handleInsertMode(msg)
	case Visual:
		return e.handleVisualMode(msg)
	case Command:
		return e.handleCommandMode(msg)
	}

	// Common message handling
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

func (e *Editor) handleNormalMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "i":
			e.mode = Insert
			e.statusBar = "INSERT MODE"
			return e, nil
		case ":":
			e.mode = Command
			e.statusBar = "COMMAND MODE"
			return e, nil
		case "h":
			// Move cursor left - need to handle properly with textarea functions
		case "j":
			// Move cursor down
		case "k":
			// Move cursor up
		case "l":
			// Move cursor right
		case "x":
			// Delete character
		case "dd":
			// Delete line
		case "yy":
			// Yank line
		case "p":
			// Paste
		case "u":
			// Undo
		case "ctrl+r":
			// Redo
		case "gg":
			// Go to beginning
		case "G":
			// Go to end
		case "/":
			// Search
		case "n":
			// Next search result
		case "N":
			// Previous search result
		case "v":
			// Visual mode
			e.mode = Visual
		case "V":
			// Visual line mode
		}
	}
	return e, nil
}

func (e *Editor) handleInsertMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			e.mode = Normal
			e.statusBar = "NORMAL MODE"
			return e, nil
		case "ctrl+s":
			e.saveQuery()
			return e, nil
		case "tab":
			// Handle autocompletion
			e.showCompletions()
			return e, nil
		}
	}
	
	// For other keys, let the textarea handle them
	e.textarea, cmd = e.textarea.Update(msg)
	return e, cmd
}

func (e *Editor) handleVisualMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			e.mode = Normal
			e.statusBar = "NORMAL MODE"
			return e, nil
		case "y":
			// Yank selection
		case "d":
			// Delete selection
		case "x":
			// Delete character
		}
	}
	return e, nil
}

func (e *Editor) handleCommandMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			e.mode = Normal
			e.statusBar = "NORMAL MODE"
			return e, nil
		case "enter":
			// Execute command
			e.mode = Normal
			e.statusBar = "NORMAL MODE"
			return e, nil
		}
	}
	return e, nil
}

func (e *Editor) showCompletions() {
	// Get the text up to current cursor position
	value := e.textarea.Value()
	cursorPos := len(value) // Simplified, would need actual cursor position
	
	// Find the word we're completing
	var word string
	for i := cursorPos - 1; i >= 0; i-- {
		if i >= len(value) {
			break
		}
		char := string(value[i])
		if char == " " || char == "\n" || char == "\t" || char == "(" || char == "," {
			word = value[i+1:cursorPos]
			break
		}
		if i == 0 {
			word = value[0:cursorPos]
		}
	}
	
	e.completions = e.Autocomplete(word)
	e.completionPos = 0
	
	// If we have completions, cycle through them
	if len(e.completions) > 0 {
		// Replace current word with first completion
		if word != "" && cursorPos <= len(value) {
			newValue := value[:cursorPos-len(word)] + e.completions[0]
			if cursorPos < len(value) {
				newValue += value[cursorPos:]
			}
			e.textarea.SetValue(newValue)
		}
	}
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

	// Mode indicator
	modeText := "NORMAL"
	switch e.mode {
	case Insert:
		modeText = "INSERT"
	case Visual:
		modeText = "VISUAL"
	case Command:
		modeText = "COMMAND"
	}
	
	modeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("0")).
		Padding(0, 1).
		Render(modeText)
		
	s.WriteString(modeStyle)
		
	if e.statusBar != "" {
		s.WriteString(" " + e.statusBar)
	}
	s.WriteString("\n\n")

	// Text area
	s.WriteString(e.textarea.View())
	s.WriteString("\n\n")

	// Show completions if available
	if len(e.completions) > 0 && e.mode == Insert {
		s.WriteString(lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Render("Completions: "))
		for i, completion := range e.completions {
			if i == e.completionPos {
				s.WriteString(lipgloss.NewStyle().
					Background(lipgloss.Color("205")).
					Foreground(lipgloss.Color("0")).
					Render(" " + completion + " "))
			} else {
				s.WriteString(" " + completion + " ")
			}
		}
		s.WriteString("\n\n")
	}

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
		"  ",
		e.keymap.Autocomplete.Help().Key,
		" ",
		e.keymap.Autocomplete.Help().Desc,
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
	e.history = append(e.history, query)
	e.historyPos = len(e.history) - 1
	e.completions = make([]string, 0) // Clear completions
	e.completionPos = -1
	return nil
}

func (e *Editor) saveQuery() {
	query := e.textarea.Value()
	if query != "" {
		e.history = append(e.history, query)
		e.historyPos = len(e.history) - 1
		e.statusBar = "Query saved to history"
	}
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
	
	// SQL keywords - extended list
	keywords := []string{
		"SELECT", "FROM", "WHERE", "INSERT", "UPDATE", "DELETE",
		"CREATE", "DROP", "ALTER", "TABLE", "INDEX", "VIEW",
		"JOIN", "INNER", "LEFT", "RIGHT", "OUTER", "ON",
		"GROUP", "BY", "ORDER", "HAVING", "LIMIT", "OFFSET",
		"AND", "OR", "NOT", "IN", "LIKE", "BETWEEN",
		"AS", "DISTINCT", "COUNT", "SUM", "AVG", "MIN", "MAX",
		"PRIMARY", "KEY", "FOREIGN", "REFERENCES", "CONSTRAINT",
		"UNIQUE", "CHECK", "DEFAULT", "NULL", "NOT NULL",
		"INT", "INTEGER", "VARCHAR", "TEXT", "BOOLEAN", "DATE",
		"TIMESTAMP", "SERIAL", "BIGINT", "SMALLINT", "DECIMAL",
		"REAL", "DOUBLE", "FLOAT", "CHAR", "BLOB", "JSON",
		"BEGIN", "COMMIT", "ROLLBACK", "TRANSACTION",
		"UNION", "ALL", "EXCEPT", "INTERSECT",
		"CASE", "WHEN", "THEN", "ELSE", "END",
		"EXISTS", "ANY", "SOME", "ALL",
		"ASC", "DESC", "COLLATE",
	}
	
	// Add table names
	tables := e.tables
	
	if input != "" {
		upperInput := strings.ToUpper(input)
		
		// Suggest keywords
		for _, keyword := range keywords {
			if strings.HasPrefix(keyword, upperInput) {
				suggestions = append(suggestions, keyword)
			}
		}
		
		// Suggest tables
		for _, table := range tables {
			if strings.HasPrefix(strings.ToUpper(table), upperInput) {
				suggestions = append(suggestions, table)
			}
		}
		
		// For better column suggestions, we'd need to track the context
		// For now, just suggest all columns if input matches any column prefix
		for _, cols := range e.columns {
			for _, col := range cols {
				if strings.HasPrefix(strings.ToUpper(col), upperInput) {
					// Check if not already in suggestions
					found := false
					for _, s := range suggestions {
						if strings.ToUpper(s) == strings.ToUpper(col) {
							found = true
							break
						}
					}
					if !found {
						suggestions = append(suggestions, col)
					}
				}
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