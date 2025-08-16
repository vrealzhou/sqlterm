// Package main implements a TUI application with multiple views using bubbletea.
// First view shows a list of database connections on the left panel and displays
// selected connection details with text inputs on the right panel.
// Press Ctrl+S to save changes and Enter to connect to the selected connection
// and jump to the second view. Second view shows a list of tables on the left
// panel and displays selected table details on the right panel.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vrealzhou/sqlterm/internal/config"
)

var (
	docStyle  = lipgloss.NewStyle().Margin(1, 2)
	colors    = []string{"#FF87D7", "#7FD4FF", "#7DFF87", "#FF7D7D", "#D787FF"}
	activeClr = lipgloss.Color(colors[0])
)

type state int

const (
	connectionState state = iota
	tableState
)

// focusState represents which element is focused in connection view
type focusState int

const (
	focusConnectionList focusState = iota
	focusConnectionDetails
)

// model represents the application state
type model struct {
	state       state
	focusState  focusState
	conns       list.Model
	connInputs  []textinput.Model
	connFocused int
	tables      list.Model
	width       int
	height      int
	panelSplit  float64 // 0.0-1.0 representing left panel width ratio
	configs     *config.Manager
}

// Table represents a database table
type Table struct {
	Name    string
	Columns []Column
}

// Column represents a table column
type Column struct {
	Name     string
	Type     string
	Nullable bool
	Key      string
}

func main() {
	configs := config.NewManager()
	err := configs.LoadConfig()
	if err != nil {
		panic(fmt.Sprintf("Error loading config: %v", err))
	}
	connections := configs.ListConnections()

	// Create list items for connections
	connItems := make([]list.Item, 0, len(connections))
	for _, conn := range connections {
		connItems = append(connItems, connItem{conn: *conn})
	}

	// Create connections list
	connsList := list.New(connItems, list.NewDefaultDelegate(), 0, 0)
	connsList.Title = "Connections"
	connsList.KeyMap.Quit = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	)

	// Create table items
	tableItems := []list.Item{
		tableItem{Name: "users"},
		tableItem{Name: "products"},
		tableItem{Name: "orders"},
	}

	// Create tables list
	tablesList := list.New(tableItems, list.NewDefaultDelegate(), 0, 0)
	tablesList.Title = "Tables"
	tablesList.KeyMap.Quit = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	)

	// Initialize model
	m := model{
		state:      connectionState,
		focusState: focusConnectionList,
		conns:      connsList,
		tables:     tablesList,
		panelSplit: 0.4, // Default: 40% left, 60% right
		configs:    configs,
	}

	// Initialize connection inputs
	m.initConnInputs()

	// Create program with alt screen
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run program
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

func (m *model) LoadConfig() error {
	err := m.configs.LoadConfig()
	if err != nil {
		return err
	}
	return nil
}

func (m *model) saveConfigs() error {
	return m.configs.SaveConfig()
}

// updatePanelSizes updates the list sizes based on current dimensions and split ratio
func (m *model) updatePanelSizes() {
	if m.width == 0 || m.height == 0 {
		return
	}

	// Calculate panel dimensions based on split ratio
	leftWidth := int(float64(m.width)*m.panelSplit) - 4 // Account for borders
	panelHeight := m.height - 4                         // Account for borders and margins

	if m.state == connectionState {
		m.conns.SetSize(leftWidth, panelHeight)
	} else {
		m.tables.SetSize(leftWidth, panelHeight)
	}
}

// initConnInputs initializes the text inputs for connection details
func (m *model) initConnInputs() {
	inputs := make([]textinput.Model, 6)
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Connection Name"
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Database Type (mysql/postgres/sqlite)"

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "Host"

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "Port"

	inputs[4] = textinput.New()
	inputs[4].Placeholder = "Database"

	inputs[5] = textinput.New()
	inputs[5].Placeholder = "Username"

	m.connInputs = inputs
	m.connFocused = 0
}

// Init initializes the model
func (m model) Init() tea.Cmd {
	return textinput.Blink
}

// Update handles messages and updates the model
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+s":
			if m.state == connectionState && m.focusState == focusConnectionList {
				// Save connection logic when connection list is focused
				// TODO: Implement actual save functionality
				return m, nil
			}
		case "enter":
			if m.state == connectionState && m.focusState == focusConnectionList {
				// Connect from connection list - switch to table view
				m.state = tableState
				return m, nil
			}
		case "esc":
			if m.state == tableState {
				// Switch back to connection view
				m.state = connectionState
				m.focusState = focusConnectionList
				return m, nil
			}
		case "tab", "shift+tab":
			if m.state == connectionState {
				// Switch focus between connection list and details
				if msg.String() == "tab" {
					if m.focusState == focusConnectionList {
						m.focusState = focusConnectionDetails
						// Focus first input
						for i := range m.connInputs {
							if i == m.connFocused {
								m.connInputs[i].Focus()
							} else {
								m.connInputs[i].Blur()
							}
						}
					} else {
						m.focusState = focusConnectionList
						// Blur all inputs
						for i := range m.connInputs {
							m.connInputs[i].Blur()
						}
					}
				}
				return m, nil
			}
		case "up", "down":
			if m.state == connectionState {
				if m.focusState == focusConnectionDetails {
					if msg.String() == "down" {
						m.connFocused = (m.connFocused + 1) % len(m.connInputs)
					} else if msg.String() == "up" {
						m.connFocused--
						if m.connFocused < 0 {
							m.connFocused = len(m.connInputs) - 1
						}
					}
					for i := range m.connInputs {
						if i == m.connFocused {
							m.connInputs[i].Focus()
						} else {
							m.connInputs[i].Blur()
						}
					}
					return m, nil
				}
			}
		case "ctrl+left", "ctrl+right":
			// Resize panels with left/right arrow keys (only when not editing text)
			if m.state == connectionState && m.focusState == focusConnectionList {
				if msg.String() == "ctrl+left" && m.panelSplit > 0.2 {
					m.panelSplit -= 0.05 // Decrease left panel width
				} else if msg.String() == "ctrl+right" && m.panelSplit < 0.8 {
					m.panelSplit += 0.05 // Increase left panel width
				}
				// Update list sizes immediately
				m.updatePanelSizes()
				return m, nil
			}
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updatePanelSizes()
	}

	// Update the current view
	switch m.state {
	case connectionState:
		return m.updateConnections(msg)
	case tableState:
		return m.updateTables(msg)
	}

	return m, nil
}

// updateConnections updates the connection view
func (m model) updateConnections(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	// Handle special keys for connection details before text input processing
	if keyMsg, ok := msg.(tea.KeyMsg); ok && m.focusState == focusConnectionDetails {
		switch keyMsg.String() {
		case "ctrl+s":
			// Save connection logic - handled in main Update, but we can add specific feedback here
			// TODO: Implement actual save functionality
			return m, nil
		case "enter":
			// Connect from details panel - switch to table view
			m.state = tableState
			return m, nil
		}
	}

	oldIndex := m.conns.Index()
	m.conns, cmd = m.conns.Update(msg)
	cmds = append(cmds, cmd)
	// Update connection list only if it has focus
	if m.focusState == focusConnectionList {
		// Update inputs only if selection actually changed
		if m.conns.Index() != oldIndex && m.conns.SelectedItem() != nil {
			connItem := m.conns.SelectedItem().(connItem)
			conn := connItem.conn
			m.connInputs[0].SetValue(conn.Name)
			m.connInputs[1].SetValue(conn.DatabaseType.String())
			m.connInputs[2].SetValue(conn.Host)
			m.connInputs[3].SetValue(strconv.Itoa(conn.Port))
			m.connInputs[4].SetValue(conn.Database)
			m.connInputs[5].SetValue(conn.Username)
		}
	}

	// Update text inputs only if details have focus (and key wasn't handled above)
	if m.focusState == focusConnectionDetails {
		for i := range m.connInputs {
			m.connInputs[i], cmd = m.connInputs[i].Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// updateTables updates the table view
func (m model) updateTables(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.tables, cmd = m.tables.Update(msg)
	return m, cmd
}

// View renders the UI
func (m model) View() string {
	switch m.state {
	case connectionState:
		return m.connectionView()
	case tableState:
		return m.tableView()
	default:
		return "Unknown state"
	}
}

// connectionView renders the connection view with two panels
func (m model) connectionView() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Left panel: connection list
	// Calculate proper panel dimensions using split ratio
	leftWidth := int(float64(m.width)*m.panelSplit) - 2
	rightWidth := m.width - leftWidth - 2
	panelHeight := m.height - 2

	leftPanelStyle := lipgloss.NewStyle().
		Width(leftWidth).
		Height(panelHeight).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63"))
	if m.focusState == focusConnectionList {
		leftPanelStyle = leftPanelStyle.BorderForeground(activeClr)
	}
	leftPanel := leftPanelStyle.Render(m.conns.View())

	// Right panel: connection details

	rightPanelStyle := lipgloss.NewStyle().
		Width(rightWidth).
		Height(panelHeight).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63"))
	if m.focusState == focusConnectionDetails {
		rightPanelStyle = rightPanelStyle.BorderForeground(activeClr)
	}
	rightPanel := rightPanelStyle.Render(m.connectionDetailsView())

	// Combine panels
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

// connectionDetailsView renders the connection details panel
func (m model) connectionDetailsView() string {
	if len(m.connInputs) == 0 {
		return "No connection selected"
	}

	// Focus style for inputs
	focusedStyle := lipgloss.NewStyle().Foreground(activeClr)

	// Style inputs
	inputViews := make([]string, len(m.connInputs))
	for i, input := range m.connInputs {
		if i == m.connFocused {
			inputViews[i] = focusedStyle.Render("> " + input.View())
		} else {
			inputViews[i] = "  " + input.View()
		}
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeClr).
		MarginBottom(1).
		Render("Connection Details")

	inputs := lipgloss.JoinVertical(lipgloss.Left,
		inputViews[0],
		inputViews[1],
		inputViews[2],
		inputViews[3],
		inputViews[4],
		inputViews[5],
	)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("↑/↓: navigate • Tab: switch panels • ←/→: resize panels • Ctrl+S: save • Enter: connect")

	panel := lipgloss.JoinVertical(lipgloss.Left,
		title,
		inputs,
		"",
		help,
	)

	rightWidth := m.width - int(float64(m.width)*m.panelSplit) - 2
	return lipgloss.NewStyle().
		Width(rightWidth-6).
		Height(m.height-6).
		Padding(1, 2).
		Render(panel)
}

// tableView renders the table view with two panels
func (m model) tableView() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	// Left panel: tables list
	// Calculate proper panel dimensions using split ratio
	leftWidth := int(float64(m.width)*m.panelSplit) - 2
	rightWidth := m.width - leftWidth - 2
	panelHeight := m.height - 4 // Extra space for help text

	leftPanel := lipgloss.NewStyle().
		Width(leftWidth).
		Height(panelHeight).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Render(m.tables.View())

	// Right panel: table details

	rightPanel := lipgloss.NewStyle().
		Width(rightWidth).
		Height(panelHeight).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")).
		Render(m.tableDetailsView())

	// Combine panels
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel) +
		lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("\n\nEsc: back to connections • Ctrl+C: exit")
}

// tableDetailsView renders the table details panel
func (m model) tableDetailsView() string {
	item := m.tables.SelectedItem()
	if item == nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7FD4FF")).
			Render("Select a table to view details")
	}

	tableName := item.(tableItem).Name

	// Mock table details
	columns := []Column{
		{Name: "id", Type: "integer", Nullable: false, Key: "PRI"},
		{Name: "name", Type: "varchar(255)", Nullable: false},
		{Name: "email", Type: "varchar(255)", Nullable: true},
		{Name: "created_at", Type: "timestamp", Nullable: false},
	}

	// Format columns
	var columnLines []string
	for _, col := range columns {
		nullable := ""
		if col.Nullable {
			nullable = " nullable"
		}
		key := ""
		if col.Key != "" {
			key = " " + col.Key
		}
		columnLines = append(columnLines, fmt.Sprintf("• %s (%s)%s%s", col.Name, col.Type, nullable, key))
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeClr).
		MarginBottom(1).
		Render(tableName)

	columnsTitle := lipgloss.NewStyle().
		Bold(true).
		MarginTop(1).
		MarginBottom(1).
		Render("Columns:")

	columnList := lipgloss.NewStyle().
		PaddingLeft(2).
		Render(strings.Join(columnLines, "\n"))

	panel := lipgloss.JoinVertical(lipgloss.Left,
		title,
		columnsTitle,
		columnList,
	)

	rightWidth := m.width - int(float64(m.width)*m.panelSplit) - 2
	return lipgloss.NewStyle().
		Width(rightWidth-6).
		Height(m.height-6).
		Padding(1, 2).
		Render(panel)
}

// connItem represents a connection list item
type connItem struct {
	conn config.ConnectionConfig
}

func (i connItem) Title() string {
	// Truncate long connection names to prevent layout issues
	maxLen := 30
	if len(i.conn.Name) > maxLen {
		return i.conn.Name[:maxLen-3] + "..."
	}
	return i.conn.Name
}
func (i connItem) Description() string {
	var desc string
	if i.conn.DatabaseType == config.SQLite {
		desc = i.conn.DatabaseType.String() + ":" + i.conn.Database
	} else {
		desc = i.conn.DatabaseType.String() + "://" + i.conn.Host + ":" + strconv.Itoa(i.conn.Port) + "/" + i.conn.Database
	}

	// Truncate long descriptions to prevent layout issues
	maxLen := 50
	if len(desc) > maxLen {
		return desc[:maxLen-3] + "..."
	}
	return desc
}
func (i connItem) FilterValue() string { return i.conn.Name }

// tableItem represents a table list item
type tableItem struct {
	Name string
}

func (i tableItem) Title() string       { return i.Name }
func (i tableItem) Description() string { return "Table with various columns" }
func (i tableItem) FilterValue() string { return i.Name }
