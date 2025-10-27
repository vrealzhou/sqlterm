// Package main implements a TUI application with multiple views using bubbletea.
// First view shows a list of database connections on the left panel and displays
// selected connection details with text inputs on the right panel.
// Press Ctrl+S to save changes and Enter to connect to the selected connection
// and jump to the second view. Second view shows a list of tables on the left
// panel and displays selected table details on the right panel.
package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/vrealzhou/sqlterm/internal/config"
	"github.com/vrealzhou/sqlterm/internal/database"
)

var (
	docStyle  = lipgloss.NewStyle().Margin(1, 2)
	colors    = []string{"#FF87D7", "#7FD4FF", "#7DFF87", "#FF7D7D", "#D787FF"}
	activeClr = lipgloss.Color(colors[0])
)

const (
	minListWidth       = 24
	minChatWidth       = 26
	minEditorWidth     = 32
	minEditorHeight    = 6
	queryChatRatio     = 0.28
	queryEditorTopPort = 0.55
	maxQueryRows       = 50
)

type state int

const (
	connectionState state = iota
	tableState
	queryState
)

// focusState represents which element is focused in connection view
type focusState int

const (
	focusConnectionList focusState = iota
	focusConnectionDetails
)

type queryFocus int

const (
	queryFocusTables queryFocus = iota
	queryFocusEditor
	queryFocusChat
)

// model represents the application state
type model struct {
	state              state
	focusState         focusState
	conns              list.Model
	connInputs         []textinput.Model
	connFocused        int
	connections        []*config.ConnectionConfig
	tables             list.Model
	tableInfos         map[string]*database.TableInfo
	width              int
	height             int
	panelSplit         float64 // 0.0-1.0 representing left panel width ratio
	configs            *config.Manager
	connection         database.Connection
	statusMsg          string
	errorMsg           string
	isConnecting       bool
	loadingTable       string
	queryInput         textarea.Model
	queryFocus         queryFocus
	isRunningQuery     bool
	queryResultColumns []string
	queryResultRows    [][]string
	queryEditorHeight  int
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

type connectionResultMsg struct {
	conn   database.Connection
	tables []string
	config *config.ConnectionConfig
	err    error
}

type tableInfoMsg struct {
	table string
	info  *database.TableInfo
	err   error
}

type queryResultMsg struct {
	columns []string
	rows    [][]string
	err     error
}

func main() {
	configs := config.NewManager()
	statusMsg := ""
	if err := configs.LoadConfig(); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			statusMsg = "No saved connections yet. Use the form to create one and press Ctrl+S to save."
		} else {
			statusMsg = fmt.Sprintf("⚠️ Unable to load saved connections: %v", err)
		}
	}

	// Build in-memory copy of connection configs so edits do not mutate manager internals until save
	connections := configs.ListConnections()
	connItems := make([]list.Item, 0, len(connections))
	connData := make([]*config.ConnectionConfig, 0, len(connections))
	for _, conn := range connections {
		if conn == nil {
			continue
		}
		copyConn := *conn
		connPtr := &copyConn
		connData = append(connData, connPtr)
		connItems = append(connItems, connItem{conn: connPtr})
	}

	if len(connData) == 0 {
		defaultConn := &config.ConnectionConfig{
			Name:         "",
			DatabaseType: config.PostgreSQL,
			Host:         "localhost",
			Port:         config.GetDefaultPort(config.PostgreSQL),
		}
		connData = append(connData, defaultConn)
		connItems = append(connItems, connItem{conn: defaultConn})
	}

	// Create connections list
	connsList := list.New(connItems, list.NewDefaultDelegate(), 0, 0)
	connsList.Title = "Connections"
	connsList.KeyMap.Quit = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	)

	// Create tables list (populated after connecting)
	tablesList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	tablesList.Title = "Tables"
	tablesList.KeyMap.Quit = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "quit"),
	)

	// Initialize model
	m := model{
		state:       connectionState,
		focusState:  focusConnectionList,
		conns:       connsList,
		tables:      tablesList,
		panelSplit:  0.4, // Default: 40% left, 60% right
		configs:     configs,
		connections: connData,
		tableInfos:  make(map[string]*database.TableInfo),
		statusMsg:   statusMsg,
		queryFocus:  queryFocusEditor,
	}

	// Initialize connection inputs
	m.initConnInputs()
	m.syncInputsWithSelection()
	m.initQueryEditor()

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

	leftWidth, midWidth, _, panelHeight := m.computeQueryLayout()
	m.conns.SetSize(leftWidth, panelHeight)
	m.tables.SetSize(leftWidth, panelHeight)

	editorWidth := max(minEditorWidth-4, midWidth-4)
	if editorWidth < 10 {
		editorWidth = 10
	}
	editorHeight := int(float64(panelHeight) * queryEditorTopPort)
	if editorHeight < minEditorHeight {
		editorHeight = minEditorHeight
	}
	m.queryEditorHeight = editorHeight
	m.queryInput.SetWidth(editorWidth)
	m.queryInput.SetHeight(editorHeight)
}

// initConnInputs initializes the text inputs for connection details
func (m *model) initConnInputs() {
	inputs := make([]textinput.Model, 8)
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Connection Name"
	inputs[0].Focus()

	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Database Type (mysql/postgres/sqlite)"

	inputs[2] = textinput.New()
	inputs[2].Placeholder = "Host (or file for sqlite)"

	inputs[3] = textinput.New()
	inputs[3].Placeholder = "Port"

	inputs[4] = textinput.New()
	inputs[4].Placeholder = "Database"

	inputs[5] = textinput.New()
	inputs[5].Placeholder = "Username"

	inputs[6] = textinput.New()
	inputs[6].Placeholder = "Password"
	inputs[6].EchoMode = textinput.EchoPassword
	inputs[6].EchoCharacter = '•'

	inputs[7] = textinput.New()
	inputs[7].Placeholder = "SSL (true/false)"

	m.connInputs = inputs
	m.connFocused = 0
}

func (m *model) initQueryEditor() {
	editor := textarea.New()
	editor.Placeholder = "SELECT * FROM table LIMIT 20;"
	editor.SetHeight(8)
	editor.SetWidth(60)
	editor.CharLimit = 0
	editor.ShowLineNumbers = true
	editor.FocusedStyle.CursorLineNumber = editor.FocusedStyle.CursorLineNumber.Foreground(activeClr)
	editor.FocusedStyle.Prompt = editor.FocusedStyle.Prompt.Foreground(activeClr)
	editor.BlurredStyle.CursorLineNumber = editor.BlurredStyle.CursorLineNumber.Foreground(lipgloss.Color("240"))
	editor.BlurredStyle.Prompt = editor.BlurredStyle.Prompt.Foreground(lipgloss.Color("240"))
	editor.SetValue("")
	editor.Blur()
	m.queryInput = editor
	m.queryEditorHeight = 8
}

func (m *model) setQueryFocus(next queryFocus) {
	m.queryFocus = next
	switch next {
	case queryFocusEditor:
		m.queryInput.Focus()
	default:
		m.queryInput.Blur()
	}
}

func (m *model) cycleQueryFocus(step int) {
	total := 3
	idx := (int(m.queryFocus) + step) % total
	if idx < 0 {
		idx += total
	}
	m.setQueryFocus(queryFocus(idx))
}

// selectedConnection returns the connection currently highlighted in the list.
func (m *model) selectedConnection() *config.ConnectionConfig {
	idx := m.conns.Index()
	if idx < 0 || idx >= len(m.connections) {
		return nil
	}
	return m.connections[idx]
}

// syncInputsWithSelection hydrates the text inputs with the selected connection config.
func (m *model) syncInputsWithSelection() {
	if len(m.connInputs) == 0 {
		return
	}

	conn := m.selectedConnection()
	if conn == nil {
		for i := range m.connInputs {
			m.connInputs[i].SetValue("")
		}
		return
	}

	m.connInputs[0].SetValue(conn.Name)
	m.connInputs[1].SetValue(conn.DatabaseType.String())
	m.connInputs[2].SetValue(conn.Host)
	if conn.Port > 0 {
		m.connInputs[3].SetValue(strconv.Itoa(conn.Port))
	} else {
		m.connInputs[3].SetValue("")
	}
	m.connInputs[4].SetValue(conn.Database)
	m.connInputs[5].SetValue(conn.Username)
	m.connInputs[6].SetValue(conn.Password)
	m.connInputs[7].SetValue(strconv.FormatBool(conn.SSL))
}

func (m *model) setStatus(msg string) {
	m.statusMsg = msg
	if msg != "" {
		m.errorMsg = ""
	}
}

func (m *model) setError(err error) {
	if err == nil {
		return
	}
	m.errorMsg = err.Error()
	m.statusMsg = ""
}

func (m *model) clearStatus() {
	m.statusMsg = ""
	m.errorMsg = ""
}

func (m *model) buildConfigFromInputs() (*config.ConnectionConfig, error) {
	if len(m.connInputs) < 8 {
		return nil, fmt.Errorf("connection form is incomplete")
	}

	name := strings.TrimSpace(m.connInputs[0].Value())
	if name == "" {
		return nil, fmt.Errorf("connection name is required")
	}

	dbTypeStr := strings.TrimSpace(m.connInputs[1].Value())
	var dbType config.DatabaseType
	var err error
	if dbTypeStr == "" {
		if existing := m.selectedConnection(); existing != nil {
			dbType = existing.DatabaseType
		} else {
			dbType = config.PostgreSQL
		}
	} else {
		dbType, err = config.ParseDatabaseType(dbTypeStr)
		if err != nil {
			return nil, err
		}
	}

	host := strings.TrimSpace(m.connInputs[2].Value())
	portStr := strings.TrimSpace(m.connInputs[3].Value())
	var port int
	if dbType == config.SQLite {
		port = 0
	} else {
		if portStr == "" {
			port = config.GetDefaultPort(dbType)
		} else {
			parsed, perr := strconv.Atoi(portStr)
			if perr != nil || parsed <= 0 {
				return nil, fmt.Errorf("invalid port: %s", portStr)
			}
			port = parsed
		}
		if host == "" {
			host = "localhost"
		}
	}

	databaseName := strings.TrimSpace(m.connInputs[4].Value())
	if databaseName == "" && dbType != config.SQLite {
		return nil, fmt.Errorf("database name is required for %s", dbType.String())
	}

	username := strings.TrimSpace(m.connInputs[5].Value())
	if username == "" && dbType != config.SQLite {
		return nil, fmt.Errorf("username is required for %s", dbType.String())
	}

	password := m.connInputs[6].Value()
	sslStr := strings.TrimSpace(m.connInputs[7].Value())
	sslEnabled := false
	if sslStr != "" {
		parsed, perr := strconv.ParseBool(sslStr)
		if perr != nil {
			return nil, fmt.Errorf("invalid SSL flag: %s", sslStr)
		}
		sslEnabled = parsed
	}

	return &config.ConnectionConfig{
		Name:         name,
		DatabaseType: dbType,
		Host:         host,
		Port:         port,
		Database:     databaseName,
		Username:     username,
		Password:     password,
		SSL:          sslEnabled,
	}, nil
}

func (m *model) saveConnectionConfig(cfg *config.ConnectionConfig, showStatus bool) tea.Cmd {
	if cfg == nil {
		return nil
	}

	idx := m.conns.Index()
	var prevName string
	if idx >= 0 && idx < len(m.connections) && m.connections[idx] != nil {
		prevName = m.connections[idx].Name
	}

	if prevName != "" && prevName != cfg.Name {
		if err := m.configs.DeleteConnection(prevName); err != nil {
			m.setError(err)
			return nil
		}
	}

	if err := m.configs.SaveConnection(cfg); err != nil {
		m.setError(err)
		return nil
	}

	var cmds []tea.Cmd
	if idx >= 0 && idx < len(m.connections) {
		m.connections[idx] = cfg
		if cmd := m.conns.SetItem(idx, connItem{conn: cfg}); cmd != nil {
			cmds = append(cmds, cmd)
		}
	} else {
		m.connections = append(m.connections, cfg)
		if cmd := m.conns.InsertItem(len(m.connections)-1, connItem{conn: cfg}); cmd != nil {
			cmds = append(cmds, cmd)
		}
		m.conns.Select(len(m.connections) - 1)
	}

	m.syncInputsWithSelection()
	if showStatus {
		m.setStatus(fmt.Sprintf("Saved connection %s", displayName(cfg)))
	}

	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

func (m *model) handleSave() tea.Cmd {
	cfg, err := m.buildConfigFromInputs()
	if err != nil {
		m.setError(err)
		return nil
	}

	return m.saveConnectionConfig(cfg, true)
}

func (m *model) handleConnect() tea.Cmd {
	if m.isConnecting {
		return nil
	}

	cfg, err := m.buildConfigFromInputs()
	if err != nil {
		m.setError(err)
		return nil
	}

	m.isConnecting = true
	m.tableInfos = make(map[string]*database.TableInfo)
	m.setStatus(fmt.Sprintf("Connecting to %s…", displayName(cfg)))

	return connectCmd(cfg)
}

func (m *model) addNewConnection() tea.Cmd {
	newConn := &config.ConnectionConfig{
		DatabaseType: config.PostgreSQL,
		Host:         "localhost",
		Port:         config.GetDefaultPort(config.PostgreSQL),
	}
	m.connections = append(m.connections, newConn)
	insertIndex := len(m.connections) - 1
	var cmds []tea.Cmd
	if cmd := m.conns.InsertItem(insertIndex, connItem{conn: newConn}); cmd != nil {
		cmds = append(cmds, cmd)
	}
	m.conns.Select(insertIndex)
	m.connFocused = 0
	for i := range m.connInputs {
		m.connInputs[i].SetValue("")
		if i == 0 {
			m.connInputs[i].Focus()
		} else {
			m.connInputs[i].Blur()
		}
	}
	m.syncInputsWithSelection()
	m.setStatus("New connection entry created. Fill in the form and press Ctrl+S to save.")
	if len(cmds) > 0 {
		return tea.Batch(cmds...)
	}
	return nil
}

func connectCmd(cfg *config.ConnectionConfig) tea.Cmd {
	if cfg == nil {
		return nil
	}
	cfgCopy := *cfg
	return func() tea.Msg {
		conn, err := database.NewConnection(&cfgCopy)
		if err != nil {
			return connectionResultMsg{err: err, config: &cfgCopy}
		}
		if err := conn.Ping(); err != nil {
			conn.Close()
			return connectionResultMsg{err: err, config: &cfgCopy}
		}
		tables, err := conn.ListTables()
		if err != nil {
			conn.Close()
			return connectionResultMsg{err: err, config: &cfgCopy}
		}
		sort.Strings(tables)
		return connectionResultMsg{conn: conn, tables: tables, config: &cfgCopy}
	}
}

func loadTableDetailsCmd(conn database.Connection, tableName string) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return tableInfoMsg{table: tableName, info: nil, err: fmt.Errorf("no active connection")}
		}
		info, err := conn.DescribeTable(tableName)
		return tableInfoMsg{table: tableName, info: info, err: err}
	}
}

func runQueryCmd(conn database.Connection, query string, limit int) tea.Cmd {
	return func() tea.Msg {
		if conn == nil {
			return queryResultMsg{err: fmt.Errorf("no active connection")}
		}
		if limit <= 0 {
			limit = maxQueryRows
		}
		result, err := conn.Execute(query)
		if err != nil {
			return queryResultMsg{err: err}
		}
		if result == nil {
			return queryResultMsg{err: fmt.Errorf("no result returned")}
		}
		defer result.Close()

		columns := result.ColumnNames()
		rows := make([][]string, 0, limit)
		count := 0
		for values := range result.Itor() {
			if result.Error() != nil {
				return queryResultMsg{err: result.Error()}
			}
			row := make([]string, len(values))
			for i, v := range values {
				row[i] = v.String()
			}
			rows = append(rows, row)
			count++
			if count >= limit {
				break
			}
		}
		if err := result.Error(); err != nil {
			return queryResultMsg{err: err}
		}

		return queryResultMsg{columns: columns, rows: rows}
	}
}

func displayName(cfg *config.ConnectionConfig) string {
	if cfg == nil {
		return ""
	}
	if cfg.Name != "" {
		return cfg.Name
	}
	if cfg.Database != "" {
		return cfg.Database
	}
	if cfg.Host != "" {
		return cfg.Host
	}
	return cfg.DatabaseType.String()
}

func (m *model) loadTableDetails(tableName string) tea.Cmd {
	if tableName == "" || m.connection == nil {
		return nil
	}
	if _, exists := m.tableInfos[tableName]; exists {
		return nil
	}
	m.loadingTable = tableName
	return loadTableDetailsCmd(m.connection, tableName)
}

func (m model) computeQueryLayout() (leftWidth, midWidth, chatWidth, panelHeight int) {
	if m.width <= 0 || m.height <= 0 {
		return minListWidth, minEditorWidth, minChatWidth, minEditorHeight
	}
	panelHeight = m.height - 4
	if panelHeight < minEditorHeight {
		panelHeight = minEditorHeight
	}
	leftWidth = int(float64(m.width)*m.panelSplit) - 2
	if leftWidth < minListWidth {
		leftWidth = minListWidth
	}
	chatWidth = int(float64(m.width) * queryChatRatio)
	if chatWidth < minChatWidth {
		chatWidth = minChatWidth
	}
	available := m.width - leftWidth - chatWidth - 4
	if available < minEditorWidth {
		deficit := minEditorWidth - available
		reducibleChat := chatWidth - minChatWidth
		if reducibleChat > 0 {
			reduction := min(deficit, reducibleChat)
			chatWidth -= reduction
			deficit -= reduction
		}
		if deficit > 0 {
			reducibleList := leftWidth - minListWidth
			if reducibleList > 0 {
				reduction := min(deficit, reducibleList)
				leftWidth -= reduction
				deficit -= reduction
			}
		}
		available = m.width - leftWidth - chatWidth - 4
		if available < minEditorWidth {
			available = minEditorWidth
		}
	}
	midWidth = available
	return
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func tableDisplayWidth(widths []int) int {
	if len(widths) == 0 {
		return 0
	}
	width := widths[0]
	for i := 1; i < len(widths); i++ {
		width += widths[i] + 3
	}
	return width
}

func indexOfMax(values []int) int {
	if len(values) == 0 {
		return -1
	}
	maxIdx := 0
	for i := 1; i < len(values); i++ {
		if values[i] > values[maxIdx] {
			maxIdx = i
		}
	}
	return maxIdx
}

func truncateCell(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}

func (m model) statusView() string {
	var lines []string
	if m.isConnecting {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("Connecting…"))
	}
	if m.loadingTable != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render(fmt.Sprintf("Loading %s…", m.loadingTable)))
	}
	if m.isRunningQuery {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("Running query…"))
	}
	if m.errorMsg != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("204")).Render("Error: "+m.errorMsg))
	}
	if m.statusMsg != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(lipgloss.Color("77")).Render(m.statusMsg))
	}
	if len(lines) == 0 {
		return ""
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
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
			if m.state == connectionState {
				return m, m.handleSave()
			}
		case "ctrl+n":
			if m.state == connectionState {
				return m, m.addNewConnection()
			}
		case "ctrl+q":
			if m.state == tableState {
				if m.connection == nil {
					m.setError(fmt.Errorf("connect to a database first"))
					return m, nil
				}
				m.state = queryState
				m.setQueryFocus(queryFocusEditor)
				m.setStatus("Query view. Type SQL and press Ctrl+Enter to run.")
				m.updatePanelSizes()
				return m, nil
			}
		case "enter":
			if m.state == connectionState {
				return m, m.handleConnect()
			}
		case "ctrl+t":
			if m.state == queryState {
				m.state = tableState
				m.setStatus("Table view. Press Ctrl+Q to open the query editor.")
				m.updatePanelSizes()
				return m, nil
			}
		case "esc":
			if m.state == queryState {
				m.state = tableState
				m.setStatus("Back to tables. Press Ctrl+Q to return to the query editor.")
				m.updatePanelSizes()
				return m, nil
			}
			if m.state == tableState {
				m.state = connectionState
				m.focusState = focusConnectionList
				m.setStatus("Returned to connections. Press Enter to reconnect.")
				m.updatePanelSizes()
				return m, nil
			}
		case "tab":
			if m.state == connectionState {
				// Switch focus between connection list and details
				if msg.String() == "tab" {
					if m.focusState == focusConnectionList {
						m.focusState = focusConnectionDetails
						m.connFocused = 0
						if m.connFocused >= len(m.connInputs) {
							m.connFocused = 0
						}
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
			} else if m.state == queryState {
				m.cycleQueryFocus(1)
				return m, nil
			}
		case "shift+tab":
			if m.state == connectionState {
				if m.focusState == focusConnectionList {
					m.focusState = focusConnectionDetails
					m.connFocused = 0
					if m.connFocused >= len(m.connInputs) {
						m.connFocused = 0
					}
					for i := range m.connInputs {
						if i == m.connFocused {
							m.connInputs[i].Focus()
						} else {
							m.connInputs[i].Blur()
						}
					}
				} else {
					m.focusState = focusConnectionList
					for i := range m.connInputs {
						m.connInputs[i].Blur()
					}
				}
				return m, nil
			} else if m.state == queryState {
				m.cycleQueryFocus(-1)
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
	case connectionResultMsg:
		m.isConnecting = false
		m.isRunningQuery = false
		if msg.err != nil {
			if msg.conn != nil {
				_ = msg.conn.Close()
			}
			m.setError(msg.err)
			return m, nil
		}
		if m.connection != nil {
			_ = m.connection.Close()
		}
		m.connection = msg.conn
		m.state = tableState
		m.focusState = focusConnectionList
		m.tableInfos = make(map[string]*database.TableInfo)
		m.queryResultColumns = nil
		m.queryResultRows = nil
		var cmds []tea.Cmd
		if persistCmd := m.saveConnectionConfig(msg.config, false); persistCmd != nil {
			cmds = append(cmds, persistCmd)
		}
		if m.errorMsg == "" {
			m.setStatus(fmt.Sprintf("Connected to %s", displayName(msg.config)))
		}
		var tableItems []list.Item
		for _, name := range msg.tables {
			tableItems = append(tableItems, tableItem{Name: name})
		}
		if cmd := m.tables.SetItems(tableItems); cmd != nil {
			cmds = append(cmds, cmd)
		}
		if len(msg.tables) > 0 {
			m.tables.Select(0)
			if loadCmd := m.loadTableDetails(msg.tables[0]); loadCmd != nil {
				cmds = append(cmds, loadCmd)
			}
		} else {
			m.loadingTable = ""
		}
		m.updatePanelSizes()
		if len(cmds) > 0 {
			return m, tea.Batch(cmds...)
		}
		return m, nil
	case tableInfoMsg:
		if msg.err != nil {
			m.setError(msg.err)
			if m.loadingTable == msg.table {
				m.loadingTable = ""
			}
			return m, nil
		}
		if m.tableInfos == nil {
			m.tableInfos = make(map[string]*database.TableInfo)
		}
		m.tableInfos[msg.table] = msg.info
		if m.loadingTable == msg.table {
			m.loadingTable = ""
		}
		return m, nil
	case queryResultMsg:
		m.isRunningQuery = false
		if msg.err != nil {
			m.setError(msg.err)
			return m, nil
		}
		m.queryResultColumns = msg.columns
		m.queryResultRows = msg.rows
		tail := ""
		if len(msg.rows) >= maxQueryRows {
			tail = fmt.Sprintf(" (showing first %d rows)", maxQueryRows)
		}
		m.setStatus(fmt.Sprintf("Query returned %d row(s)%s", len(msg.rows), tail))
		return m, nil
	}

	// Update the current view
	switch m.state {
	case connectionState:
		return m.updateConnections(msg)
	case tableState:
		return m.updateTables(msg)
	case queryState:
		return m.updateQuery(msg)
	}

	return m, nil
}

// updateConnections updates the connection view
func (m model) updateConnections(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	prevIndex := m.conns.Index()
	shouldUpdateList := true
	if m.focusState == focusConnectionDetails {
		if _, ok := msg.(tea.KeyMsg); ok {
			shouldUpdateList = false
		}
	}

	if shouldUpdateList {
		var cmd tea.Cmd
		m.conns, cmd = m.conns.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if m.conns.Index() != prevIndex {
			m.syncInputsWithSelection()
		}
	}

	// Update text inputs only if details have focus (and key wasn't handled above)
	if m.focusState == focusConnectionDetails {
		for i := range m.connInputs {
			var cmd tea.Cmd
			m.connInputs[i], cmd = m.connInputs[i].Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// updateTables updates the table view
func (m model) updateTables(msg tea.Msg) (tea.Model, tea.Cmd) {
	oldIndex := m.tables.Index()
	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.tables, cmd = m.tables.Update(msg)
	cmds = append(cmds, cmd)
	if m.tables.Index() != oldIndex {
		if item := m.tables.SelectedItem(); item != nil {
			if table, ok := item.(tableItem); ok {
				if loadCmd := m.loadTableDetails(table.Name); loadCmd != nil {
					cmds = append(cmds, loadCmd)
				}
			}
		}
	}
	return m, tea.Batch(cmds...)
}

func (m model) updateQuery(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch m.queryFocus {
	case queryFocusTables:
		oldIndex := m.tables.Index()
		var cmd tea.Cmd
		m.tables, cmd = m.tables.Update(msg)
		cmds = append(cmds, cmd)
		if m.tables.Index() != oldIndex {
			if item := m.tables.SelectedItem(); item != nil {
				if table, ok := item.(tableItem); ok {
					if loadCmd := m.loadTableDetails(table.Name); loadCmd != nil {
						cmds = append(cmds, loadCmd)
					}
				}
			}
		}
	case queryFocusEditor:
		var cmd tea.Cmd
		m.queryInput, cmd = m.queryInput.Update(msg)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.String() {
			case "ctrl+enter", "cmd+enter", "ctrl+j", "ctrl+e", "ctrl+r", "f5":
				if m.isRunningQuery {
					break
				}
				query := strings.TrimSpace(m.queryInput.Value())
				if query == "" {
					m.setStatus("Enter a SQL statement and press Ctrl+Enter to run.")
					break
				}
				if m.connection == nil {
					m.setError(fmt.Errorf("no active connection"))
					break
				}
				cmd := runQueryCmd(m.connection, query, maxQueryRows)
				if cmd != nil {
					m.isRunningQuery = true
					m.queryResultColumns = nil
					m.queryResultRows = nil
					m.setStatus("Running query…")
					cmds = append(cmds, cmd)
				}
			}
		}
	case queryFocusChat:
		// Chat panel currently informational; swallow input.
	}
	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m model) View() string {
	switch m.state {
	case connectionState:
		return m.connectionView()
	case tableState:
		return m.tableView()
	case queryState:
		return m.queryView()
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
	labels := []string{
		"Name",
		"Type",
		"Host/File",
		"Port",
		"Database",
		"Username",
		"Password",
		"SSL",
	}

	// Style inputs
	inputViews := make([]string, len(m.connInputs))
	for i, input := range m.connInputs {
		label := fmt.Sprintf("Field %d", i+1)
		if i < len(labels) {
			label = labels[i]
		}
		line := lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Render(fmt.Sprintf("%-10s", label+":"))
		view := input.View()
		if m.focusState == focusConnectionDetails && i == m.connFocused {
			view = focusedStyle.Render(view)
		}
		inputViews[i] = lipgloss.JoinHorizontal(lipgloss.Left, line, " ", view)
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeClr).
		MarginBottom(1).
		Render("Connection Details")

	inputs := lipgloss.JoinVertical(lipgloss.Left, inputViews...)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("↑/↓: move fields • Tab: switch panels • Ctrl+←/→: resize • Ctrl+N: new • Ctrl+S: save • Enter: connect")

	status := m.statusView()
	content := []string{title, inputs, "", help}
	if status != "" {
		content = append(content, "", status)
	}

	panel := lipgloss.JoinVertical(lipgloss.Left, content...)

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

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("Ctrl+Q: query editor • Esc: back to connections • Ctrl+C: exit")
	status := m.statusView()
	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	footerLines := []string{}
	if status != "" {
		footerLines = append(footerLines, status)
	}
	footerLines = append(footerLines, help)
	footer := lipgloss.JoinVertical(lipgloss.Left, footerLines...)
	return lipgloss.JoinVertical(lipgloss.Left, body, "", footer)
}

// queryView renders the query editor view with three panels
func (m model) queryView() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	leftWidth, midWidth, chatWidth, panelHeight := m.computeQueryLayout()

	leftStyle := lipgloss.NewStyle().
		Width(leftWidth).
		Height(panelHeight).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63"))
	if m.queryFocus == queryFocusTables {
		leftStyle = leftStyle.BorderForeground(activeClr)
	}
	leftPanel := leftStyle.Render(m.tables.View())

	editorTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeClr).
		Render("Query Editor")

	editorBoxHeight := m.queryEditorHeight + 2
	if editorBoxHeight > panelHeight-5 {
		editorBoxHeight = max(minEditorHeight+2, panelHeight/2)
	}
	editorBoxStyle := lipgloss.NewStyle().
		Width(midWidth).
		Height(editorBoxHeight).
		Border(lipgloss.NormalBorder()).
		Padding(0, 1).
		BorderForeground(lipgloss.Color("63"))
	if m.queryFocus == queryFocusEditor {
		editorBoxStyle = editorBoxStyle.BorderForeground(activeClr)
	}
	// Render editor using a copy to avoid mutating width in place
	editorView := m.queryInput.View()
	editorBox := editorBoxStyle.Render(editorView)

	resultsTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeClr).
		Render("Results")

	resultsHeight := panelHeight - editorBoxHeight - 4
	if resultsHeight < 5 {
		resultsHeight = 5
	}
	resultsBoxStyle := lipgloss.NewStyle().
		Width(midWidth).
		Height(resultsHeight).
		Border(lipgloss.NormalBorder()).
		Padding(1, 1).
		BorderForeground(lipgloss.Color("63"))
	resultsBox := resultsBoxStyle.Render(m.renderQueryResults(max(10, midWidth-4)))

	middlePanel := lipgloss.JoinVertical(lipgloss.Left,
		editorTitle,
		editorBox,
		"",
		resultsTitle,
		resultsBox,
	)

	chatTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeClr).
		Render("Chat")

	chatInfo := lipgloss.NewStyle().
		Foreground(lipgloss.Color("244")).
		Render("Connect an AI provider via /config ai in the main app to ask schema questions or request query drafts. Copy+paste the generated SQL into the editor.")

	chatStyle := lipgloss.NewStyle().
		Width(chatWidth).
		Height(panelHeight).
		Border(lipgloss.NormalBorder()).
		Padding(1, 1).
		BorderForeground(lipgloss.Color("63"))
	if m.queryFocus == queryFocusChat {
		chatStyle = chatStyle.BorderForeground(activeClr)
	}
	chatPanel := chatStyle.Render(lipgloss.JoinVertical(lipgloss.Left, chatTitle, "", chatInfo))

	body := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, middlePanel, chatPanel)
	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render("Tab: cycle panels • Ctrl+R/F5: run query • Ctrl+T: tables • Esc: back • Ctrl+C: exit")
	status := m.statusView()
	footer := []string{}
	if status != "" {
		footer = append(footer, status)
	}
	footer = append(footer, help)
	return lipgloss.JoinVertical(lipgloss.Left, body, "", lipgloss.JoinVertical(lipgloss.Left, footer...))
}

func (m model) renderQueryResults(width int) string {
	if width <= 0 {
		width = 10
	}
	if m.isRunningQuery {
		return "Running query…"
	}
	if len(m.queryResultColumns) == 0 {
		return "No results yet. Type a query above and press Ctrl+Enter."
	}

	colCount := len(m.queryResultColumns)
	widths := make([]int, colCount)
	for i, col := range m.queryResultColumns {
		w := lipgloss.Width(col)
		if w < 6 {
			w = 6
		}
		if w > 40 {
			w = 40
		}
		widths[i] = w
	}
	for _, row := range m.queryResultRows {
		for i := 0; i < min(len(row), colCount); i++ {
			cellWidth := lipgloss.Width(row[i])
			if cellWidth > widths[i] {
				if cellWidth > 40 {
					cellWidth = 40
				}
				widths[i] = cellWidth
			}
		}
	}

	for tableDisplayWidth(widths) > width {
		idx := indexOfMax(widths)
		if idx == -1 {
			break
		}
		if widths[idx] <= 6 {
			break
		}
		widths[idx]--
	}

	var builder strings.Builder
	for i, col := range m.queryResultColumns {
		if i > 0 {
			builder.WriteString(" │ ")
		}
		builder.WriteString(fmt.Sprintf("%-*s", widths[i], truncateCell(col, widths[i])))
	}
	builder.WriteString("\n")
	for i := range widths {
		if i > 0 {
			builder.WriteString("─┼─")
		}
		builder.WriteString(strings.Repeat("─", widths[i]))
	}
	builder.WriteString("\n")

	if len(m.queryResultRows) == 0 {
		builder.WriteString("(no rows)\n")
		return builder.String()
	}

	for _, row := range m.queryResultRows {
		for i := 0; i < colCount; i++ {
			if i > 0 {
				builder.WriteString(" │ ")
			}
			var val string
			if i < len(row) {
				val = row[i]
			}
			builder.WriteString(fmt.Sprintf("%-*s", widths[i], truncateCell(val, widths[i])))
		}
		builder.WriteString("\n")
	}

	if len(m.queryResultRows) >= maxQueryRows {
		builder.WriteString(fmt.Sprintf("(showing first %d rows)\n", maxQueryRows))
	}

	return builder.String()
}

// tableDetailsView renders the table details panel
func (m model) tableDetailsView() string {
	item := m.tables.SelectedItem()
	if item == nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7FD4FF")).
			Render("No tables loaded yet. Connect to a database and select a table.")
	}

	tableName := item.(tableItem).Name
	if m.loadingTable == tableName {
		return lipgloss.NewStyle().
			Foreground(activeClr).
			Render(fmt.Sprintf("Loading %s…", tableName))
	}

	info, ok := m.tableInfos[tableName]
	if !ok || info == nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Render("Table metadata not loaded yet.")
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(activeClr).
		Render(tableName)

	var sections []string
	if len(info.Columns) > 0 {
		var columnLines []string
		for _, col := range info.Columns {
			var attrs []string
			attrs = append(attrs, col.Type)
			if col.Nullable {
				attrs = append(attrs, "nullable")
			}
			if col.Key != "" {
				attrs = append(attrs, col.Key)
			}
			if col.Default != nil && *col.Default != "" {
				attrs = append(attrs, "default "+*col.Default)
			}
			if strings.TrimSpace(col.Extra) != "" {
				attrs = append(attrs, strings.TrimSpace(col.Extra))
			}
			columnLines = append(columnLines, fmt.Sprintf("• %s (%s)", col.Name, strings.Join(attrs, ", ")))
		}
		columnsBlock := lipgloss.JoinVertical(lipgloss.Left, append([]string{lipgloss.NewStyle().Bold(true).Render("Columns")}, lipgloss.NewStyle().PaddingLeft(2).Render(strings.Join(columnLines, "\n")))...)
		sections = append(sections, columnsBlock)
	}

	if len(info.PrimaryKeys) > 0 {
		sections = append(sections, lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Primary Keys"),
			lipgloss.NewStyle().PaddingLeft(2).Render("• "+strings.Join(info.PrimaryKeys, ", ")),
		))
	}

	if len(info.Constraints) > 0 {
		var lines []string
		for _, constraint := range info.Constraints {
			parts := []string{constraint.Type}
			if constraint.Column != "" {
				parts = append(parts, "column "+constraint.Column)
			}
			if strings.TrimSpace(constraint.Check) != "" {
				parts = append(parts, "check "+strings.TrimSpace(constraint.Check))
			}
			lines = append(lines, fmt.Sprintf("• %s (%s)", constraint.Name, strings.Join(parts, ", ")))
		}
		sections = append(sections, lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Constraints"),
			lipgloss.NewStyle().PaddingLeft(2).Render(strings.Join(lines, "\n")),
		))
	}

	if len(info.ForeignKeys) > 0 {
		var lines []string
		for _, fk := range info.ForeignKeys {
			parts := []string{fmt.Sprintf("%s → %s(%s)", fk.Column, fk.ReferencedTable, fk.ReferencedColumn)}
			if strings.TrimSpace(fk.OnDelete) != "" {
				parts = append(parts, "ON DELETE "+strings.TrimSpace(fk.OnDelete))
			}
			if strings.TrimSpace(fk.OnUpdate) != "" {
				parts = append(parts, "ON UPDATE "+strings.TrimSpace(fk.OnUpdate))
			}
			lines = append(lines, fmt.Sprintf("• %s (%s)", fk.Name, strings.Join(parts, ", ")))
		}
		sections = append(sections, lipgloss.JoinVertical(lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Render("Foreign Keys"),
			lipgloss.NewStyle().PaddingLeft(2).Render(strings.Join(lines, "\n")),
		))
	}

	if len(sections) == 0 {
		sections = append(sections, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No metadata available for this table."))
	}

	content := append([]string{title, ""}, sections...)
	panel := lipgloss.JoinVertical(lipgloss.Left, content...)

	rightWidth := m.width - int(float64(m.width)*m.panelSplit) - 2
	return lipgloss.NewStyle().
		Width(rightWidth-6).
		Height(m.height-6).
		Padding(1, 2).
		Render(panel)
}

// connItem represents a connection list item
type connItem struct {
	conn *config.ConnectionConfig
}

func (i connItem) Title() string {
	if i.conn == nil {
		return "(no connection)"
	}
	name := i.conn.Name
	if strings.TrimSpace(name) == "" {
		name = "(unsaved connection)"
	}
	maxLen := 30
	if len(name) > maxLen {
		return name[:maxLen-3] + "..."
	}
	return name
}

func (i connItem) Description() string {
	if i.conn == nil {
		return ""
	}
	var desc string
	if i.conn.DatabaseType == config.SQLite {
		desc = fmt.Sprintf("sqlite:%s", i.conn.Database)
	} else {
		port := i.conn.Port
		if port == 0 {
			port = config.GetDefaultPort(i.conn.DatabaseType)
		}
		desc = fmt.Sprintf("%s://%s:%d/%s", i.conn.DatabaseType.String(), i.conn.Host, port, i.conn.Database)
	}
	maxLen := 60
	if len(desc) > maxLen {
		return desc[:maxLen-3] + "..."
	}
	return desc
}

func (i connItem) FilterValue() string {
	if i.conn == nil {
		return ""
	}
	return i.conn.Name
}

// tableItem represents a table list item
type tableItem struct {
	Name string
}

func (i tableItem) Title() string       { return i.Name }
func (i tableItem) Description() string { return "Table with various columns" }
func (i tableItem) FilterValue() string { return i.Name }
