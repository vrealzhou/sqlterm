package cli

import (
	"fmt"
	"os"

	"github.com/vrealzhou/sqlterm/internal/config"
	"github.com/vrealzhou/sqlterm/internal/conversation"
	"github.com/vrealzhou/sqlterm/internal/database"
	"github.com/vrealzhou/sqlterm/internal/editor"
	"github.com/vrealzhou/sqlterm/internal/i18n"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool

	// Version information (set from main)
	Version   string = "dev"
	BuildTime string = "unknown"
	GitCommit string = "unknown"

	// Track current connection for editor command
	currentConnection database.Connection
)

// SetVersionInfo sets the version information
func SetVersionInfo(version, buildTime, gitCommit string) {
	Version = version
	BuildTime = buildTime
	GitCommit = gitCommit
}

var rootCmd = &cobra.Command{
	Use:   "sqlterm",
	Short: "", // Will be set in init()
	Long:  "", // Will be set in init()
	RunE: func(cmd *cobra.Command, args []string) error {
		return runConversation()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Initialize i18n for command descriptions
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		// Fallback to hardcoded strings if i18n fails
		rootCmd.Short = "A terminal-based SQL database tool"
		rootCmd.Long = "SQLTerm provides an intuitive conversation-style interface for managing database connections and executing queries across MySQL, PostgreSQL, and SQLite."
	} else {
		rootCmd.Short = i18nMgr.Get("app_short_description")
		rootCmd.Long = i18nMgr.Get("app_long_description")

		// Update command descriptions
		connectCmd.Short = i18nMgr.Get("connect_command_short")
		listCmd.Short = i18nMgr.Get("list_command_short")
		addCmd.Short = i18nMgr.Get("add_command_short")
		versionCmd.Short = i18nMgr.Get("version_command_short")
		versionCmd.Long = i18nMgr.Get("version_command_long")
		editorCmd.Short = i18nMgr.Get("editor_command_short")
		editorCmd.Short = i18nMgr.Get("editor_command_short")

		// Update flag descriptions
		updateFlagDescriptions(i18nMgr)
	}

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", getI18nString(i18nMgr, "config_file_flag", "config file (default is $HOME/.sqlterm.yaml)"))
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, getI18nString(i18nMgr, "verbose_output_flag", "verbose output"))

	rootCmd.AddCommand(connectCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(addCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(editorCmd)
}

// getI18nString safely gets an i18n string with fallback
func getI18nString(mgr *i18n.Manager, key, fallback string) string {
	if mgr == nil {
		return fallback
	}
	return mgr.Get(key)
}

func updateFlagDescriptions(i18nMgr *i18n.Manager) {
	if i18nMgr == nil {
		return
	}

	// Update connect command flag descriptions safely
	if flag := connectCmd.Flags().Lookup("db-type"); flag != nil {
		flag.Usage = i18nMgr.Get("flag_db_type")
	}
	if flag := connectCmd.Flags().Lookup("host"); flag != nil {
		flag.Usage = i18nMgr.Get("flag_host")
	}
	if flag := connectCmd.Flags().Lookup("port"); flag != nil {
		flag.Usage = i18nMgr.Get("flag_port")
	}
	if flag := connectCmd.Flags().Lookup("database"); flag != nil {
		flag.Usage = i18nMgr.Get("flag_database")
	}
	if flag := connectCmd.Flags().Lookup("username"); flag != nil {
		flag.Usage = i18nMgr.Get("flag_username")
	}
	if flag := connectCmd.Flags().Lookup("password"); flag != nil {
		flag.Usage = i18nMgr.Get("flag_password")
	}
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".sqlterm")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		if verbose {
			fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
		}
	}
}

func runConversation() error {
	app, err := conversation.NewApp()
	if err != nil {
		return fmt.Errorf("failed to create conversation app: %w", err)
	}
	return app.Run()
}

var connectCmd = &cobra.Command{
	Use:   "connect",
	Short: "", // Will be set in init()
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize i18n manager
		i18nMgr, err := i18n.NewManager("en_au")
		if err != nil {
			i18nMgr, _ = i18n.NewManager("en_au")
		}

		dbType, _ := cmd.Flags().GetString("db-type")
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		database, _ := cmd.Flags().GetString("database")
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")

		dbTypeEnum, err := config.ParseDatabaseType(dbType)
		if err != nil {
			return err
		}

		if port == 0 {
			port = config.GetDefaultPort(dbTypeEnum)
		}

		config := &config.ConnectionConfig{
			Name:         fmt.Sprintf(i18nMgr.Get("connection_name_format"), dbType),
			DatabaseType: dbTypeEnum,
			Host:         host,
			Port:         port,
			Database:     database,
			Username:     username,
			Password:     password,
			SSL:          false,
		}

		return connectAndRunConversation(config)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "", // Will be set in init()
	RunE: func(cmd *cobra.Command, args []string) error {
		return listConnections()
	},
}

var addCmd = &cobra.Command{
	Use:   "add [name]",
	Short: "", // Will be set in init()
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		dbType, _ := cmd.Flags().GetString("db-type")
		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		database, _ := cmd.Flags().GetString("database")
		username, _ := cmd.Flags().GetString("username")

		dbTypeEnum, err := config.ParseDatabaseType(dbType)
		if err != nil {
			return err
		}

		if port == 0 {
			port = config.GetDefaultPort(dbTypeEnum)
		}

		config := &config.ConnectionConfig{
			Name:         name,
			DatabaseType: dbTypeEnum,
			Host:         host,
			Port:         port,
			Database:     database,
			Username:     username,
			SSL:          false,
		}

		return addConnection(config)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "", // Will be set in init()
	Long:  "", // Will be set in init(),
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize i18n manager
		i18nMgr, err := i18n.NewManager("en_au")
		if err != nil {
			i18nMgr, _ = i18n.NewManager("en_au")
		}

		fmt.Printf(i18nMgr.Get("sqlterm_version"), Version)
		fmt.Printf(i18nMgr.Get("build_time"), BuildTime)
		fmt.Printf(i18nMgr.Get("git_commit"), GitCommit)
	},
}

var editorCmd = &cobra.Command{
	Use:   "editor",
	Short: "", // Will be set in init()
	Long:  "Launch the SQL editor with LSP support and autocomplete",
	RunE: func(cmd *cobra.Command, args []string) error {
		if rootCmd.Flag("help").Value.String() == "true" {
			return nil
		}
		
		// Check if we have an active connection
		if currentConnection == nil {
			return fmt.Errorf("no active database connection. Use 'connect' command first")
		}
		
		return runEditor(currentConnection)
	},
}

func init() {
	// Set up flags with English fallbacks - will be updated in initI18n()
	connectCmd.Flags().StringP("db-type", "t", "", "Database type (mysql, postgres, sqlite)")
	connectCmd.Flags().StringP("host", "H", "localhost", "Host")
	connectCmd.Flags().IntP("port", "p", 0, "Port")
	connectCmd.Flags().StringP("database", "d", "", "Database name")
	connectCmd.Flags().StringP("username", "u", "", "Username")
	connectCmd.Flags().StringP("password", "P", "", "Password")
	connectCmd.MarkFlagRequired("db-type")
	connectCmd.MarkFlagRequired("database")
	connectCmd.MarkFlagRequired("username")

	addCmd.Flags().StringP("db-type", "t", "", "Database type (mysql, postgres, sqlite)")
	addCmd.Flags().StringP("host", "H", "localhost", "Host")
	addCmd.Flags().IntP("port", "p", 0, "Port")
	addCmd.Flags().StringP("database", "d", "", "Database name")
	addCmd.Flags().StringP("username", "u", "", "Username")
	addCmd.MarkFlagRequired("db-type")
	addCmd.MarkFlagRequired("database")
	addCmd.MarkFlagRequired("username")
}

func connectAndRunConversation(connConfig *config.ConnectionConfig) error {
	// Initialize i18n manager for CLI
	language := "en_au" // Default language

	// Use default language for CLI (AI config is managed in the conversation app)

	i18nMgr, err := i18n.NewManager(language)
	if err != nil {
		// Fallback to default language if i18n fails
		i18nMgr, _ = i18n.NewManager("en_au")
	}

	fmt.Printf(i18nMgr.Get("connecting_to"), connConfig.Name)

	conn, err := database.NewConnection(connConfig)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	// Set the current connection for editor command
	currentConnection = conn

	fmt.Printf(i18nMgr.Get("connected_successfully"), connConfig.Name)
	fmt.Print(i18nMgr.Get("starting_conversation_mode"))

	app, err := conversation.NewApp()
	if err != nil {
		return fmt.Errorf("failed to create conversation app: %w", err)
	}

	app.SetConnection(conn, connConfig)
	return app.Run()
}

func runEditor(conn database.Connection) error {
	// Initialize i18n manager
	language := "en_au" // Default language

	// Use default language for CLI (AI config is managed in the conversation app)

	i18nMgr, err := i18n.NewManager(language)
	if err != nil {
		// Fallback to default language if i18n fails
		i18nMgr, _ = i18n.NewManager("en_au")
	}

	fmt.Println(i18nMgr.Get("launching_sql_editor"))

	// Create and run the Bubbletea editor
	editor := editor.NewEditor(conn)
	
	// Run the editor in Bubbletea
	p := tea.NewProgram(editor)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("editor failed: %w", err)
	}

	return nil
}

func listConnections() error {
	// Initialize i18n
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		i18nMgr, _ = i18n.NewManager("en_au")
	}

	configManager := config.NewManager()
	connections := configManager.ListConnections()

	if len(connections) == 0 {
		fmt.Println(i18nMgr.Get("no_saved_connections_found"))
		fmt.Println(i18nMgr.Get("add_connection_instruction"))
		return nil
	}

	fmt.Println(i18nMgr.Get("saved_connections_cli"))
	index := 1
	for _, conn := range connections {
		fmt.Printf("%d. %s (%s) - %s://%s:%d/%s\n",
			index,
			conn.Name,
			conn.DatabaseType,
			conn.DatabaseType.String(),
			conn.Host,
			conn.Port,
			conn.Database)
		index++
	}

	return nil
}

func addConnection(cfg *config.ConnectionConfig) error {
	// Initialize i18n
	i18nMgr, err := i18n.NewManager("en_au")
	if err != nil {
		i18nMgr, _ = i18n.NewManager("en_au")
	}

	fmt.Printf(i18nMgr.Get("testing_connection_cli"), cfg.Name)

	conn, err := database.NewConnection(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return fmt.Errorf("connection test failed: %w", err)
	}

	fmt.Println(i18nMgr.Get("connection_test_successful"))

	configManager := config.NewManager()
	if err := configManager.SaveConnection(cfg); err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}

	fmt.Printf(i18nMgr.Get("connection_saved_cli"), cfg.Name)
	fmt.Println(i18nMgr.Get("use_list_instruction"))
	fmt.Println(i18nMgr.Get("use_sqlterm_instruction"))

	return nil
}
