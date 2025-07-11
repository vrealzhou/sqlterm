package conversation

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sqlterm/internal/config"
	"sqlterm/internal/core"
	"sqlterm/internal/session"

	"github.com/chzyer/readline"
)

type App struct {
	rl         *readline.Instance
	connection core.Connection
	config     *core.ConnectionConfig
	configMgr  *config.Manager
	sessionMgr *session.Manager
}

func NewApp() (*App, error) {
	configMgr := config.NewManager()
	sessionMgr := session.NewManager(configMgr.GetConfigDir())

	app := &App{
		configMgr:  configMgr,
		sessionMgr: sessionMgr,
	}

	// Ensure sessions directory exists for history file
	sessionsDir := filepath.Join(configMgr.GetConfigDir(), "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	// Set up dynamic autocomplete
	completer := NewAutoCompleter(app)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:       "sqlterm > ",
		AutoComplete: completer,
		HistoryFile:  filepath.Join(configMgr.GetConfigDir(), "sessions", "history.txt"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create readline: %w", err)
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
		fmt.Printf("Warning: failed to initialize session directory: %v\n", err)
	}
}

func (a *App) updatePrompt() {
	if a.config != nil {
		a.rl.SetPrompt(fmt.Sprintf("sqlterm (%s) > ", a.config.Database))
	} else {
		a.rl.SetPrompt("sqlterm > ")
	}
}

func (a *App) Run() error {
	defer a.rl.Close()

	fmt.Println("🗄️  SQLTerm - Conversation Mode")
	fmt.Println("Type /help for available commands, or enter SQL queries directly.")
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
			fmt.Printf("Error: %v\n", err)
		}
	}

	return nil
}

func (a *App) processLine(line string) error {
	if strings.HasPrefix(line, "/") {
		return a.processCommand(line)
	} else if strings.HasPrefix(line, "@") {
		return a.processQueryFile(line)
	}
	return nil
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
		a.printHelp()
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
	default:
		fmt.Printf("Unknown command: %s. Type /help for available commands.\n", command)
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
		fmt.Println("No database connection. Use /connect to connect to a database.")
		return nil
	}

	result, err := a.connection.Execute(query)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}

	// Save as markdown and show with glow
	if a.config != nil {
		if err := a.sessionMgr.EnsureSessionDir(a.config.Name); err != nil {
			fmt.Printf("Warning: failed to create session directory: %v\n", err)
		} else {
			err := core.SaveQueryResultAsMarkdown(result, query, a.config.Name, resultWriter)
			if err != nil {
				fmt.Printf("Warning: failed to save markdown: %v\n", err)
			}
		}
	}

	return nil
}

func (a *App) prepareQueryResultMarkdown() (string, *os.File, error) {
	if err := a.sessionMgr.EnsureSessionDir(a.config.Name); err != nil {
		return "", nil, fmt.Errorf("failed to create session directory: %v", err)
	}
	// Generate filename with timestamp
	configDir := a.configMgr.GetConfigDir()
	// Create sessions directory structure
	resultsDir := filepath.Join(configDir, "sessions", a.config.Name, "results")
	if err := os.MkdirAll(resultsDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create results directory %s: %w", resultsDir, err)
	}
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("query_results_%s.md", timestamp)
	filename = filepath.Join(resultsDir, filename)
	writer, err := os.Create(filename)
	if err != nil {
		return filename, nil, err
	}
	var content strings.Builder
	content.WriteString(fmt.Sprintf("# Query Results - %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	content.WriteString(fmt.Sprintf("**Connection:** %s\n\n", a.config.Name))
	writer.Write([]byte(content.String()))
	return filename, writer, err
}

func (a *App) executeFile(filename string, queryRange []int) error {
	if a.connection == nil {
		fmt.Println("No database connection. Use /connect to connect to a database.")
		return nil
	}

	// Try to find the file in current directory or queries directory
	var filepath string
	if _, err := os.Stat(filename); err == nil {
		filepath = filename
	} else if _, err := os.Stat("queries/" + filename); err == nil {
		filepath = "queries/" + filename
	} else {
		return fmt.Errorf("file not found: %s", filename)
	}

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	queries := a.parseQueries(string(content))
	fmt.Printf("📁 Executing SQL file: %s\n", filename)
	fmt.Printf("🔄 Found %d queries in file\n", len(queries))

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
			fmt.Printf("❌ Query failed: %v\n", err)
		}
	}
	writer.Close()

	if err := a.sessionMgr.ViewMarkdownWithGlow(mdPath); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}
	fmt.Printf("📍 File location: %s\n", mdPath)

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

func (a *App) printHelp() {
	fmt.Println(`
Available commands:

/help                    Show this help message
/connect                 Interactive connection setup
/connect [name]          Connect to saved connection (Tab: autocomplete names)
/list-connections        List all saved connections
/tables                  List tables in current database
/describe [table]        Show table structure (Tab: autocomplete table names)
/status                  Show current connection status
/exec [query]            Execute a query directly
/exec [query] > file.csv Export query results to CSV
/quit, /exit             Exit SQLTerm

File commands:
@filename.sql            Execute all queries in file (Tab: autocomplete files)
@filename.sql 1          Execute only query 1
@filename.sql 2-5        Execute queries 2 through 5

CSV Export:
SELECT * FROM table > output.csv    Export query results to CSV (Tab: autocomplete filenames)
/exec SELECT * FROM table > out.csv Export with /exec command

SQL queries:
Enter any SQL query directly to execute it.
Results are automatically saved as markdown and can be viewed with glow.

Auto-completion:
- Tab after /connect to see connection names
- Tab after /describe to see table names
- Tab after @ to see .sql files (searches all subdirectories)
- Tab after > to see/create .csv files
- Excludes hidden folders (starting with .) and common build directories

Session Management:
- Results are automatically saved to ~/.config/sqlterm/sessions/{connection}/results/
- Old result files are automatically cleaned up based on retention settings
- Configure cleanup in ~/.config/sqlterm/sessions/{connection}/session.yaml
- Default retention: 30 days (cleanup_retention_days: 30)
`)
}

func (a *App) handleConnect(args []string) error {
	if len(args) == 0 {
		return a.interactiveConnect()
	}

	name := args[0]
	config, err := a.configMgr.LoadConnection(name)
	if err != nil {
		return fmt.Errorf("failed to load connection '%s': %w", name, err)
	}

	fmt.Printf("Connecting to %s...\n", config.Name)
	conn, err := core.NewConnection(config)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	if a.connection != nil {
		a.connection.Close()
	}

	a.SetConnection(conn, config)
	fmt.Printf("✅ Connected to %s (%s)\n", config.Name, config.Database)

	return nil
}

func (a *App) interactiveConnect() error {
	fmt.Println("🔧 Interactive Connection Setup")

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("📝 Enter connection name: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	fmt.Println("📊 Select database type:")
	fmt.Println("  1. MySQL")
	fmt.Println("  2. PostgreSQL")
	fmt.Println("  3. SQLite")
	fmt.Print("Enter choice (1-3): ")

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
		return fmt.Errorf("invalid choice: %s", choice)
	}

	config := &core.ConnectionConfig{
		Name:         name,
		DatabaseType: dbType,
	}

	if dbType != core.SQLite {
		fmt.Print("📝 Enter host [localhost]: ")
		host, _ := reader.ReadString('\n')
		host = strings.TrimSpace(host)
		if host == "" {
			host = "localhost"
		}
		config.Host = host

		fmt.Printf("📝 Enter port [%d]: ", core.GetDefaultPort(dbType))
		portStr, _ := reader.ReadString('\n')
		portStr = strings.TrimSpace(portStr)
		if portStr == "" {
			config.Port = core.GetDefaultPort(dbType)
		} else {
			port, err := strconv.Atoi(portStr)
			if err != nil {
				return fmt.Errorf("invalid port: %s", portStr)
			}
			config.Port = port
		}

		fmt.Print("📝 Enter username: ")
		username, _ := reader.ReadString('\n')
		config.Username = strings.TrimSpace(username)

		fmt.Print("🔐 Enter password: ")
		password, _ := reader.ReadString('\n')
		config.Password = strings.TrimSpace(password)
	}

	fmt.Print("📝 Enter database name: ")
	database, _ := reader.ReadString('\n')
	config.Database = strings.TrimSpace(database)

	// Test connection
	fmt.Printf("Testing connection to %s...\n", config.Name)
	conn, err := core.NewConnection(config)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	if a.connection != nil {
		a.connection.Close()
	}

	a.SetConnection(conn, config)
	fmt.Printf("✅ Connected to %s (%s)\n", config.Name, config.Database)

	// Save connection
	if err := a.configMgr.SaveConnection(config); err != nil {
		fmt.Printf("Warning: failed to save connection: %v\n", err)
	} else {
		fmt.Println("💾 Connection saved!")
	}

	return nil
}

func (a *App) handleListConnections() error {
	connections, err := a.configMgr.ListConnections()
	if err != nil {
		return fmt.Errorf("failed to list connections: %w", err)
	}

	if len(connections) == 0 {
		fmt.Println("No saved connections found.")
		return nil
	}

	fmt.Println("📋 Saved connections:")
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
		fmt.Println("No database connection. Use /connect to connect to a database.")
		return nil
	}

	tables, err := a.connection.ListTables()
	if err != nil {
		return fmt.Errorf("failed to list tables: %w", err)
	}

	if len(tables) == 0 {
		fmt.Printf("No tables found in database '%s'.\n", a.config.Database)
		return nil
	}

	fmt.Printf("📋 Tables in %s:\n", a.config.Database)
	for i, table := range tables {
		fmt.Printf("  %d. %s\n", i+1, table)
	}

	return nil
}

func (a *App) handleDescribeTable(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /describe <table_name>")
		return nil
	}

	if a.connection == nil {
		fmt.Println("No database connection. Use /connect to connect to a database.")
		return nil
	}

	tableName := args[0]
	tableInfo, err := a.connection.DescribeTable(tableName)
	if err != nil {
		return fmt.Errorf("failed to describe table: %w", err)
	}

	fmt.Printf("📊 Table: %s\n", tableInfo.Name)
	fmt.Println("   Columns:")

	for _, col := range tableInfo.Columns {
		nullable := "NOT NULL"
		if col.Nullable {
			nullable = "NULL"
		}

		key := ""
		if col.Key != "" {
			key = fmt.Sprintf(" (%s)", col.Key)
		}

		defaultVal := ""
		if col.Default != nil {
			defaultVal = fmt.Sprintf(" (DEFAULT %s)", *col.Default)
		}

		fmt.Printf("     %-15s %-15s %-8s%s%s\n",
			col.Name, col.Type, nullable, key, defaultVal)
	}

	return nil
}

func (a *App) handleStatus() {
	if a.connection == nil {
		fmt.Println("📡 Status: Not connected")
		fmt.Println("Use /connect to establish a database connection.")
		return
	}

	fmt.Printf("📡 Status: Connected to %s\n", a.config.Name)
	fmt.Printf("   Database: %s\n", a.config.Database)
	fmt.Printf("   Type: %s\n", a.config.DatabaseType)
	if a.config.DatabaseType != core.SQLite {
		fmt.Printf("   Host: %s:%d\n", a.config.Host, a.config.Port)
		fmt.Printf("   Username: %s\n", a.config.Username)
	}
}

func (a *App) handleExecQuery(args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: /exec <query>")
		fmt.Println("       /exec <query> > filename.csv")
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
	if err := a.sessionMgr.ViewMarkdownWithGlow(mdPath); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}
	fmt.Printf("📍 File location: %s\n", mdPath)
	return nil
}

func (a *App) processQueryWithCSVExport(line string) error {
	if a.connection == nil {
		fmt.Println("No database connection. Use /connect to connect to a database.")
		return nil
	}

	parts := strings.SplitN(line, " > ", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid CSV export syntax. Use: query > filename.csv")
	}

	query := strings.TrimSpace(parts[0])
	filename := strings.TrimSpace(parts[1])

	fmt.Printf("📊 Executing query and streaming to %s...\n", filename)

	result, err := a.connection.Execute(query)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}

	rows, err := core.SaveQueryResultAsStreamingCSV(result, filename)
	if err != nil {
		return fmt.Errorf("failed to save CSV: %w", err)
	}

	fmt.Printf("✅ Exported %d rows to %s\n", rows, filename)
	return nil
}

func (a *App) processFileCommandWithCSVExport(line string) error {
	if a.connection == nil {
		fmt.Println("No database connection. Use /connect to connect to a database.")
		return nil
	}

	parts := strings.SplitN(line, " > ", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid CSV export syntax. Use: @filename.sql > output.csv")
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
		fmt.Println("No database connection. Use /connect to connect to a database.")
		return nil
	}

	// Try to find the file in current directory or queries directory
	var filepath string
	if _, err := os.Stat(filename); err == nil {
		filepath = filename
	} else if _, err := os.Stat("queries/" + filename); err == nil {
		filepath = "queries/" + filename
	} else {
		return fmt.Errorf("file not found: %s", filename)
	}

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	queries := a.parseQueries(string(content))
	fmt.Printf("📁 Executing SQL file: %s\n", filename)
	fmt.Printf("🔄 Found %d queries in file\n", len(queries))

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

		fmt.Printf("\n📝 Query %d: %s\n", i+1, a.truncateQuery(query))
		result, err := a.connection.Execute(query)
		if err != nil {
			fmt.Printf("❌ Query failed: %v\n", err)
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
		fmt.Printf("📊 Query executed (%d rows)\n", rows)

		exportedFiles = append(exportedFiles, outputPath)
		totalRowsExported += rows
		fmt.Printf("✅ Exported %d rows to %s\n", rows, outputPath)
	}

	// Summary of exported files
	if len(exportedFiles) > 0 {
		if len(exportedFiles) == 1 {
			fmt.Printf("\n📁 Exported to: %s\n", exportedFiles[0])
		} else {
			fmt.Printf("\n📁 Exported to %d files:\n", len(exportedFiles))
			for _, file := range exportedFiles {
				fmt.Printf("   - %s\n", file)
			}
		}
		fmt.Printf("📊 Total: %d rows exported\n", totalRowsExported)
	} else {
		fmt.Printf("⚠️  No results to export to CSV (all queries returned 0 rows)\n")
	}

	return nil
}
