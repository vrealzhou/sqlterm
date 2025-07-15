package conversation

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"sqlterm/internal/ai"
	"sqlterm/internal/config"
	"sqlterm/internal/core"
	"sqlterm/internal/i18n"
	"sqlterm/internal/session"

	"github.com/chzyer/readline"
)

type App struct {
	rl         *readline.Instance
	connection core.Connection
	config     *core.ConnectionConfig
	configMgr  *config.Manager
	sessionMgr *session.Manager
	aiManager  *ai.Manager
	i18nMgr    *i18n.Manager
}

func NewApp() (*App, error) {
	configMgr := config.NewManager()

	// Initialize AI manager first
	aiManager, err := ai.NewManager(configMgr.GetConfigDir())
	if err != nil {
		// AI manager initialization failed completely
		aiManager = nil
	}

	// Initialize i18n manager
	language := "en_au" // Default language
	if aiManager != nil {
		if config := aiManager.GetConfig(); config != nil {
			language = config.Language
		}
	}

	i18nMgr, err := i18n.NewManager(language)
	if err != nil {
		// Fallback to default language if i18n fails
		i18nMgr, _ = i18n.NewManager("en_au")
	}

	// Initialize session manager with i18n manager
	sessionMgr := session.NewManager(configMgr.GetConfigDir(), i18nMgr)

	app := &App{
		configMgr:  configMgr,
		sessionMgr: sessionMgr,
		aiManager:  aiManager,
		i18nMgr:    i18nMgr,
	}

	// Ensure sessions directory exists for history file
	sessionsDir := filepath.Join(configMgr.GetConfigDir(), "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf(i18nMgr.Get("failed_to_create_sessions_dir"), err)
	}

	// Set up dynamic autocomplete
	completer := NewAutoCompleter(app)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "sqlterm > ",
		AutoComplete: completer,
		HistoryFile:  filepath.Join(configMgr.GetConfigDir(), "sessions", "global_history.txt"),
	})
	if err != nil {
		return nil, fmt.Errorf(i18nMgr.Get("failed_to_create_readline"), err)
	}

	app.rl = rl
	return app, nil
}

func (a *App) SetConnection(conn core.Connection, config *core.ConnectionConfig) {
	a.connection = conn
	a.config = config
	a.updatePrompt()

	// Ensure session directory and configuration exist
	if err := a.sessionMgr.EnsureSessionDir(config.Name); err != nil {
		fmt.Printf(a.i18nMgr.Get("session_init_warning"), err)
	}

	// Switch to session-specific history file
	if err := a.switchToSessionHistory(config.Name); err != nil {
		fmt.Printf(a.i18nMgr.Get("session_history_warning"), err)
	}

	// Initialize vector store for AI context if AI manager is available
	if a.aiManager != nil {
		fmt.Printf(a.i18nMgr.Get("initializing_vector_db"), config.Name)
		if err := a.aiManager.InitializeVectorStore(config.Name, conn); err != nil {
			fmt.Printf(a.i18nMgr.Get("vector_db_init_warning"), err)
		} else {
			fmt.Printf(a.i18nMgr.Get("vector_db_ready"), config.Name)
		}
	}
}

func (a *App) updatePrompt() {
	var prompt string
	if a.config != nil {
		prompt = fmt.Sprintf("sqlterm (%s) > ", a.config.Database)
	} else {
		prompt = "sqlterm > "
	}

	if a.rl != nil {
		a.rl.SetPrompt(prompt)
	}
}

// switchToSessionHistory changes the readline history file to be session-specific
func (a *App) switchToSessionHistory(connectionName string) error {
	// Create session-specific history file path
	sessionDir := filepath.Join(a.configMgr.GetConfigDir(), "sessions", connectionName)
	historyFile := filepath.Join(sessionDir, "history.txt")

	// Ensure the session directory exists
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_create_session_dir"), err)
	}

	// Migrate legacy global history if this is the first time using session histories
	if err := a.migrateLegacyHistory(historyFile); err != nil {
		fmt.Printf(a.i18nMgr.Get("failed_migrate_legacy_history_warning"), err)
	}

	// Update the readline config with the new history file
	// Note: The chzyer/readline library doesn't support changing history file after creation,
	// so we need to manage this manually by closing and recreating the instance
	if a.rl == nil {
		// If there's no readline instance, we can't switch history
		return nil
	}
	oldConfig := a.rl.Config
	a.rl.Close()

	// Create new readline instance with session-specific history
	newConfig := &readline.Config{
		Prompt:       oldConfig.Prompt,
		AutoComplete: oldConfig.AutoComplete,
		HistoryFile:  historyFile,
	}

	rl, err := readline.NewEx(newConfig)
	if err != nil {
		// Fallback: recreate with old config if new one fails
		a.rl, _ = readline.NewEx(oldConfig)
		return fmt.Errorf(a.i18nMgr.Get("failed_to_create_readline_session_history"), err)
	}

	a.rl = rl
	return nil
}

// migrateLegacyHistory copies the old global history.txt to session-specific history if it exists
func (a *App) migrateLegacyHistory(sessionHistoryFile string) error {
	legacyHistoryFile := filepath.Join(a.configMgr.GetConfigDir(), "sessions", "history.txt")

	// Check if legacy history file exists
	if _, err := os.Stat(legacyHistoryFile); os.IsNotExist(err) {
		return nil // No legacy history to migrate
	}

	// Check if session history file already exists
	if _, err := os.Stat(sessionHistoryFile); err == nil {
		return nil // Session history already exists, don't overwrite
	}

	// Copy legacy history to session history
	input, err := os.ReadFile(legacyHistoryFile)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_read_legacy_history"), err)
	}

	if err := os.WriteFile(sessionHistoryFile, input, 0644); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_write_session_history"), err)
	}

	fmt.Printf("📦 Migrated command history to session folder\n")
	return nil
}

// ClearConnection clears the current database connection and switches back to global history
func (a *App) ClearConnection() error {
	a.connection = nil
	a.config = nil
	a.updatePrompt()

	// Close vector store if active
	if a.aiManager != nil {
		a.aiManager.CloseVectorStore()
	}

	// Switch back to global history
	return a.switchToGlobalHistory()
}

// switchToGlobalHistory switches back to the global history file
func (a *App) switchToGlobalHistory() error {
	globalHistoryFile := filepath.Join(a.configMgr.GetConfigDir(), "sessions", "global_history.txt")

	// Update the readline config with the global history file
	if a.rl == nil {
		// If there's no readline instance, we can't switch history
		return nil
	}
	oldConfig := a.rl.Config
	a.rl.Close()

	// Create new readline instance with global history
	newConfig := &readline.Config{
		Prompt:       oldConfig.Prompt,
		AutoComplete: oldConfig.AutoComplete,
		HistoryFile:  globalHistoryFile,
	}

	rl, err := readline.NewEx(newConfig)
	if err != nil {
		// Fallback: recreate with old config if new one fails
		a.rl, _ = readline.NewEx(oldConfig)
		return fmt.Errorf(a.i18nMgr.Get("failed_to_create_readline_global_history"), err)
	}

	a.rl = rl
	return nil
}

func (a *App) Run() error {
	defer a.rl.Close()
	defer func() {
		if a.aiManager != nil {
			a.aiManager.CloseVectorStore()
		}
	}()

	fmt.Println(a.i18nMgr.Get("sqlterm_conversation_mode"))
	fmt.Println(a.i18nMgr.Get("prompt_welcome"))
	fmt.Println()

	for {
		line, err := a.rl.Readline()
		if err == readline.ErrInterrupt {
			continue
		} else if err == io.EOF {
			break
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if err := a.processLine(line); err != nil {
			fmt.Printf(a.i18nMgr.Get("generic_error"), err)
		}
	}

	return nil
}

func (a *App) processLine(line string) error {
	if strings.HasPrefix(line, "/") {
		return a.processCommand(line)
	} else if strings.HasPrefix(line, "@") {
		return a.processQueryFile(line)
	} else {
		// Handle as AI chat
		return a.processAIChat(line)
	}
}

func (a *App) processCommand(line string) error {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	command := parts[0]
	args := parts[1:]

	switch command {
	case "/help":
		return a.handleHelp(args)
	case "/quit", "/exit":
		os.Exit(0)
	case "/connect":
		return a.handleConnect(args)
	case "/list-connections":
		return a.handleListConnections()
	case "/tables":
		return a.handleListTables()
	case "/describe":
		return a.handleDescribeTable(args)
	case "/status":
		a.handleStatus()
	case "/exec":
		return a.handleExecQuery(args)
	case "/config":
		return a.handleConfig(args)
	case "/last-ai-call":
		return a.handleShowPrompts(args)
	case "/clear-conversation":
		return a.handleClearConversation()
	default:
		fmt.Printf(a.i18nMgr.Get("unknown_command"), command)
	}

	return nil
}

func (a *App) processQueryFile(line string) error {
	// Check if it's a CSV export with @
	if strings.Contains(line, " > ") {
		return a.processFileCommandWithCSVExport(line)
	}

	parts := strings.Fields(line)
	if len(parts) == 0 {
		return nil
	}

	filename := parts[0][1:] // Remove @ prefix
	var queryRange []int

	if len(parts) > 1 {
		rangeStr := parts[1]
		if strings.Contains(rangeStr, "-") {
			rangeParts := strings.Split(rangeStr, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(rangeParts[0])
				end, err2 := strconv.Atoi(rangeParts[1])
				if err1 == nil && err2 == nil {
					queryRange = []int{start, end}
				}
			}
		} else {
			if num, err := strconv.Atoi(rangeStr); err == nil {
				queryRange = []int{num, num}
			}
		}
	}

	return a.executeFile(filename, queryRange)
}

func (a *App) processQuery(query string, resultWriter io.Writer) error {
	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("no_database_connection"))
		return nil
	}

	result, err := a.connection.Execute(query)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("query_execution_failed"), err)
	}

	// Save as markdown and display with glamour
	if a.config != nil {
		if err := a.sessionMgr.EnsureSessionDir(a.config.Name); err != nil {
			fmt.Printf(a.i18nMgr.Get("failed_create_session_dir_warning"), err)
		} else {
			err := core.SaveQueryResultAsMarkdown(result, query, a.config.Name, resultWriter, a.i18nMgr)
			if err != nil {
				fmt.Printf(a.i18nMgr.Get("failed_save_markdown_warning"), err)
			}
		}
	}

	return nil
}

func (a *App) prepareQueryResultMarkdown() (string, *os.File, error) {
	if err := a.sessionMgr.EnsureSessionDir(a.config.Name); err != nil {
		return "", nil, fmt.Errorf(a.i18nMgr.Get("failed_to_create_session_dir"), err)
	}
	// Generate filename with timestamp
	configDir := a.configMgr.GetConfigDir()
	// Create sessions directory structure
	resultsDir := filepath.Join(configDir, "sessions", a.config.Name, "results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return "", nil, fmt.Errorf("%s: %w", a.i18nMgr.Get("failed_to_create_results_dir"), err)
	}
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("query_results_%s.md", timestamp)
	filename = filepath.Join(resultsDir, filename)
	writer, err := os.Create(filename)
	if err != nil {
		return filename, nil, err
	}
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# %s - %s\n\n", a.i18nMgr.Get("query_results_header"), time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("**%s:** %s\n\n", a.i18nMgr.Get("connection_header"), a.config.Name))
	writer.Write([]byte(content.String()))
	return filename, writer, err
}

func (a *App) preparePromptHistoryMarkdown() (string, *os.File, error) {
	if a.config == nil {
		return "", nil, errors.New(a.i18nMgr.Get("no_connection_for_session_dir"))
	}

	if err := a.sessionMgr.EnsureSessionDir(a.config.Name); err != nil {
		return "", nil, fmt.Errorf(a.i18nMgr.Get("failed_to_create_session_dir"), err)
	}

	// Generate filename with timestamp
	configDir := a.configMgr.GetConfigDir()
	// Create sessions directory structure
	resultsDir := filepath.Join(configDir, "sessions", a.config.Name, "results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return "", nil, fmt.Errorf("%s: %w", a.i18nMgr.Get("failed_to_create_results_dir"), err)
	}

	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("conversation_history_%s.md", timestamp)
	filename = filepath.Join(resultsDir, filename)

	writer, err := os.Create(filename)
	if err != nil {
		return filename, nil, err
	}

	var content strings.Builder
	content.WriteString(fmt.Sprintf("# %s - %s\n\n", a.i18nMgr.Get("ai_conversation_history_header"), time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("**%s:** %s\n\n", a.i18nMgr.Get("connection_header"), a.config.Name))
	writer.Write([]byte(content.String()))

	return filename, writer, err
}

func (a *App) executeFile(filename string, queryRange []int) error {
	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("no_database_connection"))
		return nil
	}

	// Try to find the file in current directory or queries directory
	var filepath string
	if _, err := os.Stat(filename); err == nil {
		filepath = filename
	} else if _, err := os.Stat("queries/" + filename); err == nil {
		filepath = "queries/" + filename
	} else {
		return fmt.Errorf(a.i18nMgr.Get("file_not_found"), filename)
	}

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_open_file"), err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_read_file"), err)
	}

	queries := a.parseQueries(string(content))
	fmt.Printf(a.i18nMgr.Get("executing_sql_file"), filename)
	fmt.Printf(a.i18nMgr.Get("found_queries_in_file"), len(queries))

	start, end := 1, len(queries)
	if len(queryRange) == 2 {
		start, end = queryRange[0], queryRange[1]
	}

	mdPath, writer, err := a.prepareQueryResultMarkdown()
	if err != nil {
		fmt.Println("Warning:", err.Error())
		return nil
	}

	for i := start - 1; i < end && i < len(queries); i++ {
		query := strings.TrimSpace(queries[i])
		if query == "" {
			continue
		}

		err = a.processQuery(query, writer)
		if err != nil {
			fmt.Printf(a.i18nMgr.Get("query_failed"), err)
		}
	}
	writer.Close()

	if err := a.sessionMgr.ViewMarkdown(mdPath); err != nil {
		fmt.Printf(a.i18nMgr.Get("generic_warning"), err)
	}
	fmt.Printf("📍 %s: %s\n", a.i18nMgr.Get("file_location"), mdPath)

	return nil
}

func (a *App) parseQueries(content string) []string {
	var queries []string
	var currentQuery strings.Builder

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "--") {
			continue
		}

		currentQuery.WriteString(line)
		currentQuery.WriteString(" ")

		if strings.HasSuffix(line, ";") {
			queries = append(queries, strings.TrimSuffix(currentQuery.String(), ";"))
			currentQuery.Reset()
		}
	}

	if currentQuery.Len() > 0 {
		queries = append(queries, currentQuery.String())
	}

	return queries
}

func (a *App) truncateQuery(query string) string {
	if len(query) > 50 {
		return query[:47] + "..."
	}
	return query
}

func (a *App) handleHelp(args []string) error {
	if len(args) == 0 {
		// Show general help
		a.printHelp()
		return nil
	}

	// Handle specific command help
	command := args[0]
	subArgs := args[1:]

	switch command {
	case "config":
		return a.printConfigHelp(subArgs)
	case "connect":
		return a.printConnectHelp()
	case "exec":
		return a.printExecHelp()
	case "tables":
		return a.printTablesHelp()
	case "describe":
		return a.printDescribeHelp()
	case "status":
		return a.printStatusHelp()
	case "prompts":
		return a.printPromptsHelp()
	default:
		fmt.Printf(a.i18nMgr.Get("unknown_help_command"), command)
		return nil
	}
}

func (a *App) printHelp() {
	fmt.Println(a.i18nMgr.Get("help_full"))
}

func (a *App) handleConnect(args []string) error {
	if len(args) == 0 {
		return a.interactiveConnect()
	}

	name := args[0]
	config, err := a.configMgr.LoadConnection(name)
	if err != nil {
		return errors.New(a.i18nMgr.GetWithArgs("failed_to_load_connection", name, err))
	}

	fmt.Printf(a.i18nMgr.Get("connecting_to"), config.Name)
	conn, err := core.NewConnection(config)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_connect"), err)
	}

	if err := conn.Ping(); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("connection_test_failed"), err)
	}

	if a.connection != nil {
		a.connection.Close()
	}

	a.SetConnection(conn, config)
	fmt.Printf(a.i18nMgr.Get("connected_to"), config.Name, config.Database)

	return nil
}

func (a *App) interactiveConnect() error {
	fmt.Println(a.i18nMgr.Get("interactive_connection_setup"))

	reader := bufio.NewReader(os.Stdin)

	fmt.Print(a.i18nMgr.Get("enter_connection_name"))
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	fmt.Println(a.i18nMgr.Get("select_database_type"))
	fmt.Println(a.i18nMgr.Get("mysql_option"))
	fmt.Println(a.i18nMgr.Get("postgresql_option"))
	fmt.Println(a.i18nMgr.Get("sqlite_option"))
	fmt.Print(a.i18nMgr.Get("enter_choice"))

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	var dbType core.DatabaseType
	switch choice {
	case "1":
		dbType = core.MySQL
	case "2":
		dbType = core.PostgreSQL
	case "3":
		dbType = core.SQLite
	default:
		return fmt.Errorf(a.i18nMgr.Get("invalid_choice"), choice)
	}

	config := &core.ConnectionConfig{
		Name:         name,
		DatabaseType: dbType,
	}

	if dbType != core.SQLite {
		fmt.Print(a.i18nMgr.Get("enter_host"))
		host, _ := reader.ReadString('\n')
		host = strings.TrimSpace(host)
		if host == "" {
			host = "localhost"
		}
		config.Host = host

		fmt.Printf(a.i18nMgr.Get("enter_port"), core.GetDefaultPort(dbType))
		portStr, _ := reader.ReadString('\n')
		portStr = strings.TrimSpace(portStr)
		if portStr == "" {
			config.Port = core.GetDefaultPort(dbType)
		} else {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return fmt.Errorf(a.i18nMgr.Get("invalid_port"), portStr)
			}
			config.Port = port
		}

		fmt.Print(a.i18nMgr.Get("enter_username"))
		username, _ := reader.ReadString('\n')
		config.Username = strings.TrimSpace(username)

		fmt.Print(a.i18nMgr.Get("enter_password"))
		password, _ := reader.ReadString('\n')
		config.Password = strings.TrimSpace(password)
	}

	fmt.Print(a.i18nMgr.Get("enter_database_name"))
	database, _ := reader.ReadString('\n')
	config.Database = strings.TrimSpace(database)

	// Test connection
	fmt.Printf(a.i18nMgr.Get("testing_connection"), config.Name)
	conn, err := core.NewConnection(config)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_connect"), err)
	}

	if err := conn.Ping(); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("connection_test_failed"), err)
	}

	if a.connection != nil {
		a.connection.Close()
	}

	a.SetConnection(conn, config)
	fmt.Printf(a.i18nMgr.Get("connected_to"), config.Name, config.Database)

	// Save connection
	if err := a.configMgr.SaveConnection(config); err != nil {
		fmt.Printf(a.i18nMgr.Get("failed_save_connection_warning"), err)
	} else {
		fmt.Print(a.i18nMgr.Get("connection_saved"))
	}

	return nil
}

func (a *App) handleListConnections() error {
	connections, err := a.configMgr.ListConnections()
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_list_connections"), err)
	}

	if len(connections) == 0 {
		fmt.Println(a.i18nMgr.Get("no_saved_connections_found"))
		return nil
	}

	fmt.Println(a.i18nMgr.Get("saved_connections"))
	for i, conn := range connections {
		fmt.Printf("  %d. %s (%s) - %s://%s:%d/%s\n",
			i+1,
			conn.Name,
			conn.DatabaseType,
			conn.DatabaseType.String(),
			conn.Host,
			conn.Port,
			conn.Database)
	}

	return nil
}

func (a *App) handleListTables() error {
	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("no_database_connection"))
		return nil
	}

	tables, err := a.connection.ListTables()
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_list_tables"), err)
	}

	if len(tables) == 0 {
		fmt.Printf(a.i18nMgr.Get("no_tables_found"), a.config.Database)
		return nil
	}

	fmt.Printf(a.i18nMgr.Get("tables_in_database"), a.config.Database)
	for i, table := range tables {
		fmt.Printf("  %d. %s\n", i+1, table)
	}

	return nil
}

func (a *App) handleDescribeTable(args []string) error {
	if len(args) == 0 {
		fmt.Println(a.i18nMgr.Get("usage_describe_table"))
		return nil
	}

	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("no_database_connection"))
		return nil
	}

	tableName := args[0]
	tableInfo, err := a.connection.DescribeTable(tableName)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_describe_table"), err)
	}

	// Generate markdown content
	markdown := a.generateTableMarkdown(tableInfo)

	// Display with glamour
	return a.displayMarkdown(markdown)
}

func (a *App) generateTableMarkdown(tableInfo *core.TableInfo) string {
	var sb strings.Builder

	// Title
	sb.WriteString(fmt.Sprintf("# 📊 %s: %s\n\n", a.i18nMgr.Get("table_header"), tableInfo.Name))

	// Columns section
	sb.WriteString(fmt.Sprintf("## 📋 %s\n\n", a.i18nMgr.Get("columns_header")))
	sb.WriteString(a.i18nMgr.Get("column_table_header"))
	sb.WriteString(a.i18nMgr.Get("column_table_separator"))

	for _, col := range tableInfo.Columns {
		nullable := a.i18nMgr.Get("not_nullable")
		if col.Nullable {
			nullable = a.i18nMgr.Get("nullable")
		}

		key := ""
		if col.Key != "" {
			key = fmt.Sprintf(a.i18nMgr.Get("key_format"), col.Key)
		}

		defaultVal := ""
		if col.Default != nil {
			defaultVal = fmt.Sprintf("`%s`", *col.Default)
		}

		sb.WriteString(fmt.Sprintf("| **%s** | `%s` | %s | %s | %s |\n",
			col.Name, col.Type, nullable, key, defaultVal))
	}

	// Primary keys section
	if len(tableInfo.PrimaryKeys) > 0 {
		sb.WriteString(fmt.Sprintf("\n## 🔑 %s\n\n", a.i18nMgr.Get("primary_keys_header")))
		for _, pk := range tableInfo.PrimaryKeys {
			sb.WriteString(fmt.Sprintf("- **%s**\n", pk))
		}
	}

	// Constraints section
	if len(tableInfo.Constraints) > 0 {
		sb.WriteString(fmt.Sprintf("\n## ⚠️ %s\n\n", a.i18nMgr.Get("constraints_header")))
		for _, constraint := range tableInfo.Constraints {
			sb.WriteString(fmt.Sprintf("### %s (%s)\n", constraint.Name, constraint.Type))
			sb.WriteString(fmt.Sprintf("- **%s:** %s\n", a.i18nMgr.Get("column_header"), constraint.Column))
			if constraint.Check != "" {
				sb.WriteString(fmt.Sprintf("- **%s:** `%s`\n", a.i18nMgr.Get("check_header"), constraint.Check))
			}
			sb.WriteString("\n")
		}
	}

	// Foreign keys section
	if len(tableInfo.ForeignKeys) > 0 {
		sb.WriteString(fmt.Sprintf("\n## 🔗 %s\n\n", a.i18nMgr.Get("foreign_keys_header")))
		for _, fk := range tableInfo.ForeignKeys {
			sb.WriteString(fmt.Sprintf("### %s\n", fk.Name))
			sb.WriteString(fmt.Sprintf("- **%s:** %s\n", a.i18nMgr.Get("column_header"), fk.Column))
			sb.WriteString(fmt.Sprintf("- **References:** %s.%s\n", fk.ReferencedTable, fk.ReferencedColumn))
			if fk.OnDelete != "" {
				sb.WriteString(fmt.Sprintf("- **On Delete:** %s\n", fk.OnDelete))
			}
			if fk.OnUpdate != "" {
				sb.WriteString(fmt.Sprintf("- **On Update:** %s\n", fk.OnUpdate))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func (a *App) displayMarkdown(markdown string) error {
	// Use the shared markdown renderer
	renderer := core.NewMarkdownRenderer(a.i18nMgr)
	return renderer.RenderAndDisplay(markdown)
}

func (a *App) handleStatus() {
	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("status_not_connected"))
		fmt.Println(a.i18nMgr.Get("use_connect_to_establish_connection"))
		return
	}

	fmt.Printf(a.i18nMgr.Get("status_connected"), a.config.Name)
	fmt.Printf(a.i18nMgr.Get("database_info"), a.config.Database)
	fmt.Printf(a.i18nMgr.Get("type_info"), a.config.DatabaseType)
	if a.config.DatabaseType != core.SQLite {
		fmt.Printf(a.i18nMgr.Get("host_info"), a.config.Host, a.config.Port)
		fmt.Printf(a.i18nMgr.Get("username_info"), a.config.Username)
	}
}

func (a *App) handleExecQuery(args []string) error {
	if len(args) == 0 {
		return a.handleMultilineExec()
	}

	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("no_database_connection"))
		return nil
	}

	line := strings.Join(args, " ")

	// Check if it's a CSV export
	if strings.Contains(line, " > ") {
		return a.processQueryWithCSVExport(line)
	}
	mdPath, writer, err := a.prepareQueryResultMarkdown()
	if err != nil {
		fmt.Println("Warning:", err.Error())
		return nil
	}
	err = a.processQuery(line, writer)
	writer.Close()
	if err != nil {
		fmt.Println("Warning:", err.Error())
		return nil
	}
	if err := a.sessionMgr.ViewMarkdown(mdPath); err != nil {
		fmt.Printf(a.i18nMgr.Get("generic_warning"), err)
	}
	fmt.Printf("📍 %s: %s\n", a.i18nMgr.Get("file_location"), mdPath)
	return nil
}

func (a *App) handleMultilineExec() error {
	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("no_database_connection"))
		return nil
	}

	fmt.Println(a.i18nMgr.Get("multi_line_sql_mode"))
	fmt.Println(a.i18nMgr.Get("multi_line_sql_paste_lines"))
	fmt.Println(a.i18nMgr.Get("multi_line_sql_end_with_semicolon"))
	fmt.Println(a.i18nMgr.Get("multi_line_sql_cancel"))
	fmt.Println(a.i18nMgr.Get("multi_line_sql_csv_export"))
	fmt.Println()

	var queryLines []string
	lineNumber := 1

	// Temporarily disable history for multi-line input
	a.rl.HistoryDisable()
	defer a.rl.HistoryEnable()

	for {
		// Create a custom prompt for multi-line input
		prompt := fmt.Sprintf("  %2d│ ", lineNumber)
		a.rl.SetPrompt(prompt)

		line, err := a.rl.Readline()
		if err != nil {
			// User pressed Ctrl+C or EOF
			fmt.Println(a.i18nMgr.Get("multi_line_input_cancelled"))
			a.updatePrompt() // Restore original prompt
			return nil
		}

		line = strings.TrimSpace(line)

		if line != "" {
			queryLines = append(queryLines, line)

			// Check if this line ends with semicolon - if so, we're done
			// Also handle cases like "; -- comment" or "; > file.csv"
			if strings.Contains(line, ";") {
				// Find the position of the last semicolon
				lastSemi := strings.LastIndex(line, ";")
				afterSemi := strings.TrimSpace(line[lastSemi+1:])

				// If there's nothing after the semicolon, or only CSV export syntax, we're done
				if afterSemi == "" || strings.HasPrefix(afterSemi, ">") || strings.HasPrefix(afterSemi, "--") {
					break
				}
			}
		}
		lineNumber++
	}

	// Restore original prompt
	a.updatePrompt()

	if len(queryLines) == 0 {
		fmt.Println(a.i18nMgr.Get("no_query_entered"))
		return nil
	}

	// Join all lines into a single query
	fullQuery := strings.Join(queryLines, " ")

	// Add the complete multi-line query as a single history entry
	historyEntry := "/exec " + fullQuery
	if err := a.rl.SaveHistory(historyEntry); err != nil {
		fmt.Printf(a.i18nMgr.Get("failed_save_command_history_warning"), err)
	}

	fmt.Print(a.i18nMgr.Get("executing_query"))
	fmt.Printf(a.i18nMgr.Get("query_truncated"), a.truncateQuery(fullQuery))

	// Check if it's a CSV export
	if strings.Contains(fullQuery, " > ") {
		return a.processQueryWithCSVExport(fullQuery)
	}

	// Regular execution
	mdPath, writer, err := a.prepareQueryResultMarkdown()
	if err != nil {
		fmt.Println("Warning:", err.Error())
		return nil
	}
	err = a.processQuery(fullQuery, writer)
	writer.Close()
	if err != nil {
		fmt.Println("Warning:", err.Error())
		return nil
	}
	if err := a.sessionMgr.ViewMarkdown(mdPath); err != nil {
		fmt.Printf(a.i18nMgr.Get("generic_warning"), err)
	}
	fmt.Printf("📍 %s: %s\n", a.i18nMgr.Get("file_location"), mdPath)
	return nil
}

func (a *App) processQueryWithCSVExport(line string) error {
	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("no_database_connection"))
		return nil
	}

	parts := strings.SplitN(line, " > ", 2)
	if len(parts) != 2 {
		return errors.New(a.i18nMgr.Get("invalid_csv_export_syntax"))
	}

	query := strings.TrimSpace(parts[0])
	filename := strings.TrimSpace(parts[1])

	fmt.Printf(a.i18nMgr.Get("executing_query_streaming"), filename)

	result, err := a.connection.Execute(query)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("query_execution_failed"), err)
	}

	rows, err := core.SaveQueryResultAsStreamingCSV(result, filename)
	if err != nil {
		return fmt.Errorf("failed to save CSV: %w", err)
	}

	fmt.Printf(a.i18nMgr.Get("exported_rows_to_file"), rows, filename)
	return nil
}

func (a *App) processFileCommandWithCSVExport(line string) error {
	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("no_database_connection"))
		return nil
	}

	parts := strings.SplitN(line, " > ", 2)
	if len(parts) != 2 {
		return errors.New(a.i18nMgr.Get("invalid_csv_export_syntax_file"))
	}

	fileCmd := strings.TrimSpace(parts[0])
	csvFilename := strings.TrimSpace(parts[1])

	// Parse the file command
	cmdParts := strings.Fields(fileCmd)
	if len(cmdParts) == 0 {
		return nil
	}

	filename := cmdParts[0][1:] // Remove @ prefix
	var queryRange []int

	if len(cmdParts) > 1 {
		rangeStr := cmdParts[1]
		if strings.Contains(rangeStr, "-") {
			rangeParts := strings.Split(rangeStr, "-")
			if len(rangeParts) == 2 {
				start, err1 := strconv.Atoi(rangeParts[0])
				end, err2 := strconv.Atoi(rangeParts[1])
				if err1 == nil && err2 == nil {
					queryRange = []int{start, end}
				}
			}
		} else {
			if num, err := strconv.Atoi(rangeStr); err == nil {
				queryRange = []int{num, num}
			}
		}
	}

	return a.executeFileWithCSVExport(filename, queryRange, csvFilename)
}

func (a *App) executeFileWithCSVExport(filename string, queryRange []int, csvFilename string) error {
	if a.connection == nil {
		fmt.Println(a.i18nMgr.Get("no_database_connection"))
		return nil
	}

	// Try to find the file in current directory or queries directory
	var filepath string
	if _, err := os.Stat(filename); err == nil {
		filepath = filename
	} else if _, err := os.Stat("queries/" + filename); err == nil {
		filepath = "queries/" + filename
	} else {
		return fmt.Errorf(a.i18nMgr.Get("file_not_found"), filename)
	}

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_open_file"), err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_read_file"), err)
	}

	queries := a.parseQueries(string(content))
	fmt.Printf(a.i18nMgr.Get("executing_sql_file"), filename)
	fmt.Printf(a.i18nMgr.Get("found_queries_in_file"), len(queries))

	start, end := 1, len(queries)
	if len(queryRange) == 2 {
		start, end = queryRange[0], queryRange[1]
	}

	// First pass: count queries that will return results
	var queriesWithResults []string
	for i := start - 1; i < end && i < len(queries); i++ {
		query := strings.TrimSpace(queries[i])
		if query != "" {
			queriesWithResults = append(queriesWithResults, query)
		}
	}

	// Track exported files and statistics
	var exportedFiles []string
	var totalRowsExported int
	queryNumber := 0

	for i := start - 1; i < end && i < len(queries); i++ {
		query := strings.TrimSpace(queries[i])
		if query == "" {
			continue
		}

		fmt.Printf(a.i18nMgr.Get("query_number_truncated_query"), i+1, a.truncateQuery(query))
		result, err := a.connection.Execute(query)
		if err != nil {
			fmt.Printf(a.i18nMgr.Get("query_failed"), err)
			continue
		}

		// Export each query to a separate CSV file
		queryNumber++
		var outputPath string
		if len(queriesWithResults) == 1 {
			// Single query in file - use original filename
			outputPath = csvFilename
		} else {
			// Multiple queries - use numbered filenames
			outputPath = core.GenerateNumberedCSVPath(csvFilename, queryNumber)
		}

		rows, err := core.SaveQueryResultAsStreamingCSV(result, outputPath)
		if err != nil {
			fmt.Printf("❌ Failed to save CSV: %v\n", err)
			continue
		}
		fmt.Printf(a.i18nMgr.Get("query_executed_rows"), rows)

		exportedFiles = append(exportedFiles, outputPath)
		totalRowsExported += rows
		fmt.Printf(a.i18nMgr.Get("exported_rows_to_file"), rows, outputPath)
	}

	// Summary of exported files
	if len(exportedFiles) > 0 {
		if len(exportedFiles) == 1 {
			fmt.Printf(a.i18nMgr.Get("exported_to_single_file"), exportedFiles[0])
		} else {
			fmt.Printf(a.i18nMgr.Get("exported_to_multiple_files"), len(exportedFiles))
			for _, file := range exportedFiles {
				fmt.Printf("   - %s\n", file)
			}
		}
		fmt.Printf(a.i18nMgr.Get("total_rows_exported"), totalRowsExported)
	} else {
		fmt.Println(a.i18nMgr.Get("no_results_to_export"))
	}

	return nil
}

func (a *App) processAIChat(message string) error {
	if a.aiManager == nil || !a.aiManager.IsConfigured() {
		fmt.Println(a.i18nMgr.Get("ai_not_configured"))
		return nil
	}

	// Get current conversation or show thinking message
	conversation := a.aiManager.GetCurrentConversation()
	if conversation == nil {
		fmt.Print(a.i18nMgr.Get("ai_starting_new_conversation"))
	} else {
		fmt.Printf(a.i18nMgr.Get("ai_processing_conversation"), conversation.CurrentPhase.String())
	}

	// Get database tables for context
	var tables []string
	if a.connection != nil {
		var err error
		tables, err = a.connection.ListTables()
		if err != nil {
			fmt.Printf("Warning: failed to get table list for AI context: %v\n", err)
		}
	}

	// Create context with timeout for AI requests
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Use new conversational chat system
	response, err := a.aiManager.ChatWithConversation(ctx, message, tables)
	if err != nil {
		// Provide more helpful error messages for common issues
		if strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "deadline exceeded") {
			fmt.Print(a.i18nMgr.Get("ai_timeout_message"))
			fmt.Print(a.i18nMgr.Get("ai_timeout_complex_queries"))
			fmt.Print(a.i18nMgr.Get("ai_timeout_network_issues"))
			fmt.Print(a.i18nMgr.Get("ai_timeout_provider_overloaded"))
			fmt.Print(a.i18nMgr.Get("ai_timeout_suggestion"))
			return nil
		}
		return fmt.Errorf(a.i18nMgr.Get("ai_chat_failed"), err)
	}

	// Format any SQL code blocks in the response
	formattedResponse := core.FormatSQLInMarkdown(response)

	// Display response using markdown renderer
	renderer := core.NewMarkdownRenderer(a.i18nMgr)
	if err := renderer.RenderAndDisplay(formattedResponse); err != nil {
		// Fallback to plain text if markdown rendering fails
		fmt.Println(a.i18nMgr.Get("ai_response_header"))
		fmt.Println(formattedResponse)
	}

	// Show conversation status and AI info
	conversation = a.aiManager.GetCurrentConversation()
	if conversation != nil {
		statusInfo := fmt.Sprintf("📊 Conversation: %s | Tables loaded: %d",
			conversation.CurrentPhase.String(), len(conversation.LoadedTables))
		if conversation.IsComplete {
			statusInfo += " | ✅ Complete"
		}
		fmt.Printf("\n%s\n", statusInfo)
	}

	// Show AI status after response
	if a.aiManager != nil && a.aiManager.IsConfigured() {
		aiConfig := a.aiManager.GetConfig()
		// Get usage statistics from AI manager if available
		usageInfo := a.i18nMgr.Get("usage_data_unavailable")
		if a.aiManager.GetUsageStore() != nil {
			if summary, err := a.aiManager.GetUsageStore().GetUsageSummary(); err == nil {
				if todayStats, ok := summary["today"]; ok {
					if today, ok := todayStats.(map[string]interface{}); ok {
						usageInfo = fmt.Sprintf(a.i18nMgr.Get("usage_today_summary"), 
							int(today["requests"].(int)), today["cost"].(float64))
					}
				}
			}
		}
		aiInfo := fmt.Sprintf("🤖 %s | %s", aiConfig.FormatProviderInfo(), usageInfo)
		fmt.Printf("%s\n", aiInfo)
	}

	return nil
}

func (a *App) handleConfig(args []string) error {
	if len(args) == 0 {
		return a.printConfigHelp([]string{})
	}

	section := args[0]
	switch section {
	case "status":
		return a.handleConfigStatus()
	case "ai":
		return a.handleConfigAI(args[1:])
	case "language":
		return a.handleConfigLanguage(args[1:])
	default:
		fmt.Printf(a.i18nMgr.Get("unknown_config_section"), section)
		a.printConfigHelp([]string{})
		return nil
	}
}

func (a *App) handleConfigStatus() error {
	fmt.Println("🔧 SQLTerm Configuration Status")
	fmt.Println()

	// Language configuration
	if a.aiManager != nil {
		config := a.aiManager.GetConfig()
		fmt.Printf("🌐 Language: %s\n", config.Language)
	} else {
		fmt.Printf("🌐 Language: en_au (default)\n")
	}
	fmt.Println()

	// AI configuration status
	fmt.Println("🤖 AI Configuration:")
	if a.aiManager == nil {
		fmt.Println("   Status: ❌ Not initialized")
		fmt.Println("   Use '/config ai' to set up AI integration")
	} else {
		config := a.aiManager.GetConfig()
		if a.aiManager.IsConfigured() {
			fmt.Printf("   Status: ✅ Configured\n")
			fmt.Printf("   Provider: %s\n", config.AI.Provider)
			fmt.Printf("   Model: %s\n", config.AI.Model)
			
			// Show masked API keys
			hasKeys := false
			for provider, key := range config.AI.APIKeys {
				if key != "" {
					if !hasKeys {
						fmt.Println("   API Keys:")
						hasKeys = true
					}
					maskedKey := "xxxxxx..." + key[max(0, len(key)-4):]
					fmt.Printf("     %s: %s\n", provider, maskedKey)
				}
			}
			
			// Show base URLs
			hasURLs := false
			for provider, url := range config.AI.BaseURLs {
				if url != "" {
					if !hasURLs {
						fmt.Println("   Base URLs:")
						hasURLs = true
					}
					fmt.Printf("     %s: %s\n", provider, url)
				}
			}
		} else {
			fmt.Printf("   Status: ⚠️  Initialized but not configured\n")
			fmt.Println("   Use '/config ai' to complete setup")
		}
	}
	fmt.Println()

	// Connection status
	fmt.Println("🔗 Database Connection:")
	if a.connection == nil {
		fmt.Println("   Status: ❌ Not connected")
		fmt.Println("   Use '/connect' to establish a connection")
	} else {
		fmt.Printf("   Status: ✅ Connected to %s\n", a.config.Name)
		fmt.Printf("   Database: %s (%s)\n", a.config.Database, a.config.DatabaseType)
		if a.config.DatabaseType != core.SQLite {
			fmt.Printf("   Host: %s:%d\n", a.config.Host, a.config.Port)
			fmt.Printf("   Username: %s\n", a.config.Username)
		}
	}

	return nil
}

func (a *App) handleConfigAI(args []string) error {
	if len(args) == 0 {
		return a.interactiveAIConfig()
	}

	subcmd := args[0]
	switch subcmd {
	case "status":
		return a.handleAIConfigStatus()
	case "provider":
		return a.handleAIConfigProvider(args[1:])
	case "model":
		return a.handleAIConfigModel(args[1:])
	case "api-key":
		return a.handleAIConfigAPIKey(args[1:])
	case "base-url":
		return a.handleAIConfigBaseURL(args[1:])
	case "list-models":
		return a.handleAIConfigListModels()
	case "openrouter":
		return a.handleConfigAIOpenRouter(args[1:])
	default:
		fmt.Printf(a.i18nMgr.Get("unknown_ai_subcommand"), subcmd)
		a.printAIConfigHelp()
		return nil
	}
}

func (a *App) printAIConfigHelp() {
	fmt.Println(`
🤖 AI Configuration Commands:

/config ai                     Interactive AI setup wizard (recommended)
/config ai status              Show current AI configuration and usage
/config ai provider <name>     Set AI provider (openrouter, ollama, lmstudio)
/config ai model <model>       Set AI model for current provider
/config ai api-key <provider> <key>  Set API key for provider
/config ai base-url <provider> <url> Set base URL for local providers
/config language <lang>        Set interface language (en_au, zh_cn)
/config ai list-models         List available models for current provider

Interactive Setup:
Run /config ai without arguments to start the setup wizard that will:
1. Let you choose between OpenRouter, Ollama, or LM Studio
2. Configure API keys (for cloud providers) or base URLs (for local)
3. Fetch and display available models for your provider
4. Let you select your preferred model with pricing information

Manual Examples:
/config ai provider openrouter
/config ai api-key openrouter sk-or-v1-xxx...
/config ai model anthropic/claude-3.5-sonnet
/config ai base-url ollama http://localhost:11434

Providers:
- openrouter: Cloud AI models (requires API key from https://openrouter.ai/keys)
- ollama: Local AI models (requires Ollama installation)
- lmstudio: Local AI models (requires LM Studio)`)
}

func (a *App) handleAIConfigStatus() error {
	if a.aiManager == nil {
		fmt.Println("❌ AI manager not initialized")
		return nil
	}

	config := a.aiManager.GetConfig()
	fmt.Printf("🤖 AI Configuration:\n")
	fmt.Printf("   Provider: %s\n", config.AI.Provider)
	fmt.Printf("   Model: %s\n", config.AI.Model)
	
	// Show usage statistics from usage store if available
	if a.aiManager.GetUsageStore() != nil {
		if summary, err := a.aiManager.GetUsageStore().GetUsageSummary(); err == nil {
			if todayStats, ok := summary["today"]; ok {
				if today, ok := todayStats.(map[string]interface{}); ok {
					fmt.Printf(a.i18nMgr.Get("todays_usage_display"), 
						int(today["requests"].(int)), today["cost"].(float64))
				}
			}
			if weekStats, ok := summary["last_7_days"]; ok {
				if week, ok := weekStats.(map[string]interface{}); ok {
					fmt.Printf(a.i18nMgr.Get("last_7_days_display"), 
						int(week["requests"].(int)), week["cost"].(float64))
				}
			}
		}
	} else {
		fmt.Printf("   Usage: %s\n", a.i18nMgr.Get("usage_not_available_no_db"))
	}

	// Show API key status (masked)
	for provider, key := range config.AI.APIKeys {
		if key != "" {
			maskedKey := key[:min(8, len(key))] + "..." + key[max(0, len(key)-4):]
			fmt.Printf("   %s API Key: %s\n", provider, maskedKey)
		}
	}

	// Show base URLs
	for provider, url := range config.AI.BaseURLs {
		if url != "" {
			fmt.Printf("   %s Base URL: %s\n", provider, url)
		}
	}

	return nil
}

func (a *App) handleAIConfigProvider(args []string) error {
	if len(args) == 0 {
		fmt.Println(a.i18nMgr.Get("usage_ai_config_provider"))
		return nil
	}

	if a.aiManager == nil {
		return errors.New(a.i18nMgr.Get("ai_manager_not_initialized"))
	}

	provider := config.Provider(args[0])
	config := a.aiManager.GetConfig()
	defaultModel := config.GetDefaultModel(provider)

	if err := a.aiManager.SetProvider(provider, defaultModel); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_set_provider"), err)
	}

	fmt.Printf("✅ Set AI provider to %s with model %s\n", provider, defaultModel)
	a.updatePrompt()

	return nil
}

func (a *App) handleAIConfigModel(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /config ai model <model_name>")
		return nil
	}

	if a.aiManager == nil {
		return errors.New(a.i18nMgr.Get("ai_manager_not_initialized"))
	}

	model := args[0]
	config := a.aiManager.GetConfig()

	if err := a.aiManager.SetProvider(config.AI.Provider, model); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_set_model"), err)
	}

	fmt.Printf("✅ Set AI model to %s\n", model)
	a.updatePrompt()

	return nil
}

func (a *App) handleAIConfigAPIKey(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: /config ai api-key <provider> <api_key>")
		return nil
	}

	if a.aiManager == nil {
		return errors.New(a.i18nMgr.Get("ai_manager_not_initialized"))
	}

	provider := config.Provider(args[0])
	apiKey := args[1]

	if err := a.aiManager.SetAPIKey(provider, apiKey); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_set_api_key"), err)
	}

	fmt.Printf("✅ Set API key for %s\n", provider)

	return nil
}

func (a *App) handleAIConfigBaseURL(args []string) error {
	if len(args) < 2 {
		fmt.Println("Usage: /config ai base-url <provider> <url>")
		return nil
	}

	if a.aiManager == nil {
		return errors.New(a.i18nMgr.Get("ai_manager_not_initialized"))
	}

	provider := config.Provider(args[0])
	baseURL := args[1]

	if err := a.aiManager.SetBaseURL(provider, baseURL); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_set_base_url"), err)
	}

	fmt.Printf("✅ Set base URL for %s to %s\n", provider, baseURL)

	return nil
}

func (a *App) handleConfigLanguage(args []string) error {
	if a.aiManager == nil {
		return errors.New(a.i18nMgr.Get("ai_manager_not_initialized"))
	}

	if len(args) == 0 {
		// Show current language
		config := a.aiManager.GetConfig()
		fmt.Printf("Current language: %s\n", config.Language)

		// Show available languages
		availableLanguages := a.i18nMgr.GetAvailableLanguages()
		fmt.Printf("Available languages: %s\n", strings.Join(availableLanguages, ", "))
		return nil
	}

	// Handle status subcommand
	if args[0] == "status" {
		fmt.Println("🌐 Language Configuration:")
		config := a.aiManager.GetConfig()
		fmt.Printf("   Current: %s\n", config.Language)
		
		availableLanguages := a.i18nMgr.GetAvailableLanguages()
		fmt.Printf("   Available: %s\n", strings.Join(availableLanguages, ", "))
		return nil
	}

	newLanguage := args[0]

	// Check if language is available
	availableLanguages := a.i18nMgr.GetAvailableLanguages()
	isAvailable := slices.Contains(availableLanguages, newLanguage)

	if !isAvailable {
		return errors.New(a.i18nMgr.GetWithArgs("language_not_available", newLanguage, strings.Join(availableLanguages, ", ")))
	}

	// Update language in AI config
	if err := a.aiManager.SetLanguage(newLanguage); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_update_language"), err)
	}

	// Update i18n manager
	a.i18nMgr.SetLanguage(newLanguage)

	// Update AI manager i18n
	if err := a.aiManager.UpdateLanguage(newLanguage); err != nil {
		// Don't fail if AI manager i18n update fails
		fmt.Printf("Warning: failed to update AI manager language: %v\n", err)
	}

	fmt.Printf("✅ Language changed to %s\n", newLanguage)
	return nil
}

func (a *App) handleConfigAIOpenRouter(args []string) error {
	if a.aiManager == nil {
		return errors.New(a.i18nMgr.Get("ai_manager_not_initialized"))
	}

	if len(args) == 0 {
		fmt.Println("Available OpenRouter commands:")
		fmt.Println("  key <api-key>    Set OpenRouter API key")
		return nil
	}

	subcmd := args[0]
	switch subcmd {
	case "key":
		if len(args) < 2 {
			return errors.New(a.i18nMgr.Get("api_key_required"))
		}
		apiKey := args[1]

		// Set the API key
		aiConfig := a.aiManager.GetConfig()
		aiConfig.SetAPIKey(config.ProviderOpenRouter, apiKey)

		// Save configuration
		if err := a.aiManager.SetProvider(config.ProviderOpenRouter, aiConfig.AI.Model); err != nil {
			return fmt.Errorf(a.i18nMgr.Get("failed_to_save_configuration"), err)
		}

		fmt.Printf("✅ OpenRouter API key updated\n")
		return nil
	default:
		return fmt.Errorf(a.i18nMgr.Get("unknown_openrouter_subcommand"), subcmd)
	}
}

func (a *App) printConfigHelp(args []string) error {
	if len(args) == 0 {
		// General config help
		fmt.Print(a.i18nMgr.Get("help_config_title"))
		fmt.Print(a.i18nMgr.Get("help_config_general"))
		fmt.Print(a.i18nMgr.Get("help_config_examples"))
		fmt.Print(a.i18nMgr.Get("help_config_subcommand_tip"))
		return nil
	}

	// Subcommand-specific help
	subcommand := args[0]
	switch subcommand {
	case "ai":
		return a.printConfigAIHelp()
	case "language":
		return a.printConfigLanguageHelp()
	case "status":
		return a.printConfigStatusHelp()
	default:
		fmt.Printf(a.i18nMgr.Get("unknown_config_help_subcommand"), subcommand)
		return nil
	}
}

func (a *App) handleAIConfigListModels() error {
	if a.aiManager == nil {
		return errors.New(a.i18nMgr.Get("ai_manager_not_initialized"))
	}

	fmt.Printf("🔍 Fetching available models...\n")

	ctx := context.Background()
	models, err := a.aiManager.ListModels(ctx)
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_list_models"), err)
	}

	if len(models) == 0 {
		fmt.Println("No models available for current provider")
		return nil
	}

	fmt.Printf("📋 Available models for %s:\n", a.aiManager.GetConfig().AI.Provider)
	for i, model := range models {
		fmt.Printf("  %d. %s - %s\n", i+1, model.ID, model.Description)
		if model.Pricing != nil {
			inputCost := ai.FormatPrice(model.Pricing.InputCostPerToken * 1000000)   // per 1M tokens
			outputCost := ai.FormatPrice(model.Pricing.OutputCostPerToken * 1000000) // per 1M tokens
			fmt.Printf("     Pricing: %s input / %s output (per 1M tokens)\n", inputCost, outputCost)
		}
	}

	return nil
}

func (a *App) interactiveAIConfig() error {
	fmt.Println("🤖 Interactive AI Configuration")

	// Use readline instance instead of os.Stdin to avoid conflicts
	fmt.Println("\nNote: Use Ctrl+C to cancel setup at any time")

	// Step 1: Provider Selection
	fmt.Println("\n📊 Select AI provider:")
	fmt.Println("  1. OpenRouter (Cloud AI - requires API key)")
	fmt.Println("  2. Ollama (Local AI - requires Ollama installation)")
	fmt.Println("  3. LM Studio (Local AI - requires LM Studio)")

	a.rl.SetPrompt("Enter choice (1-3): ")
	choice, err := a.rl.Readline()
	if err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_read_input"), err)
	}
	choice = strings.TrimSpace(choice)

	var selectedProvider config.Provider
	switch choice {
	case "1":
		selectedProvider = config.ProviderOpenRouter
	case "2":
		selectedProvider = config.ProviderOllama
	case "3":
		selectedProvider = config.ProviderLMStudio
	default:
		return fmt.Errorf(a.i18nMgr.Get("invalid_choice"), choice)
	}

	fmt.Printf("✅ Selected provider: %s\n", selectedProvider)

	// Step 2: API Key Setup (for cloud providers)
	var needsAPIKey bool
	var apiKey string

	if selectedProvider == config.ProviderOpenRouter {
		needsAPIKey = true

		// Check if API key already exists
		if a.aiManager != nil {
			config := a.aiManager.GetConfig()
			existingKey := config.GetAPIKey(selectedProvider)
			if existingKey != "" {
				maskedKey := existingKey[:min(8, len(existingKey))] + "..." + existingKey[max(0, len(existingKey)-4):]
				fmt.Printf("\n🔑 Existing OpenRouter API key found: %s - keeping existing key\n", maskedKey)
				apiKey = existingKey
				needsAPIKey = false
			}
		}

		if needsAPIKey {
			a.rl.SetPrompt("\n🔐 Enter OpenRouter API key (get one from https://openrouter.ai/keys): ")
			apiKey, err = a.rl.Readline()
			if err != nil {
				return fmt.Errorf(a.i18nMgr.Get("failed_to_read_api_key"), err)
			}
			apiKey = strings.TrimSpace(apiKey)

			if apiKey == "" {
				return fmt.Errorf("API key is required for OpenRouter")
			}
		}
	}

	// Step 3: Base URL Setup (for local providers)
	var baseURL string
	if selectedProvider == config.ProviderOllama || selectedProvider == config.ProviderLMStudio {
		var defaultURL string
		if selectedProvider == config.ProviderOllama {
			defaultURL = "http://localhost:11434"
		} else {
			defaultURL = "http://localhost:1234"
		}

		a.rl.SetPrompt(fmt.Sprintf("\n🌐 Enter base URL for %s [%s]: ", selectedProvider, defaultURL))
		baseURL, err = a.rl.Readline()
		if err != nil {
			return fmt.Errorf(a.i18nMgr.Get("failed_to_read_base_url"), err)
		}
		baseURL = strings.TrimSpace(baseURL)
		if baseURL == "" {
			baseURL = defaultURL
		}
	}

	// Step 4: Initialize/Update AI Manager
	if a.aiManager == nil {
		var err error
		a.aiManager, err = ai.NewManager(a.configMgr.GetConfigDir())
		if err != nil {
			return fmt.Errorf(a.i18nMgr.Get("failed_to_initialize_ai_manager"), err)
		}
	}

	// Set API key if needed
	if apiKey != "" {
		if err := a.aiManager.SetAPIKey(selectedProvider, apiKey); err != nil {
			return fmt.Errorf(a.i18nMgr.Get("failed_to_set_api_key"), err)
		}
		fmt.Printf("✅ API key configured for %s\n", selectedProvider)
	}

	// Set base URL if needed
	if baseURL != "" {
		if err := a.aiManager.SetBaseURL(selectedProvider, baseURL); err != nil {
			return fmt.Errorf(a.i18nMgr.Get("failed_to_set_base_url"), err)
		}
		fmt.Printf("✅ Base URL set to %s\n", baseURL)
	}

	// Step 5: Model Selection
	fmt.Printf("\n🔍 Fetching available models for %s...\n", selectedProvider)

	// Temporarily set the provider to fetch models
	tempConfig := a.aiManager.GetConfig()

	if err := a.aiManager.SetProvider(selectedProvider, tempConfig.GetDefaultModel(selectedProvider)); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_set_temporary_provider"), err)
	}

	ctx := context.Background()
	models, err := a.aiManager.ListModels(ctx)
	if err != nil {
		// If we can't fetch models, fall back to defaults
		fmt.Printf("⚠️  Could not fetch models from %s: %v\n", selectedProvider, err)
		fmt.Println("Using default model for provider.")

		defaultModel := tempConfig.GetDefaultModel(selectedProvider)
		if err := a.aiManager.SetProvider(selectedProvider, defaultModel); err != nil {
			return fmt.Errorf(a.i18nMgr.Get("failed_to_set_default_model"), err)
		}

		fmt.Printf("✅ AI configured with %s using model %s\n", selectedProvider, defaultModel)
		a.updatePrompt()
		return nil
	}

	if len(models) == 0 {
		fmt.Printf("⚠️  No models available for %s\n", selectedProvider)
		defaultModel := tempConfig.GetDefaultModel(selectedProvider)
		if err := a.aiManager.SetProvider(selectedProvider, defaultModel); err != nil {
			return fmt.Errorf(a.i18nMgr.Get("failed_to_set_default_model"), err)
		}
		fmt.Printf("✅ Using default model: %s\n", defaultModel)
		a.updatePrompt()
		return nil
	}

	// Set up model selection with autocomplete
	fmt.Printf("\n🎯 Found %d available models for %s\n", len(models), selectedProvider)

	// Show a few popular examples to help users
	if selectedProvider == config.ProviderOpenRouter && len(models) > 3 {
		fmt.Println("💡 Popular models:")
		count := 0
		for _, model := range models {
			if strings.Contains(model.ID, "claude") || strings.Contains(model.ID, "gpt-4") || strings.Contains(model.ID, "llama") {
				fmt.Printf("   - %s", model.ID)
				if model.Description != "" && len(model.Description) < 50 {
					fmt.Printf(" (%s)", model.Description)
				}
				if model.Pricing != nil {
					inputCost := ai.FormatPrice(model.Pricing.InputCostPerToken * 1000000)
					outputCost := ai.FormatPrice(model.Pricing.OutputCostPerToken * 1000000)
					fmt.Printf(" [%s/%s per 1M tokens]", inputCost, outputCost)
				}
				fmt.Println()
				count++
				if count >= 3 {
					break
				}
			}
		}
		fmt.Println("")
	}

	// Create model name mapping for lookup
	modelMap := make(map[string]ai.ModelInfo)
	for _, model := range models {
		modelMap[model.ID] = model
	}

	// Create a temporary autocompleter that provides full model names as suggestions
	modelCompleter := readline.NewPrefixCompleter(
		readline.PcItemDynamic(func(line string) []string {
			var candidates []string
			prefix := strings.ToLower(line)
			for _, model := range models {
				if strings.Contains(strings.ToLower(model.ID), prefix) {
					candidates = append(candidates, model.ID)
				}
			}
			// Limit suggestions to keep it manageable
			if len(candidates) > 10 {
				candidates = candidates[:10]
			}
			return candidates
		}),
	)

	// Temporarily replace autocompleter
	originalCompleter := a.rl.Config.AutoComplete
	a.rl.Config.AutoComplete = modelCompleter

	var selectedModel ai.ModelInfo
	var modelChoice string

	for {
		a.rl.SetPrompt("🤖 Enter model name (Tab for autocomplete): ")
		modelChoice, err = a.rl.Readline()
		if err != nil {
			// Restore original autocompleter before returning
			a.rl.Config.AutoComplete = originalCompleter
			return fmt.Errorf(a.i18nMgr.Get("failed_to_read_model_choice"), err)
		}
		modelChoice = strings.TrimSpace(modelChoice)

		if modelChoice == "" {
			fmt.Println("Please enter a model name.")
			continue
		}

		// Look for exact match first
		if model, exists := modelMap[modelChoice]; exists {
			selectedModel = model
			break
		}

		// Look for partial matches
		var matches []ai.ModelInfo
		lowerChoice := strings.ToLower(modelChoice)
		for _, model := range models {
			if strings.Contains(strings.ToLower(model.ID), lowerChoice) {
				matches = append(matches, model)
			}
		}

		if len(matches) == 1 {
			selectedModel = matches[0]
			fmt.Printf("✅ Selected: %s\n", selectedModel.ID)
			break
		} else if len(matches) > 1 {
			fmt.Printf("⚠️  Found %d matches:\n", len(matches))
			for i, match := range matches {
				if i >= 5 { // Show max 5 matches
					fmt.Printf("   ... and %d more\n", len(matches)-5)
					break
				}
				fmt.Printf("   - %s", match.ID)
				if match.Description != "" && len(match.Description) < 60 {
					fmt.Printf(" (%s)", match.Description)
				}
				fmt.Println()
			}
			fmt.Println("Please be more specific or copy-paste the exact model name.")
		} else {
			fmt.Printf("❌ No model found matching '%s'.\n", modelChoice)
			fmt.Println("💡 Use Tab for autocomplete or try a different search term.")
		}
	}

	// Restore original autocompleter
	a.rl.Config.AutoComplete = originalCompleter

	// Step 6: Final Configuration
	if err := a.aiManager.SetProvider(selectedProvider, selectedModel.ID); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_set_final_configuration"), err)
	}

	// Ensure the client is properly configured
	if err := a.aiManager.EnsureConfigured(); err != nil {
		return fmt.Errorf(a.i18nMgr.Get("failed_to_initialize_ai_client"), err)
	}

	fmt.Printf("\n🎉 AI Configuration Complete!\n")
	fmt.Printf("   Provider: %s\n", selectedProvider)
	fmt.Printf("   Model: %s\n", selectedModel.ID)
	if selectedModel.Description != "" {
		fmt.Printf("   Description: %s\n", selectedModel.Description)
	}

	if selectedProvider == config.ProviderOpenRouter && selectedModel.Pricing != nil {
		inputCost := ai.FormatPrice(selectedModel.Pricing.InputCostPerToken * 1000000)
		outputCost := ai.FormatPrice(selectedModel.Pricing.OutputCostPerToken * 1000000)
		fmt.Printf("   Pricing: %s input / %s output (per 1M tokens)\n", inputCost, outputCost)
	}

	fmt.Println("\n💬 You can now chat with AI by typing messages without / or @ prefixes!")
	fmt.Println("Example: 'show me all tables' or 'help me write a query to find users'")

	// Restore the original prompt
	a.updatePrompt()
	return nil
}

func (a *App) handleShowPrompts(args []string) error {
	if a.aiManager == nil {
		fmt.Println(a.i18nMgr.Get("ai_not_configured"))
		return nil
	}

	// Prepare markdown file for output
	var mdPath string
	var writer *os.File
	var err error

	if a.config != nil {
		mdPath, writer, err = a.preparePromptHistoryMarkdown()
		if err != nil {
			fmt.Printf(a.i18nMgr.Get("markdown_export_warning"), err)
			writer = nil
		}
	}

	// Helper function to write to both console and file
	writeOutput := func(content string) {
		fmt.Print(content)
		if writer != nil {
			writer.WriteString(content)
		}
	}

	// Get AI conversation history from prompt history
	history := a.aiManager.GetPromptHistory()

	if len(history) == 0 {
		writeOutput(a.i18nMgr.Get("no_ai_history"))
		if writer != nil {
			writer.Close()
			if mdPath != "" {
				fmt.Printf(a.i18nMgr.Get("conversation_history_saved"), mdPath)
			}
		}
		return nil
	}

	// Support optional count argument
	count := len(history)
	if len(args) > 0 {
		if parsedCount, err := strconv.Atoi(args[0]); err == nil && parsedCount > 0 {
			if parsedCount < count {
				count = parsedCount
			}
		}
	}

	// Show the last N conversations
	startIdx := len(history) - count
	if startIdx < 0 {
		startIdx = 0
	}

	writeOutput(a.i18nMgr.GetWithArgs("ai_conversation_history", count))

	for i := startIdx; i < len(history); i++ {
		entry := history[i]

		// Format timestamp
		timeStr := entry.Timestamp.Format("2006-01-02 15:04:05")

		writeOutput(a.i18nMgr.GetWithArgs("request_number", i+1, timeStr))

		// Provider, model, tokens, cost info
		writeOutput(a.i18nMgr.GetWithArgs("provider_info", entry.Provider, entry.Model, entry.InputTokens, entry.OutputTokens))

		if entry.Cost > 0 {
			writeOutput(a.i18nMgr.GetWithArgs("cost_paid", entry.Cost))
		} else {
			writeOutput(a.i18nMgr.Get("cost_free"))
		}
		writeOutput("\n\n")

		writeOutput(a.i18nMgr.Get("user_request"))
		writeOutput(entry.UserMessage)
		writeOutput("\n```\n\n")

		// System prompt section
		if entry.SystemPrompt != "" {
			writeOutput(a.i18nMgr.Get("system_prompt_header"))
			writeOutput("```\n")
			writeOutput(entry.SystemPrompt)
			writeOutput("\n```\n\n")
		}

		writeOutput(a.i18nMgr.Get("ai_response"))
		if entry.AIResponse != "" {
			writeOutput(entry.AIResponse)
		} else {
			writeOutput(a.i18nMgr.Get("ai_response_unavailable"))
		}
		writeOutput("\n\n")

		if i < len(history)-1 {
			writeOutput("---\n\n")
		}
	}

	// Close file and show location
	if writer != nil {
		writer.Close()
		if mdPath != "" {
			// Display the markdown file using the same method as query results
			if err := a.sessionMgr.ViewMarkdown(mdPath); err != nil {
				fmt.Printf(a.i18nMgr.Get("generic_warning"), err)
			}
			fmt.Printf(a.i18nMgr.Get("conversation_history_saved"), mdPath)
		}
	}

	return nil
}

func (a *App) handleClearConversation() error {
	if a.aiManager == nil {
		fmt.Println(a.i18nMgr.Get("ai_not_configured_short"))
		return nil
	}

	conversation := a.aiManager.GetCurrentConversation()
	if conversation == nil {
		fmt.Println(a.i18nMgr.Get("no_active_conversation"))
		return nil
	}

	// Show conversation summary before clearing
	fmt.Printf(a.i18nMgr.Get("clearing_conversation"),
		conversation.CurrentPhase.String(), len(conversation.LoadedTables))

	// Clear the conversation
	a.aiManager.ClearConversation()
	fmt.Println(a.i18nMgr.Get("conversation_cleared"))

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Help functions for specific commands and subcommands

func (a *App) printConfigAIHelp() error {
	fmt.Print(a.i18nMgr.Get("help_config_ai_title"))
	fmt.Print(a.i18nMgr.Get("help_config_ai_interactive"))
	fmt.Print(a.i18nMgr.Get("help_config_ai_status"))
	fmt.Print(a.i18nMgr.Get("help_config_ai_providers"))
	fmt.Print(a.i18nMgr.Get("help_config_ai_models"))
	fmt.Print(a.i18nMgr.Get("help_config_ai_auth"))
	fmt.Print(a.i18nMgr.Get("help_config_ai_shortcuts"))
	fmt.Print(a.i18nMgr.Get("help_config_ai_examples"))
	fmt.Print(a.i18nMgr.Get("help_config_ai_provider_list"))
	return nil
}

func (a *App) printConfigLanguageHelp() error {
	fmt.Print(a.i18nMgr.Get("help_config_language_title"))
	fmt.Print(a.i18nMgr.Get("help_config_language_commands"))
	fmt.Print(a.i18nMgr.Get("help_config_language_options"))
	fmt.Print(a.i18nMgr.Get("help_config_language_examples"))
	return nil
}

func (a *App) printConfigStatusHelp() error {
	fmt.Print(a.i18nMgr.Get("help_config_status_title"))
	fmt.Print(a.i18nMgr.Get("help_config_status_description"))
	return nil
}

func (a *App) printConnectHelp() error {
	fmt.Print(a.i18nMgr.Get("help_connect_title"))
	fmt.Print(a.i18nMgr.Get("help_connect_commands"))
	fmt.Print(a.i18nMgr.Get("help_connect_interactive"))
	fmt.Print(a.i18nMgr.Get("help_connect_examples"))
	return nil
}

func (a *App) printExecHelp() error {
	fmt.Print(a.i18nMgr.Get("help_exec_title"))
	fmt.Print(a.i18nMgr.Get("help_exec_commands"))
	fmt.Print(a.i18nMgr.Get("help_exec_multiline_detailed"))
	fmt.Print(a.i18nMgr.Get("help_exec_examples"))
	return nil
}

func (a *App) printTablesHelp() error {
	fmt.Print(a.i18nMgr.Get("help_tables_title"))
	fmt.Print(a.i18nMgr.Get("help_tables_description"))
	return nil
}

func (a *App) printDescribeHelp() error {
	fmt.Print(a.i18nMgr.Get("help_describe_title"))
	fmt.Print(a.i18nMgr.Get("help_describe_usage"))
	fmt.Print(a.i18nMgr.Get("help_describe_features"))
	fmt.Print(a.i18nMgr.Get("help_describe_examples"))
	return nil
}

func (a *App) printStatusHelp() error {
	fmt.Print(a.i18nMgr.Get("help_status_title"))
	fmt.Print(a.i18nMgr.Get("help_status_description"))
	return nil
}

func (a *App) printPromptsHelp() error {
	fmt.Print(a.i18nMgr.Get("help_prompts_title"))
	fmt.Print(a.i18nMgr.Get("help_prompts_usage"))
	fmt.Print(a.i18nMgr.Get("help_prompts_features"))
	fmt.Print(a.i18nMgr.Get("help_prompts_examples"))
	return nil
}
